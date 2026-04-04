package repository

import "gorm.io/gorm"

type GormStore struct {
	db                *gorm.DB
	userRepo          UserRepository
	projectRepo       ProjectRepository
	userProjectRepo   UserProjectRoleRepository
	projectBucketRepo ProjectBucketRepository
	projectCDNRepo    ProjectCDNRepository
	auditLogRepo      AuditLogRepository
}

func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{
		db:                db,
		userRepo:          &gormUserRepository{db: db},
		projectRepo:       &gormProjectRepository{db: db},
		userProjectRepo:   &gormUserProjectRoleRepository{db: db},
		projectBucketRepo: &gormProjectBucketRepository{db: db},
		projectCDNRepo:    &gormProjectCDNRepository{db: db},
		auditLogRepo:      &gormAuditLogRepository{db: db},
	}
}

func (s *GormStore) Users() UserRepository {
	return s.userRepo
}

func (s *GormStore) Projects() ProjectRepository {
	return s.projectRepo
}

func (s *GormStore) UserProjectRoles() UserProjectRoleRepository {
	return s.userProjectRepo
}

func (s *GormStore) ProjectBuckets() ProjectBucketRepository {
	return s.projectBucketRepo
}

func (s *GormStore) ProjectCDNs() ProjectCDNRepository {
	return s.projectCDNRepo
}

func (s *GormStore) AuditLogs() AuditLogRepository {
	return s.auditLogRepo
}
