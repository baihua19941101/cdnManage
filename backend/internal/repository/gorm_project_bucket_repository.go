package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/baihua19941101/cdnManage/internal/model"
)

type gormProjectBucketRepository struct {
	db *gorm.DB
}

func (r *gormProjectBucketRepository) Create(ctx context.Context, bucket *model.ProjectBucket) error {
	return r.db.WithContext(ctx).Create(bucket).Error
}

func (r *gormProjectBucketRepository) Update(ctx context.Context, bucket *model.ProjectBucket) error {
	return r.db.WithContext(ctx).Save(bucket).Error
}

func (r *gormProjectBucketRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.ProjectBucket{}, id).Error
}

func (r *gormProjectBucketRepository) ListByProjectID(ctx context.Context, projectID uint64) ([]model.ProjectBucket, error) {
	var buckets []model.ProjectBucket
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("id asc").
		Find(&buckets).Error; err != nil {
		return nil, err
	}
	return buckets, nil
}
