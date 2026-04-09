package projects

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

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
	syncTaskCache    SyncTaskStatusCache
	syncTaskTTL      time.Duration
}

const (
	credentialOperationKeep    = "KEEP"
	credentialOperationReplace = "REPLACE"
)

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
	ID                   uint64
	ProviderType         string
	BucketName           string
	Region               string
	CredentialOperation  string
	Credential           string
	CredentialCiphertext string
	IsPrimary            bool
}

type ProjectCDNInput struct {
	ID                   uint64
	ProviderType         string
	CDNEndpoint          string
	Region               string
	CredentialOperation  string
	Credential           string
	CredentialCiphertext string
	PurgeScope           string
	IsPrimary            bool
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

type DeleteBucketObjectsInput struct {
	BucketName string
	Keys       []string
}

type DeleteBucketObjectResult struct {
	Key            string
	TargetType     string
	Success        bool
	DeletedObjects int
	FailedObjects  int
	ErrorCode      string
	Message        string
}

type RenameBucketObjectInput struct {
	BucketName string
	SourceKey  string
	TargetKey  string
}

type RenameBucketObjectResult struct {
	SourceKey       string
	TargetKey       string
	TargetType      string
	Success         bool
	MigratedObjects int
	FailedObjects   int
	FailureReasons  []string
	ErrorCode       string
	Message         string
}

type RefreshURLsInput struct {
	CDNEndpoint string
	URLs        []string
}

type RefreshDirectoriesInput struct {
	CDNEndpoint string
	Directories []string
}

type SyncResourcesInput struct {
	CDNEndpoint string
	BucketName  string
	Paths       []string
}

type projectCDNRepository interface {
	ListByProjectID(ctx context.Context, projectID uint64) ([]model.ProjectCDN, error)
	Delete(ctx context.Context, id uint64) error
	Create(ctx context.Context, cdn *model.ProjectCDN) error
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
		syncTaskTTL:      10 * time.Minute,
	}
}

func (s *Service) RegisterObjectStorageProvider(p provider.ObjectStorageProvider) error {
	return s.providers.RegisterObjectStorage(p)
}

func (s *Service) RegisterCDNProvider(p provider.CDNProvider) error {
	return s.providers.RegisterCDN(p)
}

func (s *Service) ConfigureSyncTaskStatusCache(cache SyncTaskStatusCache, ttl time.Duration) {
	s.syncTaskCache = cache
	if ttl > 0 {
		s.syncTaskTTL = ttl
	}
}

func (s *Service) List(ctx context.Context, filter repository.ProjectFilter) ([]model.Project, error) {
	projects, err := s.projects.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	for index := range projects {
		s.maskBucketCredentials(&projects[index])
		s.maskCDNCredentials(&projects[index])
	}

	return projects, nil
}

func (s *Service) GetByID(ctx context.Context, projectID uint64) (*model.Project, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}
	s.maskBucketCredentials(project)
	s.maskCDNCredentials(project)
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
	if err := s.validateBindingProviderRegistration(normalizedBuckets, input.CDNs); err != nil {
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
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}

	normalizedBuckets, normalizedCDNs, err := s.prepareUpdateBindings(input.Buckets, input.CDNs, project.Buckets, project.CDNs)
	if err != nil {
		return nil, err
	}
	if err := validateBindings(normalizedBuckets, normalizedCDNs); err != nil {
		return nil, err
	}
	if err := s.validateBindingProviderRegistration(normalizedBuckets, normalizedCDNs); err != nil {
		return nil, err
	}

	project.Name = input.Name
	project.Description = input.Description
	if err := s.tx.WithinTransaction(ctx, func(repos repository.Repositories) error {
		if err := repos.Projects().Update(ctx, project); err != nil {
			return fmt.Errorf("update project: %w", err)
		}
		return s.replaceBindings(ctx, repos, projectID, normalizedBuckets, normalizedCDNs)
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

func (s *Service) GetCDNs(ctx context.Context, projectID uint64) ([]model.ProjectCDN, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}
	return project.CDNs, nil
}

func (s *Service) UpdateCDNs(ctx context.Context, projectID uint64, cdns []ProjectCDNInput) ([]model.ProjectCDN, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}
	if err := validateCDNBindings(cdns); err != nil {
		return nil, err
	}
	if err := s.validateBindingProviderRegistration(nil, cdns); err != nil {
		return nil, err
	}

	if err := s.tx.WithinTransaction(ctx, func(repos repository.Repositories) error {
		project.Name = project.Name
		return s.replaceCDNBindings(ctx, repos.ProjectCDNs(), projectID, cdns)
	}); err != nil {
		return nil, err
	}

	updated, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}
	return updated.CDNs, nil
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
	_, err := s.DeleteBucketObjectWithResult(ctx, projectID, input)
	return err
}

func (s *Service) DeleteBucketObjectWithResult(ctx context.Context, projectID uint64, input DeleteBucketObjectInput) (DeleteBucketObjectResult, error) {
	bucket, storageProvider, credentialPayload, err := s.resolveBucketProviderAndCredential(ctx, projectID, strings.TrimSpace(input.BucketName))
	if err != nil {
		return DeleteBucketObjectResult{}, err
	}
	key := strings.TrimSpace(input.Key)
	if key == "" {
		return DeleteBucketObjectResult{}, httpresp.NewAppError(400, "validation_error", "object key is required", nil)
	}
	if strings.HasSuffix(key, "/") {
		return s.deleteDirectoryRecursively(ctx, storageProvider, provider.DeleteObjectRequest{
			Bucket:     bucket.BucketName,
			Region:     bucket.Region,
			Key:        key,
			Credential: credentialPayload,
		}), nil
	}

	if err := storageProvider.DeleteObject(ctx, provider.DeleteObjectRequest{
		Bucket:     bucket.BucketName,
		Region:     bucket.Region,
		Key:        key,
		Credential: credentialPayload,
	}); err != nil {
		return DeleteBucketObjectResult{}, mapObjectStorageOperationError(err, "delete")
	}
	return DeleteBucketObjectResult{
		Key:            key,
		TargetType:     "file",
		Success:        true,
		DeletedObjects: 1,
		Message:        "object deleted",
	}, nil
}

func (s *Service) deleteDirectoryRecursively(ctx context.Context, storageProvider provider.ObjectStorageProvider, request provider.DeleteObjectRequest) DeleteBucketObjectResult {
	prefix := strings.TrimSpace(request.Key)
	result := DeleteBucketObjectResult{
		Key:        prefix,
		TargetType: "directory",
		Success:    true,
	}

	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	queue := []string{prefix}
	visited := make(map[string]struct{}, 8)
	failureReasons := make([]string, 0, 3)
	const maxKeys = 200

	for len(queue) > 0 {
		currentPrefix := queue[0]
		queue = queue[1:]
		if _, ok := visited[currentPrefix]; ok {
			continue
		}
		visited[currentPrefix] = struct{}{}

		marker := ""
		for {
			objects, err := storageProvider.ListObjects(ctx, provider.ListObjectsRequest{
				Bucket:     request.Bucket,
				Region:     request.Region,
				Prefix:     currentPrefix,
				Marker:     marker,
				MaxKeys:    maxKeys,
				Credential: request.Credential,
			})
			if err != nil {
				result.Success = false
				result.ErrorCode = "provider_operation_failed"
				result.Message = mapObjectStorageListError(err).Error()
				return result
			}
			if len(objects) == 0 {
				break
			}

			lastMarker := marker
			for _, object := range objects {
				lastMarker = object.Key
				if object.IsDir {
					normalized := strings.TrimSpace(object.Key)
					if normalized != "" {
						queue = append(queue, normalized)
					}
					continue
				}
				err = storageProvider.DeleteObject(ctx, provider.DeleteObjectRequest{
					Bucket:     request.Bucket,
					Region:     request.Region,
					Key:        object.Key,
					Credential: request.Credential,
				})
				if err != nil {
					result.Success = false
					result.FailedObjects++
					if len(failureReasons) < 3 {
						failureReasons = append(failureReasons, object.Key+": "+mapObjectStorageOperationError(err, "delete").Error())
					}
					continue
				}
				result.DeletedObjects++
			}

			if len(objects) < maxKeys || strings.TrimSpace(lastMarker) == "" || lastMarker == marker {
				break
			}
			marker = lastMarker
		}
	}

	if !result.Success {
		if len(failureReasons) > 0 {
			result.Message = "directory delete partially failed: " + strings.Join(failureReasons, "; ")
		} else {
			result.Message = "directory delete partially failed"
		}
		return result
	}
	result.Message = "directory deleted"
	return result
}

func (s *Service) DeleteBucketObjects(ctx context.Context, projectID uint64, input DeleteBucketObjectsInput) ([]DeleteBucketObjectResult, error) {
	bucket, storageProvider, credentialPayload, err := s.resolveBucketProviderAndCredential(ctx, projectID, strings.TrimSpace(input.BucketName))
	if err != nil {
		return nil, err
	}

	keys := trimNonEmptyValues(input.Keys)
	if len(keys) == 0 {
		return nil, httpresp.NewAppError(400, "validation_error", "at least one object key is required", nil)
	}

	results := make([]DeleteBucketObjectResult, 0, len(keys))
	for _, key := range keys {
		if strings.HasSuffix(key, "/") {
			result := s.deleteDirectoryRecursively(ctx, storageProvider, provider.DeleteObjectRequest{
				Bucket:     bucket.BucketName,
				Region:     bucket.Region,
				Key:        key,
				Credential: credentialPayload,
			})
			if result.Key == "" {
				result.Key = key
			}
			if result.TargetType == "" {
				result.TargetType = "directory"
			}
			results = append(results, result)
			continue
		}

		deleteErr := storageProvider.DeleteObject(ctx, provider.DeleteObjectRequest{
			Bucket:     bucket.BucketName,
			Region:     bucket.Region,
			Key:        key,
			Credential: credentialPayload,
		})
		if deleteErr != nil {
			mappedErr := mapObjectStorageOperationError(deleteErr, "delete")
			result := DeleteBucketObjectResult{
				Key:        key,
				TargetType: "file",
				Success:    false,
				ErrorCode:  "provider_operation_failed",
				Message:    mappedErr.Error(),
			}
			appErr := &httpresp.AppError{}
			if errors.As(mappedErr, &appErr) {
				result.ErrorCode = appErr.Code
			}
			results = append(results, result)
			continue
		}

		results = append(results, DeleteBucketObjectResult{
			Key:            key,
			TargetType:     "file",
			Success:        true,
			DeletedObjects: 1,
			Message:        "object deleted",
		})
	}

	return results, nil
}

func (s *Service) RenameBucketObject(ctx context.Context, projectID uint64, input RenameBucketObjectInput) (RenameBucketObjectResult, error) {
	bucket, storageProvider, credentialPayload, err := s.resolveBucketProviderAndCredential(ctx, projectID, strings.TrimSpace(input.BucketName))
	if err != nil {
		return RenameBucketObjectResult{}, err
	}
	sourceKey := strings.TrimSpace(input.SourceKey)
	targetKey := strings.TrimSpace(input.TargetKey)
	if sourceKey == "" || targetKey == "" {
		return RenameBucketObjectResult{}, httpresp.NewAppError(400, "validation_error", "sourceKey and targetKey are required", nil)
	}

	if strings.HasSuffix(sourceKey, "/") {
		return s.renameDirectoryRecursively(ctx, storageProvider, provider.RenameObjectRequest{
			Bucket:     bucket.BucketName,
			Region:     bucket.Region,
			SourceKey:  sourceKey,
			TargetKey:  targetKey,
			Credential: credentialPayload,
		})
	}

	if err := storageProvider.RenameObject(ctx, provider.RenameObjectRequest{
		Bucket:     bucket.BucketName,
		Region:     bucket.Region,
		SourceKey:  sourceKey,
		TargetKey:  targetKey,
		Credential: credentialPayload,
	}); err != nil {
		return RenameBucketObjectResult{}, mapObjectStorageOperationError(err, "rename")
	}
	return RenameBucketObjectResult{
		SourceKey:       sourceKey,
		TargetKey:       targetKey,
		TargetType:      "file",
		Success:         true,
		MigratedObjects: 1,
		Message:         "object renamed",
	}, nil
}

func (s *Service) renameDirectoryRecursively(
	ctx context.Context,
	storageProvider provider.ObjectStorageProvider,
	request provider.RenameObjectRequest,
) (RenameBucketObjectResult, error) {
	sourcePrefix := ensureDirectoryPrefix(request.SourceKey)
	targetPrefix := ensureDirectoryPrefix(request.TargetKey)
	if sourcePrefix == "" || targetPrefix == "" {
		return RenameBucketObjectResult{}, httpresp.NewAppError(400, "validation_error", "directory sourceKey and targetKey are required", nil)
	}
	if sourcePrefix == targetPrefix {
		return RenameBucketObjectResult{}, httpresp.NewAppError(400, "validation_error", "directory sourceKey and targetKey must be different", nil)
	}

	result := RenameBucketObjectResult{
		SourceKey:      sourcePrefix,
		TargetKey:      targetPrefix,
		TargetType:     "directory",
		Success:        true,
		FailureReasons: make([]string, 0, 3),
	}

	queue := []string{sourcePrefix}
	visitedDirectories := make(map[string]struct{}, 8)
	processedObjects := make(map[string]struct{}, 64)
	const maxKeys = 200

	for len(queue) > 0 {
		currentPrefix := queue[0]
		queue = queue[1:]
		if _, ok := visitedDirectories[currentPrefix]; ok {
			continue
		}
		visitedDirectories[currentPrefix] = struct{}{}

		marker := ""
		for {
			objects, err := storageProvider.ListObjects(ctx, provider.ListObjectsRequest{
				Bucket:     request.Bucket,
				Region:     request.Region,
				Prefix:     currentPrefix,
				Marker:     marker,
				MaxKeys:    maxKeys,
				Credential: request.Credential,
			})
			if err != nil {
				return RenameBucketObjectResult{}, mapObjectStorageListError(err)
			}
			if len(objects) == 0 {
				break
			}

			lastMarker := marker
			for _, object := range objects {
				lastMarker = object.Key
				if object.IsDir {
					normalized := ensureDirectoryPrefix(object.Key)
					if normalized != "" {
						queue = append(queue, normalized)
					}
					continue
				}
				if _, duplicated := processedObjects[object.Key]; duplicated {
					continue
				}
				processedObjects[object.Key] = struct{}{}

				if !strings.HasPrefix(object.Key, sourcePrefix) {
					continue
				}

				relativePath := strings.TrimPrefix(object.Key, sourcePrefix)
				targetObjectKey := targetPrefix + relativePath
				err = storageProvider.RenameObject(ctx, provider.RenameObjectRequest{
					Bucket:     request.Bucket,
					Region:     request.Region,
					SourceKey:  object.Key,
					TargetKey:  targetObjectKey,
					Credential: request.Credential,
				})
				if err != nil {
					result.Success = false
					result.FailedObjects++
					if len(result.FailureReasons) < 3 {
						result.FailureReasons = append(result.FailureReasons, object.Key+": "+mapObjectStorageOperationError(err, "rename").Error())
					}
					continue
				}
				result.MigratedObjects++
			}

			if len(objects) < maxKeys || strings.TrimSpace(lastMarker) == "" || lastMarker == marker {
				break
			}
			marker = lastMarker
		}
	}

	if result.Success {
		result.Message = "directory renamed"
		return result, nil
	}

	result.ErrorCode = "provider_operation_failed"
	result.Message = "directory rename partially failed"
	return result, nil
}

func ensureDirectoryPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasSuffix(trimmed, "/") {
		return trimmed
	}
	return trimmed + "/"
}

func (s *Service) RefreshURLs(ctx context.Context, projectID uint64, input RefreshURLsInput) (provider.TaskResult, error) {
	cdnBinding, cdnProvider, credentialPayload, err := s.resolveCDNProviderAndCredential(ctx, projectID, strings.TrimSpace(input.CDNEndpoint))
	if err != nil {
		return provider.TaskResult{}, err
	}

	urls := trimNonEmptyValues(input.URLs)
	if len(urls) == 0 {
		return provider.TaskResult{}, httpresp.NewAppError(400, "validation_error", "at least one url is required", nil)
	}

	result, err := cdnProvider.RefreshURLs(ctx, provider.RefreshURLsRequest{
		Endpoint:   cdnBinding.CDNEndpoint,
		URLs:       urls,
		Credential: credentialPayload,
	})
	if err != nil {
		return provider.TaskResult{}, mapCDNOperationError(err, "refresh_urls")
	}

	return result, nil
}

func (s *Service) RefreshDirectories(ctx context.Context, projectID uint64, input RefreshDirectoriesInput) (provider.TaskResult, error) {
	cdnBinding, cdnProvider, credentialPayload, err := s.resolveCDNProviderAndCredential(ctx, projectID, strings.TrimSpace(input.CDNEndpoint))
	if err != nil {
		return provider.TaskResult{}, err
	}

	directories := trimNonEmptyValues(input.Directories)
	if len(directories) == 0 {
		return provider.TaskResult{}, httpresp.NewAppError(400, "validation_error", "at least one directory is required", nil)
	}

	result, err := cdnProvider.RefreshDirectories(ctx, provider.RefreshDirectoriesRequest{
		Endpoint:    cdnBinding.CDNEndpoint,
		Directories: directories,
		Credential:  credentialPayload,
	})
	if err != nil {
		return provider.TaskResult{}, mapCDNOperationError(err, "refresh_directories")
	}

	return result, nil
}

func (s *Service) SyncResources(ctx context.Context, projectID uint64, input SyncResourcesInput) (provider.TaskResult, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return provider.TaskResult{}, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}

	cdnBinding, err := selectProjectCDN(project, strings.TrimSpace(input.CDNEndpoint))
	if err != nil {
		return provider.TaskResult{}, err
	}

	cdnProvider, err := s.providers.CDN(provider.Type(cdnBinding.ProviderType))
	if err != nil {
		return provider.TaskResult{}, mapCDNOperationError(err, "sync_resources")
	}

	bucket, err := selectProjectBucket(project, strings.TrimSpace(input.BucketName))
	if err != nil {
		return provider.TaskResult{}, err
	}

	credentialPayload, err := s.bucketCredentialPayload(*bucket)
	if err != nil {
		return provider.TaskResult{}, err
	}

	paths := trimNonEmptyValues(input.Paths)
	if len(paths) == 0 {
		return provider.TaskResult{}, httpresp.NewAppError(400, "validation_error", "at least one path is required", nil)
	}

	result, err := cdnProvider.SyncLatestResources(ctx, provider.SyncResourcesRequest{
		Endpoint:      cdnBinding.CDNEndpoint,
		Bucket:        bucket.BucketName,
		Region:        bucket.Region,
		Paths:         paths,
		Credential:    credentialPayload,
		InvalidateCDN: true,
	})
	if err != nil {
		return provider.TaskResult{}, mapCDNOperationError(err, "sync_resources")
	}

	if result.SubmittedAt.IsZero() {
		result.SubmittedAt = time.Now().UTC()
	}
	if strings.TrimSpace(result.Status) == "" {
		result.Status = "accepted"
	}
	if strings.TrimSpace(result.TaskID) == "" {
		result.TaskID = newSyncTaskID()
	}

	if s.syncTaskCache != nil {
		_ = s.syncTaskCache.Set(ctx, result.TaskID, SyncTaskStatus{
			TaskID:            result.TaskID,
			ProjectID:         projectID,
			BucketName:        bucket.BucketName,
			CDNEndpoint:       cdnBinding.CDNEndpoint,
			Paths:             paths,
			Status:            result.Status,
			ProviderRequestID: result.ProviderRequestID,
			SubmittedAt:       result.SubmittedAt,
			CompletedAt:       result.CompletedAt,
			Metadata:          result.Metadata,
		}, s.syncTaskTTL)
	}

	return result, nil
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

	if err := s.replaceCDNBindings(ctx, repos.ProjectCDNs(), projectID, cdns); err != nil {
		return err
	}

	for _, bucket := range buckets {
		credentialCiphertext, err := s.resolveBucketCredentialCiphertext(bucket)
		if err != nil {
			return err
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

	return nil
}

func validateBindings(buckets []ProjectBucketInput, cdns []ProjectCDNInput) error {
	if len(buckets) > 2 {
		return httpresp.NewAppError(400, "invalid_bucket_count", "project can bind at most 2 buckets", nil)
	}

	if err := validateBucketBindings(buckets); err != nil {
		return err
	}

	return validateCDNBindings(cdns)
}

func validateBucketBindings(buckets []ProjectBucketInput) error {
	if len(buckets) == 0 {
		return nil
	}

	primaryBucketCount := 0
	seenBuckets := make(map[string]struct{}, len(buckets))
	for _, bucket := range buckets {
		bucketName := strings.TrimSpace(bucket.BucketName)
		credential := strings.TrimSpace(bucketCredentialPlaintext(bucket))
		if bucket.ProviderType == "" || bucketName == "" || credential == "" {
			return httpresp.NewAppError(400, "invalid_bucket_binding", "bucket binding is incomplete", nil)
		}
		if !model.IsKnownProviderType(bucket.ProviderType) || bucket.ProviderType == model.ProviderTypeUnknown {
			return httpresp.NewAppError(400, "invalid_provider_type", "bucket provider type is invalid", nil)
		}
		if _, exists := seenBuckets[bucketName]; exists {
			return httpresp.NewAppError(400, "duplicate_bucket_binding", "bucket bindings must be unique", nil)
		}
		seenBuckets[bucketName] = struct{}{}
		if bucket.IsPrimary {
			primaryBucketCount++
		}
	}
	if primaryBucketCount != 1 {
		return httpresp.NewAppError(400, "invalid_bucket_primary_binding", "exactly one primary bucket is required", nil)
	}

	return nil
}

func validateCDNBindings(cdns []ProjectCDNInput) error {
	if len(cdns) > 2 {
		return httpresp.NewAppError(400, "invalid_cdn_count", "project can bind at most 2 cdn endpoints", nil)
	}
	if len(cdns) == 0 {
		return nil
	}

	primaryCDNCount := 0
	seenCDNs := make(map[string]struct{}, len(cdns))
	for _, cdn := range cdns {
		if cdn.ProviderType == "" || cdn.CDNEndpoint == "" {
			return httpresp.NewAppError(400, "invalid_cdn_binding", "cdn binding is incomplete", nil)
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

func (s *Service) validateBindingProviderRegistration(buckets []ProjectBucketInput, cdns []ProjectCDNInput) error {
	for index, bucket := range buckets {
		providerType := strings.TrimSpace(bucket.ProviderType)
		if providerType == "" {
			continue
		}
		if _, err := s.providers.ObjectStorage(provider.Type(providerType)); err != nil {
			return bindingProviderNotRegisteredError("buckets", index, providerType, "object_storage")
		}
	}

	for index, cdn := range cdns {
		providerType := strings.TrimSpace(cdn.ProviderType)
		if providerType == "" {
			continue
		}
		if _, err := s.providers.CDN(provider.Type(providerType)); err != nil {
			return bindingProviderNotRegisteredError("cdns", index, providerType, "cdn")
		}
	}

	return nil
}

func bindingProviderNotRegisteredError(bindingType string, index int, providerType, providerService string) error {
	path := fmt.Sprintf("%s[%d].providerType", bindingType, index)
	return httpresp.NewAppError(400, "provider_not_registered", "binding provider is not registered", map[string]any{
		"bindingType":     bindingType,
		"bindingIndex":    index,
		"bindingPath":     path,
		"providerType":    providerType,
		"providerService": providerService,
	})
}

func projectBucketInputProviderType(buckets []ProjectBucketInput) string {
	providerType := ""
	primaryBucketCount := 0
	seenBuckets := make(map[string]struct{}, len(buckets))
	for _, bucket := range buckets {
		if bucket.ProviderType == "" || bucket.BucketName == "" || bucketCredentialPlaintext(bucket) == "" {
			return ""
		}
		if !model.IsKnownProviderType(bucket.ProviderType) || bucket.ProviderType == model.ProviderTypeUnknown {
			return ""
		}
		if providerType == "" {
			providerType = bucket.ProviderType
		} else if providerType != bucket.ProviderType {
			return ""
		}
		if _, exists := seenBuckets[bucket.BucketName]; exists {
			return ""
		}
		seenBuckets[bucket.BucketName] = struct{}{}
		if bucket.IsPrimary {
			primaryBucketCount++
		}
	}
	if primaryBucketCount != 1 {
		return ""
	}
	return providerType
}

func projectBucketProviderType(buckets []model.ProjectBucket) string {
	providerType := ""
	for _, bucket := range buckets {
		if bucket.IsPrimary {
			return bucket.ProviderType
		}
		if providerType == "" {
			providerType = bucket.ProviderType
		}
	}
	return providerType
}

func (s *Service) replaceCDNBindings(ctx context.Context, repo projectCDNRepository, projectID uint64, cdns []ProjectCDNInput) error {
	existingCDNs, err := repo.ListByProjectID(ctx, projectID)
	if err != nil {
		return fmt.Errorf("list existing cdns: %w", err)
	}
	for _, cdn := range existingCDNs {
		if err := repo.Delete(ctx, cdn.ID); err != nil {
			return fmt.Errorf("delete existing cdn: %w", err)
		}
	}
	for _, cdn := range cdns {
		credentialCiphertext, err := s.resolveCDNCredentialCiphertext(cdn)
		if err != nil {
			return err
		}

		purgeScope := cdn.PurgeScope
		if purgeScope == "" {
			purgeScope = "url"
		}
		if err := repo.Create(ctx, &model.ProjectCDN{
			ProjectID:            projectID,
			ProviderType:         cdn.ProviderType,
			CDNEndpoint:          cdn.CDNEndpoint,
			Region:               cdn.Region,
			CredentialCiphertext: credentialCiphertext,
			PurgeScope:           purgeScope,
			IsPrimary:            cdn.IsPrimary,
		}); err != nil {
			return fmt.Errorf("create project cdn: %w", err)
		}
	}
	return nil
}

func (s *Service) resolveBucketCredentialCiphertext(bucket ProjectBucketInput) (string, error) {
	if normalizeCredentialOperation(bucket.CredentialOperation) == credentialOperationKeep {
		return bucket.CredentialCiphertext, nil
	}

	credentialCiphertext := bucket.CredentialCiphertext
	if s.credentialCipher != nil {
		encrypted, err := s.credentialCipher.Encrypt(bucketCredentialPlaintext(bucket))
		if err != nil {
			return "", fmt.Errorf("encrypt bucket credential: %w", err)
		}
		credentialCiphertext = encrypted
	}
	return credentialCiphertext, nil
}

func (s *Service) resolveCDNCredentialCiphertext(cdn ProjectCDNInput) (string, error) {
	if normalizeCredentialOperation(cdn.CredentialOperation) == credentialOperationKeep {
		return cdn.CredentialCiphertext, nil
	}

	credentialCiphertext := cdn.CredentialCiphertext
	if s.credentialCipher != nil {
		encrypted, err := s.credentialCipher.Encrypt(cdnCredentialPlaintext(cdn))
		if err != nil {
			return "", fmt.Errorf("encrypt cdn credential: %w", err)
		}
		credentialCiphertext = encrypted
	}
	return credentialCiphertext, nil
}

func (s *Service) prepareUpdateBindings(
	buckets []ProjectBucketInput,
	cdns []ProjectCDNInput,
	existingBuckets []model.ProjectBucket,
	existingCDNs []model.ProjectCDN,
) ([]ProjectBucketInput, []ProjectCDNInput, error) {
	bucketMap := make(map[uint64]model.ProjectBucket, len(existingBuckets))
	for _, bucket := range existingBuckets {
		bucketMap[bucket.ID] = bucket
	}

	resolvedBuckets := make([]ProjectBucketInput, 0, len(buckets))
	for index, bucket := range buckets {
		existingBucket, hasExisting := bucketMap[bucket.ID]
		operation := normalizeCredentialOperation(bucket.CredentialOperation)
		if hasExisting && operation == "" {
			operation = credentialOperationKeep
		}

		if !hasExisting && operation != credentialOperationReplace {
			return nil, nil, bindingCredentialOperationError("credential_missing_for_new_binding", "new binding requires credential replacement", "buckets", index)
		}

		if hasExisting && !isValidCredentialOperation(operation) {
			return nil, nil, invalidCredentialOperationError("buckets", index, operation)
		}

		if hasExisting {
			if strings.TrimSpace(bucket.ProviderType) == "" {
				bucket.ProviderType = existingBucket.ProviderType
			}

			if !strings.EqualFold(bucket.ProviderType, existingBucket.ProviderType) && operation != credentialOperationReplace {
				return nil, nil, bindingCredentialOperationError("provider_change_requires_credential_replace", "provider type change requires credential replacement", "buckets", index)
			}

			if operation == credentialOperationKeep {
				if strings.TrimSpace(existingBucket.CredentialCiphertext) == "" {
					return nil, nil, bindingCredentialOperationError("credential_not_found_for_keep", "historical credential was not found for keep operation", "buckets", index)
				}
				bucket.Credential = ""
				bucket.CredentialCiphertext = existingBucket.CredentialCiphertext
				bucket.CredentialOperation = credentialOperationKeep
				resolvedBuckets = append(resolvedBuckets, bucket)
				continue
			}
		}

		normalizedBucket, err := s.normalizeBucketWithProviderDetection(bucket)
		if err != nil {
			return nil, nil, err
		}
		normalizedBucket.CredentialOperation = credentialOperationReplace
		resolvedBuckets = append(resolvedBuckets, normalizedBucket)
	}

	cdnMap := make(map[uint64]model.ProjectCDN, len(existingCDNs))
	for _, cdn := range existingCDNs {
		cdnMap[cdn.ID] = cdn
	}

	resolvedCDNs := make([]ProjectCDNInput, 0, len(cdns))
	for index, cdn := range cdns {
		existingCDN, hasExisting := cdnMap[cdn.ID]
		operation := normalizeCredentialOperation(cdn.CredentialOperation)
		if hasExisting && operation == "" {
			operation = credentialOperationKeep
		}

		if !hasExisting && operation != credentialOperationReplace {
			return nil, nil, bindingCredentialOperationError("credential_missing_for_new_binding", "new binding requires credential replacement", "cdns", index)
		}

		if hasExisting && !isValidCredentialOperation(operation) {
			return nil, nil, invalidCredentialOperationError("cdns", index, operation)
		}

		if hasExisting {
			if strings.TrimSpace(cdn.ProviderType) == "" {
				cdn.ProviderType = existingCDN.ProviderType
			}

			if !strings.EqualFold(cdn.ProviderType, existingCDN.ProviderType) && operation != credentialOperationReplace {
				return nil, nil, bindingCredentialOperationError("provider_change_requires_credential_replace", "provider type change requires credential replacement", "cdns", index)
			}

			if operation == credentialOperationKeep {
				if strings.TrimSpace(existingCDN.CredentialCiphertext) == "" {
					return nil, nil, bindingCredentialOperationError("credential_not_found_for_keep", "historical credential was not found for keep operation", "cdns", index)
				}
				cdn.Credential = ""
				cdn.CredentialCiphertext = existingCDN.CredentialCiphertext
				cdn.CredentialOperation = credentialOperationKeep
				resolvedCDNs = append(resolvedCDNs, cdn)
				continue
			}
		}

		if strings.TrimSpace(cdnCredentialPlaintext(cdn)) == "" {
			return nil, nil, httpresp.NewAppError(400, "invalid_cdn_binding", "cdn credential is required", map[string]any{
				"bindingType":  "cdns",
				"bindingIndex": index,
				"bindingPath":  fmt.Sprintf("cdns[%d].credential", index),
			})
		}
		cdn.CredentialOperation = credentialOperationReplace
		resolvedCDNs = append(resolvedCDNs, cdn)
	}

	return resolvedBuckets, resolvedCDNs, nil
}

func normalizeCredentialOperation(operation string) string {
	return strings.ToUpper(strings.TrimSpace(operation))
}

func isValidCredentialOperation(operation string) bool {
	return operation == credentialOperationKeep || operation == credentialOperationReplace
}

func bindingCredentialOperationError(code, message, bindingType string, index int) error {
	return httpresp.NewAppError(400, code, message, map[string]any{
		"bindingType":  bindingType,
		"bindingIndex": index,
		"bindingPath":  fmt.Sprintf("%s[%d].credentialOperation", bindingType, index),
	})
}

func invalidCredentialOperationError(bindingType string, index int, operation string) error {
	return httpresp.NewAppError(400, "invalid_credential_operation", "credential operation is invalid", map[string]any{
		"bindingType":         bindingType,
		"bindingIndex":        index,
		"bindingPath":         fmt.Sprintf("%s[%d].credentialOperation", bindingType, index),
		"credentialOperation": operation,
		"allowedOperations":   []string{credentialOperationKeep, credentialOperationReplace},
	})
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

func mapCDNOperationError(err error, operation string) error {
	var providerErr *provider.Error
	if !errors.As(err, &providerErr) {
		return httpresp.NewAppError(502, "cdn_operation_failed", "cdn provider operation failed", map[string]string{
			"operation": operation,
		})
	}

	switch providerErr.Code {
	case provider.ErrCodeInvalidCredentials:
		return httpresp.NewAppError(400, "invalid_bucket_credential", "bucket credential is invalid", nil)
	case provider.ErrCodeInvalidRequest:
		return httpresp.NewAppError(400, "invalid_cdn_request", "cdn refresh request is invalid", map[string]string{
			"operation": operation,
		})
	case provider.ErrCodeTimeout:
		return httpresp.NewAppError(504, "cdn_request_timeout", "cdn provider request timed out", map[string]string{
			"operation": operation,
		})
	default:
		return httpresp.NewAppError(502, "cdn_operation_failed", "cdn provider operation failed", map[string]string{
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

func cdnCredentialPlaintext(cdn ProjectCDNInput) string {
	if cdn.Credential != "" {
		return cdn.Credential
	}
	return cdn.CredentialCiphertext
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

func selectProjectCDN(project *model.Project, cdnEndpoint string) (*model.ProjectCDN, error) {
	if len(project.CDNs) == 0 {
		return nil, httpresp.NewAppError(404, "cdn_binding_not_found", "project has no cdn binding", nil)
	}

	if cdnEndpoint != "" {
		for i := range project.CDNs {
			if project.CDNs[i].CDNEndpoint == cdnEndpoint {
				return &project.CDNs[i], nil
			}
		}
		return nil, httpresp.NewAppError(404, "cdn_binding_not_found", "requested cdn binding was not found", map[string]string{
			"cdnEndpoint": cdnEndpoint,
		})
	}

	for i := range project.CDNs {
		if project.CDNs[i].IsPrimary {
			return &project.CDNs[i], nil
		}
	}

	return &project.CDNs[0], nil
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

func (s *Service) resolveCDNProviderAndCredential(ctx context.Context, projectID uint64, cdnEndpoint string) (*model.ProjectCDN, provider.CDNProvider, provider.CredentialPayload, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, nil, provider.CredentialPayload{}, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}

	cdnBinding, err := selectProjectCDN(project, cdnEndpoint)
	if err != nil {
		return nil, nil, provider.CredentialPayload{}, err
	}

	cdnProvider, err := s.providers.CDN(provider.Type(cdnBinding.ProviderType))
	if err != nil {
		return nil, nil, provider.CredentialPayload{}, mapCDNOperationError(err, "resolve_provider")
	}

	credentialPayload, err := s.cdnCredentialPayload(*cdnBinding)
	if err != nil {
		return nil, nil, provider.CredentialPayload{}, err
	}

	return cdnBinding, cdnProvider, credentialPayload, nil
}

func (s *Service) cdnCredentialPayload(cdn model.ProjectCDN) (provider.CredentialPayload, error) {
	plaintext := cdn.CredentialCiphertext
	if s.credentialCipher != nil {
		decrypted, err := s.credentialCipher.Decrypt(cdn.CredentialCiphertext)
		if err == nil {
			plaintext = decrypted
		}
	}

	payload := parseCredentialPayload(plaintext)
	if strings.TrimSpace(payload.AccessKeyID) == "" {
		return provider.CredentialPayload{}, httpresp.NewAppError(400, "invalid_cdn_credential", "cdn credential is invalid", nil)
	}
	if strings.TrimSpace(cdn.Region) != "" {
		if payload.CustomFields == nil {
			payload.CustomFields = map[string]string{}
		}
		payload.CustomFields["region"] = cdn.Region
	}
	return payload, nil
}

func trimNonEmptyValues(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func newSyncTaskID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("sync-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func (s *Service) maskBucketCredentials(project *model.Project) {
	if project == nil {
		return
	}
	for index := range project.Buckets {
		project.Buckets[index].CredentialCiphertext = s.maskCredential(project.Buckets[index].CredentialCiphertext)
	}
}

func (s *Service) maskCDNCredentials(project *model.Project) {
	if project == nil {
		return
	}
	for index := range project.CDNs {
		project.CDNs[index].CredentialCiphertext = s.maskCredential(project.CDNs[index].CredentialCiphertext)
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
