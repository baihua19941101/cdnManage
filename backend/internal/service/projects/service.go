package projects

import (
	"context"
	"fmt"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
)

type Service struct {
	projects repository.ProjectRepository
	tx       repository.TxManager
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
	CredentialCiphertext string
	IsPrimary            bool
}

type ProjectCDNInput struct {
	ProviderType string
	CDNEndpoint  string
	PurgeScope   string
	IsPrimary    bool
}

func NewService(projects repository.ProjectRepository, tx repository.TxManager) *Service {
	return &Service{
		projects: projects,
		tx:       tx,
	}
}

func (s *Service) List(ctx context.Context, filter repository.ProjectFilter) ([]model.Project, error) {
	return s.projects.List(ctx, filter)
}

func (s *Service) GetByID(ctx context.Context, projectID uint64) (*model.Project, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}
	return project, nil
}

func (s *Service) Create(ctx context.Context, input CreateProjectInput) (*model.Project, error) {
	if err := validateBindings(input.Buckets, input.CDNs); err != nil {
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
		return replaceBindings(ctx, repos, project.ID, input.Buckets, input.CDNs)
	}); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, project.ID)
}

func (s *Service) Update(ctx context.Context, projectID uint64, input UpdateProjectInput) (*model.Project, error) {
	if err := validateBindings(input.Buckets, input.CDNs); err != nil {
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
		return replaceBindings(ctx, repos, projectID, input.Buckets, input.CDNs)
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

func replaceBindings(ctx context.Context, repos repository.Repositories, projectID uint64, buckets []ProjectBucketInput, cdns []ProjectCDNInput) error {
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
		if err := repos.ProjectBuckets().Create(ctx, &model.ProjectBucket{
			ProjectID:            projectID,
			ProviderType:         bucket.ProviderType,
			BucketName:           bucket.BucketName,
			Region:               bucket.Region,
			CredentialCiphertext: bucket.CredentialCiphertext,
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
		if bucket.ProviderType == "" || bucket.BucketName == "" || bucket.CredentialCiphertext == "" {
			return httpresp.NewAppError(400, "invalid_bucket_binding", "bucket binding is incomplete", nil)
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
