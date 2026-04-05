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
}

type UpdateProjectInput struct {
	Name        string
	Description string
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
	project := &model.Project{
		Name:        input.Name,
		Description: input.Description,
	}
	if err := s.projects.Create(ctx, project); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	return project, nil
}

func (s *Service) Update(ctx context.Context, projectID uint64, input UpdateProjectInput) (*model.Project, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "project_not_found", "project not found", nil)
	}

	project.Name = input.Name
	project.Description = input.Description
	if err := s.projects.Update(ctx, project); err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}

	return project, nil
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
