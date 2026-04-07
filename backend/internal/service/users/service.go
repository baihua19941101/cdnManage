package users

import (
	"context"
	"fmt"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
)

type Service struct {
	users    repository.UserRepository
	projects repository.ProjectRepository
	tx       repository.TxManager
}

type CreateUserInput struct {
	Username     string
	Email        string
	Password     string
	Status       string
	PlatformRole string
}

type UpdateUserInput struct {
	Username     string
	Email        string
	Status       string
	PlatformRole string
}

type ProjectBindingInput struct {
	ProjectID   uint64
	ProjectRole string
}

const minPasswordLength = 8

func NewService(users repository.UserRepository, projects repository.ProjectRepository, tx repository.TxManager) *Service {
	return &Service{
		users:    users,
		projects: projects,
		tx:       tx,
	}
}

func (s *Service) List(ctx context.Context, filter repository.UserFilter) ([]model.User, error) {
	return s.users.List(ctx, filter)
}

func (s *Service) Create(ctx context.Context, input CreateUserInput) (*model.User, error) {
	if err := validateUserStatus(input.Status); err != nil {
		return nil, err
	}
	if err := validatePlatformRole(input.PlatformRole); err != nil {
		return nil, err
	}

	passwordHash, err := serviceauth.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash user password: %w", err)
	}

	user := &model.User{
		Username:     input.Username,
		Email:        input.Email,
		PasswordHash: passwordHash,
		Status:       input.Status,
		PlatformRole: input.PlatformRole,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *Service) Update(ctx context.Context, userID uint64, input UpdateUserInput) (*model.User, error) {
	if err := validateUserStatus(input.Status); err != nil {
		return nil, err
	}
	if err := validatePlatformRole(input.PlatformRole); err != nil {
		return nil, err
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, httpresp.NewAppError(404, "user_not_found", "user not found", nil)
	}

	user.Username = input.Username
	user.Email = input.Email
	user.Status = input.Status
	user.PlatformRole = input.PlatformRole

	if err := s.users.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return user, nil
}

func (s *Service) Delete(ctx context.Context, userID uint64) error {
	_, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return httpresp.NewAppError(404, "user_not_found", "user not found", nil)
	}

	return s.tx.WithinTransaction(ctx, func(repos repository.Repositories) error {
		if err := repos.UserProjectRoles().DeleteByUserID(ctx, userID); err != nil {
			return fmt.Errorf("delete user project bindings: %w", err)
		}
		if err := repos.Users().Delete(ctx, userID); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		return nil
	})
}

func (s *Service) ResetPassword(ctx context.Context, userID uint64, newPassword string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return httpresp.NewAppError(404, "user_not_found", "user not found", nil)
	}

	if len(newPassword) < minPasswordLength {
		return httpresp.NewAppError(400, "password_policy_violation", "new password must be at least 8 characters", map[string]interface{}{"minLength": minPasswordLength})
	}

	passwordHash, err := serviceauth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash reset password: %w", err)
	}

	user.PasswordHash = passwordHash
	if err := s.users.Update(ctx, user); err != nil {
		return fmt.Errorf("update user password: %w", err)
	}

	return nil
}

func (s *Service) ReplaceProjectBindings(ctx context.Context, userID uint64, bindings []ProjectBindingInput) ([]model.UserProjectRole, error) {
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return nil, httpresp.NewAppError(404, "user_not_found", "user not found", nil)
	}

	seenProjects := make(map[uint64]struct{}, len(bindings))
	for _, binding := range bindings {
		if _, exists := seenProjects[binding.ProjectID]; exists {
			return nil, httpresp.NewAppError(400, "duplicate_project_binding", "project bindings must be unique per project", nil)
		}
		seenProjects[binding.ProjectID] = struct{}{}

		if !model.IsKnownProjectRole(binding.ProjectRole) {
			return nil, httpresp.NewAppError(400, "invalid_project_role", "project role is invalid", nil)
		}
		if _, err := s.projects.GetByID(ctx, binding.ProjectID); err != nil {
			return nil, httpresp.NewAppError(400, "project_not_found", "project for binding was not found", nil)
		}
	}

	var result []model.UserProjectRole
	if err := s.tx.WithinTransaction(ctx, func(repos repository.Repositories) error {
		if err := repos.UserProjectRoles().DeleteByUserID(ctx, userID); err != nil {
			return fmt.Errorf("clear user project bindings: %w", err)
		}

		for _, binding := range bindings {
			role := &model.UserProjectRole{
				UserID:      userID,
				ProjectID:   binding.ProjectID,
				ProjectRole: binding.ProjectRole,
			}
			if err := repos.UserProjectRoles().Create(ctx, role); err != nil {
				return fmt.Errorf("create project binding: %w", err)
			}
			result = append(result, *role)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func validateUserStatus(status string) error {
	switch status {
	case model.UserStatusActive, model.UserStatusDisabled:
		return nil
	default:
		return httpresp.NewAppError(400, "invalid_user_status", "user status is invalid", nil)
	}
}

func validatePlatformRole(role string) error {
	if !model.IsKnownPlatformRole(role) {
		return httpresp.NewAppError(400, "invalid_platform_role", "platform role is invalid", nil)
	}
	return nil
}
