package repository

import (
	"context"

	"github.com/baihua19941101/cdnManage/internal/model"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	Update(ctx context.Context, user *model.User) error
	Delete(ctx context.Context, id uint64) error
	GetByID(ctx context.Context, id uint64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context, filter UserFilter) ([]model.User, error)
}

type ProjectRepository interface {
	Create(ctx context.Context, project *model.Project) error
	Update(ctx context.Context, project *model.Project) error
	Delete(ctx context.Context, id uint64) error
	GetByID(ctx context.Context, id uint64) (*model.Project, error)
	GetByName(ctx context.Context, name string) (*model.Project, error)
	List(ctx context.Context, filter ProjectFilter) ([]model.Project, error)
}

type UserProjectRoleRepository interface {
	Create(ctx context.Context, binding *model.UserProjectRole) error
	DeleteByUserID(ctx context.Context, userID uint64) error
	DeleteByProjectID(ctx context.Context, projectID uint64) error
	ListByUserID(ctx context.Context, userID uint64) ([]model.UserProjectRole, error)
	ListByProjectID(ctx context.Context, projectID uint64) ([]model.UserProjectRole, error)
}

type ProjectBucketRepository interface {
	Create(ctx context.Context, bucket *model.ProjectBucket) error
	Update(ctx context.Context, bucket *model.ProjectBucket) error
	Delete(ctx context.Context, id uint64) error
	ListByProjectID(ctx context.Context, projectID uint64) ([]model.ProjectBucket, error)
}

type ProjectCDNRepository interface {
	Create(ctx context.Context, cdn *model.ProjectCDN) error
	Update(ctx context.Context, cdn *model.ProjectCDN) error
	Delete(ctx context.Context, id uint64) error
	ListByProjectID(ctx context.Context, projectID uint64) ([]model.ProjectCDN, error)
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
	List(ctx context.Context, filter AuditLogFilter) ([]model.AuditLog, error)
}

type Repositories interface {
	Users() UserRepository
	Projects() ProjectRepository
	UserProjectRoles() UserProjectRoleRepository
	ProjectBuckets() ProjectBucketRepository
	ProjectCDNs() ProjectCDNRepository
	AuditLogs() AuditLogRepository
}
