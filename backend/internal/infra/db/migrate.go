package db

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/baihua19941101/cdnManage/internal/model"
)

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	if err := db.AutoMigrate(model.AllModels()...); err != nil {
		return fmt.Errorf("auto migrate models: %w", err)
	}

	return nil
}
