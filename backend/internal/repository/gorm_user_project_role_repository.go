package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/baihua19941101/cdnManage/internal/model"
)

type gormUserProjectRoleRepository struct {
	db *gorm.DB
}

func (r *gormUserProjectRoleRepository) Create(ctx context.Context, binding *model.UserProjectRole) error {
	return r.db.WithContext(ctx).Create(binding).Error
}

func (r *gormUserProjectRoleRepository) DeleteByUserID(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.UserProjectRole{}).Error
}

func (r *gormUserProjectRoleRepository) DeleteByProjectID(ctx context.Context, projectID uint64) error {
	return r.db.WithContext(ctx).Where("project_id = ?", projectID).Delete(&model.UserProjectRole{}).Error
}

func (r *gormUserProjectRoleRepository) ListByUserID(ctx context.Context, userID uint64) ([]model.UserProjectRole, error) {
	var roles []model.UserProjectRole
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Preload("Project").
		Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *gormUserProjectRoleRepository) ListByProjectID(ctx context.Context, projectID uint64) ([]model.UserProjectRole, error) {
	var roles []model.UserProjectRole
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Preload("User").
		Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}
