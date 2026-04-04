package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"

	"github.com/baihua19941101/cdnManage/internal/config"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
)

const bootstrapRequestID = "system-bootstrap"

type Service struct {
	users  repository.UserRepository
	audits repository.AuditLogRepository
	tx     repository.TxManager
	config config.SuperAdminConfig
}

func NewService(
	users repository.UserRepository,
	audits repository.AuditLogRepository,
	tx repository.TxManager,
	cfg config.SuperAdminConfig,
) *Service {
	return &Service{
		users:  users,
		audits: audits,
		tx:     tx,
		config: cfg,
	}
}

func (s *Service) Run(ctx context.Context) error {
	userCount, err := s.users.Count(ctx, repository.UserFilter{})
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	if userCount > 0 {
		return nil
	}

	passwordHash, err := hashPassword(s.config.Password)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}

	username := strings.TrimSpace(s.config.Username)
	if username == "" {
		username = buildBootstrapUsername(s.config.Email)
	}

	return s.tx.WithinTransaction(ctx, func(repos repository.Repositories) error {
		superAdmin := &model.User{
			Username:     username,
			Email:        s.config.Email,
			PasswordHash: passwordHash,
			Status:       model.UserStatusActive,
			PlatformRole: model.PlatformRoleSuperAdmin,
		}
		if err := repos.Users().Create(ctx, superAdmin); err != nil {
			return fmt.Errorf("create bootstrap super admin: %w", err)
		}

		metadata, err := json.Marshal(map[string]string{
			"source": "startup",
		})
		if err != nil {
			return fmt.Errorf("marshal bootstrap audit metadata: %w", err)
		}

		if err := repos.AuditLogs().Create(ctx, &model.AuditLog{
			ActorUserID:      superAdmin.ID,
			ProjectID:        nil,
			Action:           "system.bootstrap_super_admin",
			TargetType:       "user",
			TargetIdentifier: superAdmin.Email,
			Result:           model.AuditResultSuccess,
			RequestID:        bootstrapRequestID,
			Metadata:         datatypes.JSON(metadata),
		}); err != nil {
			return fmt.Errorf("create bootstrap audit log: %w", err)
		}

		return nil
	})
}

func hashPassword(raw string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

func buildBootstrapUsername(email string) string {
	trimmed := strings.TrimSpace(email)
	if at := strings.Index(trimmed, "@"); at > 0 {
		return trimmed[:at]
	}

	return "super-admin"
}
