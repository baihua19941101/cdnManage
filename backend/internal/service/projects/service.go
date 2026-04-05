package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/provider"
	"github.com/baihua19941101/cdnManage/internal/repository"
)

type Service struct {
	projects         repository.ProjectRepository
	tx               repository.TxManager
	credentialCipher credentialCipher
	providers        *provider.Registry
}

type credentialCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type CreateProjectInput struct {
	Name        string
	Description string
	Buckets     []ProjectBucketInput
	CDNs        []ProjectCDNInput
}

type UpdateProjectInput struct {
	Name        string
	Description string
	Buckets     []ProjectBucketInput
	CDNs        []ProjectCDNInput
}

type ProjectBucketInput struct {
	ProviderType         string
	BucketName           string
	Region               string
	Credential           string
	CredentialCiphertext string
	IsPrimary            bool
}

type ProjectCDNInput struct {
	ProviderType string
	CDNEndpoint  string
	PurgeScope   string
	IsPrimary    bool
}

type ListBucketObjectsInput struct {
	BucketName string
	Prefix     string
	Marker     string
	MaxKeys    int
}

type UploadBucketObjectInput struct {
	BucketName  string
	Key         string
	ContentType string
	Content     io.Reader
	Size        int64
}

type DownloadBucketObjectInput struct {
	BucketName string
	Key        string
}

type DeleteBucketObjectInput struct {
	BucketName string
	Key        string
}

type RenameBucketObjectInput struct {
	BucketName string
	SourceKey  string
	TargetKey  string
}

func NewService(projects repository.ProjectRepository, tx repository.TxManager, cipher ...credentialCipher) *Service {
	var credentialCipherInstance credentialCipher
	if len(cipher) > 0 {
		credentialCipherInstance = cipher[0]
	}

	return &Service{
		projects:         projects,
		tx:               tx,
		credentialCipher: credentialCipherInstance,
		providers:        provider.NewRegistry(),
	}
}

func (s *Service) RegisterObjectStorageProvider(p provider.ObjectStorageProvider) error {
	return s.providers.RegisterObjectStorage(p)
}

func (s *Service) List(ctx context.Context, filter repository.ProjectFilter) ([]model.Project, error) {
	projects, err := s.projects.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	for index := range projects {
		s.maskBucketCredentials(&projects[index])
	}

	return projects, nil
}

func (s *Service) GetByID(ctx context.Context, projectID uint64) (*model.Project, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}
	s.maskBucketCredentials(project)
	return project, nil
}

func (s *Service) Create(ctx context.Context, input CreateProjectInput) (*model.Project, error) {
	normalizedBuckets, err := s.normalizeBucketsWithProviderDetection(input.Buckets)
	if err != nil {
		return nil, err
	}
	if err := validateBindings(normalizedBuckets, input.CDNs); err != nil {
		return nil, err
	}

	project := &model.Project{
		Name:        input.Name,
		Description: input.Description,
	}
	if err := s.tx.WithinTransaction(ctx, func(repos repository.Repositories) error {
		if err := repos.Projects().Create(ctx, project); err != nil {
			return fmt.Errorf("create project: %w", err)
		}
		return s.replaceBindings(ctx, repos, project.ID, normalizedBuckets, input.CDNs)
	}); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, project.ID)
}

func (s *Service) Update(ctx context.Context, projectID uint64, input UpdateProjectInput) (*model.Project, error) {
	normalizedBuckets, err := s.normalizeBucketsWithProviderDetection(input.Buckets)
	if err != nil {
		return nil, err
	}
	if err := validateBindings(normalizedBuckets, input.CDNs); err != nil {
		return nil, err
	}

	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}

	project.Name = input.Name
	project.Description = input.Description
	if err := s.tx.WithinTransaction(ctx, func(repos repository.Repositories) error {
		if err := repos.Projects().Update(ctx, project); err != nil {
			return fmt.Errorf("update project: %w", err)
		}
		return s.replaceBindings(ctx, repos, projectID, normalizedBuckets, input.CDNs)
	}); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, projectID)
}

func (s *Service) Delete(ctx context.Context, projectID uint64) error {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}

	return s.tx.WithinTransaction(ctx, func(repos repository.Repositories) error {
		if err := repos.UserProjectRoles().DeleteByProjectID(ctx, projectID); err != nil {
			return fmt.Errorf("delete project role bindings: %w", err)
		}
		if err := repos.Projects().Delete(ctx, projectID); err != nil {
			return fmt.Errorf("delete project: %w", err)
		}
		return nil
	})
}

func (s *Service) ValidateBucketConnection(_ context.Context, input ProjectBucketInput) (string, error) {
	normalized, err := s.normalizeBucketWithProviderDetection(input)
	if err != nil {
		return "", err
	}
	return normalized.ProviderType, nil
}

func (s *Service) ListBucketObjects(ctx context.Context, projectID uint64, input ListBucketObjectsInput) ([]provider.ObjectInfo, error) {
	bucket, storageProvider, credentialPayload, err := s.resolveBucketProviderAndCredential(ctx, projectID, strings.TrimSpace(input.BucketName))
	if err != nil {
		return nil, err
	}

	objects, err := storageProvider.ListObjects(ctx, provider.ListObjectsRequest{
		Bucket:     bucket.BucketName,
		Region:     bucket.Region,
		Prefix:     input.Prefix,
		Marker:     input.Marker,
		MaxKeys:    input.MaxKeys,
		Credential: credentialPayload,
	})
	if err != nil {
		return nil, mapObjectStorageListError(err)
	}

	return objects, nil
}

func (s *Service) UploadBucketObject(ctx context.Context, projectID uint64, input UploadBucketObjectInput) error {
	bucket, storageProvider, credentialPayload, err := s.resolveBucketProviderAndCredential(ctx, projectID, strings.TrimSpace(input.BucketName))
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.Key) == "" || input.Content == nil {
		return httpresp.NewAppError(400, "validation_error", "object key and content are required", nil)
	}

	if err := storageProvider.UploadObject(ctx, provider.UploadObjectRequest{
		Bucket:      bucket.BucketName,
		Region:      bucket.Region,
		Key:         strings.TrimSpace(input.Key),
		ContentType: strings.TrimSpace(input.ContentType),
		Content:     input.Content,
		Size:        input.Size,
		Credential:  credentialPayload,
	}); err != nil {
		return mapObjectStorageOperationError(err, "upload")
	}
	return nil
}

func (s *Service) DownloadBucketObject(ctx context.Context, projectID uint64, input DownloadBucketObjectInput) (io.ReadCloser, provider.ObjectMeta, error) {
	bucket, storageProvider, credentialPayload, err := s.resolveBucketProviderAndCredential(ctx, projectID, strings.TrimSpace(input.BucketName))
	if err != nil {
		return nil, provider.ObjectMeta{}, err
	}
	if strings.TrimSpace(input.Key) == "" {
		return nil, provider.ObjectMeta{}, httpresp.NewAppError(400, "validation_error", "object key is required", nil)
	}

	reader, meta, err := storageProvider.DownloadObject(ctx, provider.DownloadObjectRequest{
		Bucket:     bucket.BucketName,
		Region:     bucket.Region,
		Key:        strings.TrimSpace(input.Key),
		Credential: credentialPayload,
	})
	if err != nil {
		return nil, provider.ObjectMeta{}, mapObjectStorageOperationError(err, "download")
	}

	return reader, meta, nil
}

func (s *Service) DeleteBucketObject(ctx context.Context, projectID uint64, input DeleteBucketObjectInput) error {
	bucket, storageProvider, credentialPayload, err := s.resolveBucketProviderAndCredential(ctx, projectID, strings.TrimSpace(input.BucketName))
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.Key) == "" {
		return httpresp.NewAppError(400, "validation_error", "object key is required", nil)
	}

	if err := storageProvider.DeleteObject(ctx, provider.DeleteObjectRequest{
		Bucket:     bucket.BucketName,
		Region:     bucket.Region,
		Key:        strings.TrimSpace(input.Key),
		Credential: credentialPayload,
	}); err != nil {
		return mapObjectStorageOperationError(err, "delete")
	}
	return nil
}

func (s *Service) RenameBucketObject(ctx context.Context, projectID uint64, input RenameBucketObjectInput) error {
	bucket, storageProvider, credentialPayload, err := s.resolveBucketProviderAndCredential(ctx, projectID, strings.TrimSpace(input.BucketName))
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.SourceKey) == "" || strings.TrimSpace(input.TargetKey) == "" {
		return httpresp.NewAppError(400, "validation_error", "sourceKey and targetKey are required", nil)
	}

	if err := storageProvider.RenameObject(ctx, provider.RenameObjectRequest{
		Bucket:     bucket.BucketName,
		Region:     bucket.Region,
		SourceKey:  strings.TrimSpace(input.SourceKey),
		TargetKey:  strings.TrimSpace(input.TargetKey),
		Credential: credentialPayload,
	}); err != nil {
		return mapObjectStorageOperationError(err, "rename")
	}
	return nil
}

func (s *Service) replaceBindings(ctx context.Context, repos repository.Repositories, projectID uint64, buckets []ProjectBucketInput, cdns []ProjectCDNInput) error {
	existingBuckets, err := repos.ProjectBuckets().ListByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list existing buckets: %w", err)
	}
	for _, bucket := range existingBuckets {
		if err := repos.ProjectBuckets().Delete(ctx, bucket.ID); err != nil {
			return fmt.Errorf("delete existing bucket: %w", err)
		}
	}

	existingCDNs, err := repos.ProjectCDNs().ListByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list existing cdns: %w", err)
	}
	for _, cdn := range existingCDNs {
		if err := repos.ProjectCDNs().Delete(ctx, cdn.ID); err != nil {
			return fmt.Errorf("delete existing cdn: %w", err)
		}
	}

	for _, bucket := range buckets {
		credentialCiphertext := bucket.CredentialCiphertext
		if s.credentialCipher != nil {
			encrypted, err := s.credentialCipher.Encrypt(bucketCredentialPlaintext(bucket))
			if err != nil {
				return fmt.Errorf("encrypt bucket credential: %w", err)
			}
			credentialCiphertext = encrypted
		}

		if err := repos.ProjectBuckets().Create(ctx, &model.ProjectBucket{
			ProjectID:            projectID,
			ProviderType:         bucket.ProviderType,
			BucketName:           bucket.BucketName,
			Region:               bucket.Region,
			CredentialCiphertext: credentialCiphertext,
			IsPrimary:            bucket.IsPrimary,
		}); err != nil {
			return fmt.Errorf("create project bucket: %w", err)
		}
	}

	for _, cdn := range cdns {
		purgeScope := cdn.PurgeScope
		if purgeScope == "" {
			purgeScope = "url"
		}
		if err := repos.ProjectCDNs().Create(ctx, &model.ProjectCDN{
			ProjectID:    projectID,
			ProviderType: cdn.ProviderType,
			CDNEndpoint:  cdn.CDNEndpoint,
			PurgeScope:   purgeScope,
			IsPrimary:    cdn.IsPrimary,
		}); err != nil {
			return fmt.Errorf("create project cdn: %w", err)
		}
	}

	return nil
}

func validateBindings(buckets []ProjectBucketInput, cdns []ProjectCDNInput) error {
	if len(buckets) < 1 || len(buckets) > 2 {
		return httpresp.NewAppError(400, "invalid_bucket_count", "project must bind 1 or 2 buckets", nil)
	}
	if len(cdns) < 1 || len(cdns) > 2 {
		return httpresp.NewAppError(400, "invalid_cdn_count", "project must bind 1 or 2 cdn endpoints", nil)
	}

	providerType := ""
	primaryBucketCount := 0
	seenBuckets := make(map[string]struct{}, len(buckets))
	for _, bucket := range buckets {
		if bucket.ProviderType == "" || bucket.BucketName == "" || bucketCredentialPlaintext(bucket) == "" {
			return httpresp.NewAppError(400, "invalid_bucket_binding", "bucket binding is incomplete", nil)
		}
		if !model.IsKnownProviderType(bucket.ProviderType) || bucket.ProviderType == model.ProviderTypeUnknown {
			return httpresp.NewAppError(400, "invalid_provider_type", "bucket provider type is invalid", nil)
		}
		if providerType == "" {
			providerType = bucket.ProviderType
		} else if providerType != bucket.ProviderType {
			return httpresp.NewAppError(400, "inconsistent_provider_type", "all bindings must use the same provider type", nil)
		}
		if _, exists := seenBuckets[bucket.BucketName]; exists {
			return httpresp.NewAppError(400, "duplicate_bucket_binding", "bucket bindings must be unique", nil)
		}
		seenBuckets[bucket.BucketName] = struct{}{}
		if bucket.IsPrimary {
			primaryBucketCount++
		}
	}
	if primaryBucketCount != 1 {
		return httpresp.NewAppError(400, "invalid_bucket_primary_binding", "exactly one primary bucket is required", nil)
	}

	primaryCDNCount := 0
	seenCDNs := make(map[string]struct{}, len(cdns))
	for _, cdn := range cdns {
		if cdn.ProviderType == "" || cdn.CDNEndpoint == "" {
			return httpresp.NewAppError(400, "invalid_cdn_binding", "cdn binding is incomplete", nil)
		}
		if providerType == "" {
			providerType = cdn.ProviderType
		} else if providerType != cdn.ProviderType {
			return httpresp.NewAppError(400, "inconsistent_provider_type", "all bindings must use the same provider type", nil)
		}
		if _, exists := seenCDNs[cdn.CDNEndpoint]; exists {
			return httpresp.NewAppError(400, "duplicate_cdn_binding", "cdn bindings must be unique", nil)
		}
		seenCDNs[cdn.CDNEndpoint] = struct{}{}
		if cdn.IsPrimary {
			primaryCDNCount++
		}
	}
	if primaryCDNCount != 1 {
		return httpresp.NewAppError(400, "invalid_cdn_primary_binding", "exactly one primary cdn endpoint is required", nil)
	}

	return nil
}

func (s *Service) normalizeBucketsWithProviderDetection(buckets []ProjectBucketInput) ([]ProjectBucketInput, error) {
	normalized := make([]ProjectBucketInput, 0, len(buckets))
	for _, bucket := range buckets {
		resolved, err := s.normalizeBucketWithProviderDetection(bucket)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, resolved)
	}
	return normalized, nil
}

func (s *Service) normalizeBucketWithProviderDetection(bucket ProjectBucketInput) (ProjectBucketInput, error) {
	credentialPlaintext := strings.TrimSpace(bucketCredentialPlaintext(bucket))
	if credentialPlaintext == "" {
		return bucket, httpresp.NewAppError(400, "invalid_bucket_binding", "bucket credential is required", nil)
	}

	requestedProvider := strings.TrimSpace(bucket.ProviderType)
	credentialPayload := parseCredentialPayload(credentialPlaintext)
	if requestedProvider != "" && requestedProvider != model.ProviderTypeUnknown {
		if !model.IsKnownProviderType(requestedProvider) {
			return bucket, httpresp.NewAppError(400, "invalid_provider_type", "bucket provider type is invalid", nil)
		}
		if credentialPayload.CustomFields == nil {
			credentialPayload.CustomFields = map[string]string{}
		}
		credentialPayload.CustomFields["providerType"] = requestedProvider
	}
	detectedProvider, err := provider.DetectObjectStorageProvider(credentialPayload, bucket.BucketName)
	if err != nil {
		return bucket, mapProviderDetectionError(err)
	}

	if requestedProvider == "" || requestedProvider == model.ProviderTypeUnknown {
		bucket.ProviderType = detectedProvider.String()
		return bucket, nil
	}

	if requestedProvider != detectedProvider.String() {
		return bucket, httpresp.NewAppError(400, "provider_type_mismatch", "requested provider type does not match detected provider type", map[string]string{
			"requestedProviderType": requestedProvider,
			"detectedProviderType":  detectedProvider.String(),
		})
	}

	return bucket, nil
}

func parseCredentialPayload(credential string) provider.CredentialPayload {
	type credentialJSON struct {
		AccessKeyID     string            `json:"accessKeyId"`
		AccessKeySecret string            `json:"accessKeySecret"`
		SecurityToken   string            `json:"securityToken"`
		CustomFields    map[string]string `json:"customFields"`
	}

	var parsed credentialJSON
	if err := json.Unmarshal([]byte(credential), &parsed); err == nil && parsed.AccessKeyID != "" {
		return provider.CredentialPayload{
			AccessKeyID:     parsed.AccessKeyID,
			AccessKeySecret: parsed.AccessKeySecret,
			SecurityToken:   parsed.SecurityToken,
			CustomFields:    parsed.CustomFields,
		}
	}

	// Backward-compatible fallback: treat the raw credential string as access key ID.
	return provider.CredentialPayload{
		AccessKeyID: credential,
	}
}

func mapProviderDetectionError(err error) error {
	var providerErr *provider.Error
	if !errors.As(err, &providerErr) {
		return httpresp.NewAppError(400, "provider_connection_failed", "bucket connection validation failed", nil)
	}

	switch providerErr.Code {
	case provider.ErrCodeUnsupportedProvider:
		return httpresp.NewAppError(400, "provider_not_supported", "detected provider is not supported", nil)
	case provider.ErrCodeInvalidCredentials:
		return httpresp.NewAppError(400, "invalid_bucket_credential", "bucket credential is invalid", nil)
	default:
		return httpresp.NewAppError(400, "provider_connection_failed", "bucket connection validation failed", map[string]string{
			"providerErrorCode": string(providerErr.Code),
		})
	}
}

func mapObjectStorageListError(err error) error {
	var providerErr *provider.Error
	if !errors.As(err, &providerErr) {
		return httpresp.NewAppError(502, "provider_operation_failed", "list objects failed at provider boundary", nil)
	}

	switch providerErr.Code {
	case provider.ErrCodeInvalidCredentials:
		return httpresp.NewAppError(400, "invalid_bucket_credential", "bucket credential is invalid", nil)
	case provider.ErrCodeNotFound:
		return httpresp.NewAppError(404, "bucket_not_found", "bucket was not found on provider", nil)
	default:
		return httpresp.NewAppError(502, "provider_operation_failed", "list objects failed at provider boundary", map[string]string{
			"providerErrorCode": string(providerErr.Code),
		})
	}
}

func mapObjectStorageOperationError(err error, operation string) error {
	var providerErr *provider.Error
	if !errors.As(err, &providerErr) {
		return httpresp.NewAppError(502, "provider_operation_failed", "storage provider operation failed", map[string]string{
			"operation": operation,
		})
	}

	switch providerErr.Code {
	case provider.ErrCodeInvalidCredentials:
		return httpresp.NewAppError(400, "invalid_bucket_credential", "bucket credential is invalid", nil)
	case provider.ErrCodeNotFound:
		return httpresp.NewAppError(404, "storage_object_not_found", "storage object was not found", nil)
	default:
		return httpresp.NewAppError(502, "provider_operation_failed", "storage provider operation failed", map[string]string{
			"providerErrorCode": string(providerErr.Code),
			"operation":         operation,
		})
	}
}

func bucketCredentialPlaintext(bucket ProjectBucketInput) string {
	if bucket.Credential != "" {
		return bucket.Credential
	}
	return bucket.CredentialCiphertext
}

func (s *Service) bucketCredentialPayload(bucket model.ProjectBucket) (provider.CredentialPayload, error) {
	plaintext := bucket.CredentialCiphertext
	if s.credentialCipher != nil {
		decrypted, err := s.credentialCipher.Decrypt(bucket.CredentialCiphertext)
		if err == nil {
			plaintext = decrypted
		}
	}

	payload := parseCredentialPayload(plaintext)
	if strings.TrimSpace(payload.AccessKeyID) == "" {
		return provider.CredentialPayload{}, httpresp.NewAppError(400, "invalid_bucket_credential", "bucket credential is invalid", nil)
	}
	return payload, nil
}

func selectProjectBucket(project *model.Project, bucketName string) (*model.ProjectBucket, error) {
	if len(project.Buckets) == 0 {
		return nil, httpresp.NewAppError(404, "bucket_binding_not_found", "project has no bucket binding", nil)
	}

	if bucketName != "" {
		for i := range project.Buckets {
			if project.Buckets[i].BucketName == bucketName {
				return &project.Buckets[i], nil
			}
		}
		return nil, httpresp.NewAppError(404, "bucket_binding_not_found", "requested bucket binding was not found", map[string]string{
			"bucketName": bucketName,
		})
	}

	for i := range project.Buckets {
		if project.Buckets[i].IsPrimary {
			return &project.Buckets[i], nil
		}
	}

	return &project.Buckets[0], nil
}

func (s *Service) resolveBucketProviderAndCredential(ctx context.Context, projectID uint64, bucketName string) (*model.ProjectBucket, provider.ObjectStorageProvider, provider.CredentialPayload, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, nil, provider.CredentialPayload{}, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}

	bucket, err := selectProjectBucket(project, bucketName)
	if err != nil {
		return nil, nil, provider.CredentialPayload{}, err
	}

	storageProvider, err := s.providers.ObjectStorage(provider.Type(bucket.ProviderType))
	if err != nil {
		return nil, nil, provider.CredentialPayload{}, mapProviderDetectionError(err)
	}

	credentialPayload, err := s.bucketCredentialPayload(*bucket)
	if err != nil {
		return nil, nil, provider.CredentialPayload{}, err
	}

	return bucket, storageProvider, credentialPayload, nil
}

func (s *Service) maskBucketCredentials(project *model.Project) {
	if project == nil {
		return
	}
	for index := range project.Buckets {
		project.Buckets[index].CredentialCiphertext = s.maskCredential(project.Buckets[index].CredentialCiphertext)
	}
}

func (s *Service) maskCredential(ciphertext string) string {
	plaintext := ciphertext
	if s.credentialCipher != nil {
		decrypted, err := s.credentialCipher.Decrypt(ciphertext)
		if err == nil {
			plaintext = decrypted
		}
	}

	if plaintext == "" {
		return ""
	}
	if len(plaintext) <= 4 {
		return "****"
	}
	return plaintext[:2] + "****" + plaintext[len(plaintext)-2:]
}
