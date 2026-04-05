package auth

import (
	"context"
	"fmt"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
)

type Service struct {
	users  repository.UserRepository
	tx     repository.TxManager
	tokens *TokenManager
}

type LoginResult struct {
	AccessToken string
	User        *model.User
}

func NewService(users repository.UserRepository, tx repository.TxManager, tokens *TokenManager) *Service {
	return &Service{
		users:  users,
		tx:     tx,
		tokens: tokens,
	}
}

func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, httpresp.NewAppError(401, "authentication_failed", "invalid email or password", nil)
	}

	if user.Status != model.UserStatusActive {
		return nil, httpresp.NewAppError(403, "user_disabled", "user account is disabled", nil)
	}

	if err := ComparePassword(user.PasswordHash, password); err != nil {
		return nil, httpresp.NewAppError(401, "authentication_failed", "invalid email or password", nil)
	}

	token, err := s.tokens.Generate(user.ID, user.PlatformRole)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	return &LoginResult{
		AccessToken: token,
		User:        user,
	}, nil
}

func (s *Service) Me(ctx context.Context, token string) (*model.User, error) {
	claims, err := s.tokens.Parse(token)
	if err != nil {
		return nil, httpresp.NewAppError(401, "authentication_failed", "invalid token", nil)
	}

	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, httpresp.NewAppError(401, "authentication_failed", "user not found", nil)
	}

	if user.Status != model.UserStatusActive {
		return nil, httpresp.NewAppError(403, "user_disabled", "user account is disabled", nil)
	}

	return user, nil
}

func (s *Service) ChangePassword(ctx context.Context, token, currentPassword, newPassword string) error {
	claims, err := s.tokens.Parse(token)
	if err != nil {
		return httpresp.NewAppError(401, "authentication_failed", "invalid token", nil)
	}

	newPasswordHash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}

	return s.tx.WithinTransaction(ctx, func(repos repository.Repositories) error {
		user, err := repos.Users().GetByID(ctx, claims.UserID)
		if err != nil {
			return httpresp.NewAppError(401, "authentication_failed", "user not found", nil)
		}

		if user.Status != model.UserStatusActive {
			return httpresp.NewAppError(403, "user_disabled", "user account is disabled", nil)
		}

		if err := ComparePassword(user.PasswordHash, currentPassword); err != nil {
			return httpresp.NewAppError(400, "invalid_current_password", "current password is incorrect", nil)
		}

		user.PasswordHash = newPasswordHash
		if err := repos.Users().Update(ctx, user); err != nil {
			return fmt.Errorf("update user password: %w", err)
		}

		return nil
	})
}
