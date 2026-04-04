package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/baihua19941101/cdnManage/internal/model"
)

type gormProjectCDNRepository struct {
	db *gorm.DB
}

func (r *gormProjectCDNRepository) Create(ctx context.Context, cdn *model.ProjectCDN) error {
	return r.db.WithContext(ctx).Create(cdn).Error
}

func (r *gormProjectCDNRepository) Update(ctx context.Context, cdn *model.ProjectCDN) error {
	return r.db.WithContext(ctx).Save(cdn).Error
}

func (r *gormProjectCDNRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.ProjectCDN{}, id).Error
}

func (r *gormProjectCDNRepository) ListByProjectID(ctx context.Context, projectID uint64) ([]model.ProjectCDN, error) {
	var cdns []model.ProjectCDN
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("id asc").
		Find(&cdns).Error; err != nil {
		return nil, err
	}
	return cdns, nil
}
