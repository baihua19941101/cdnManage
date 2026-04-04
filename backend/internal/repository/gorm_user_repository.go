package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/baihua19941101/cdnManage/internal/model"
)

type gormUserRepository struct {
	db *gorm.DB
}

func (r *gormUserRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *gormUserRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *gormUserRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, id).Error
}

func (r *gormUserRepository) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormUserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormUserRepository) List(ctx context.Context, filter UserFilter) ([]model.User, error) {
	query := r.db.WithContext(ctx).Model(&model.User{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.PlatformRole != "" {
		query = query.Where("platform_role = ?", filter.PlatformRole)
	}

	var users []model.User
	if err := query.Order("id desc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
