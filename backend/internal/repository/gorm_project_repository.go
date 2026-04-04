package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/baihua19941101/cdnManage/internal/model"
)

type gormProjectRepository struct {
	db *gorm.DB
}

func (r *gormProjectRepository) Create(ctx context.Context, project *model.Project) error {
	return r.db.WithContext(ctx).Create(project).Error
}

func (r *gormProjectRepository) Update(ctx context.Context, project *model.Project) error {
	return r.db.WithContext(ctx).Save(project).Error
}

func (r *gormProjectRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Project{}, id).Error
}

func (r *gormProjectRepository) GetByID(ctx context.Context, id uint64) (*model.Project, error) {
	var project model.Project
	if err := r.db.WithContext(ctx).
		Preload("ProjectRoles").
		Preload("Buckets").
		Preload("CDNs").
		First(&project, id).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *gormProjectRepository) GetByName(ctx context.Context, name string) (*model.Project, error) {
	var project model.Project
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *gormProjectRepository) List(ctx context.Context, filter ProjectFilter) ([]model.Project, error) {
	query := r.db.WithContext(ctx).Model(&model.Project{})
	if filter.Name != "" {
		query = query.Where("name LIKE ?", "%"+filter.Name+"%")
	}

	var projects []model.Project
	if err := query.Order("id desc").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}
