package repository

import (
	"context"

	"gorm.io/gorm"
)

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(repos Repositories) error) error
}

type GormTxManager struct {
	db *gorm.DB
}

func NewGormTxManager(db *gorm.DB) *GormTxManager {
	return &GormTxManager{db: db}
}

func (m *GormTxManager) WithinTransaction(ctx context.Context, fn func(repos Repositories) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewGormStore(tx))
	})
}
