package users

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	infraDB "github.com/baihua19941101/cdnManage/internal/infra/db"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/repository"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	config := mysqlTestConfig()

	adminDB, err := gorm.Open(mysql.Open(config.adminDSN()), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, adminDB.Exec("CREATE DATABASE IF NOT EXISTS "+config.Database+" CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error)

	db, err := gorm.Open(mysql.Open(config.databaseDSN()), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.Migrator().DropTable(model.AllModels()...))
	require.NoError(t, infraDB.AutoMigrate(db))

	t.Cleanup(func() {
		for _, table := range []string{
			"audit_logs",
			"project_cdns",
			"project_buckets",
			"user_project_roles",
			"projects",
			"users",
		} {
			require.NoError(t, db.Exec("DELETE FROM "+table).Error)
		}
	})

	return db
}

type testMySQLConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

func mysqlTestConfig() testMySQLConfig {
	return testMySQLConfig{
		Host:     envOrDefault("TEST_MYSQL_HOST", "127.0.0.1"),
		Port:     envOrDefault("TEST_MYSQL_PORT", "3306"),
		User:     envOrDefault("TEST_MYSQL_USER", "root"),
		Password: envOrDefault("TEST_MYSQL_PASSWORD", "123456"),
		Database: envOrDefault("TEST_MYSQL_DATABASE", "cdn_manage_users_service_test"),
	}
}

func (c testMySQLConfig) adminDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/mysql?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true", c.User, c.Password, c.Host, c.Port)
}

func (c testMySQLConfig) databaseDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true", c.User, c.Password, c.Host, c.Port, c.Database)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func uniqueSuffix() string {
	return time.Now().Format("20060102150405.000000000")
}

func TestServiceReplaceProjectBindingsRejectsDuplicateProjectIDs(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Users(), store.Projects(), repository.NewGormTxManager(db))
	ctx := context.Background()
	suffix := uniqueSuffix()

	user := &model.User{
		Username:     "operator-" + suffix,
		Email:        "operator-" + suffix + "@example.com",
		PasswordHash: "hash",
		Status:       model.UserStatusActive,
		PlatformRole: model.PlatformRoleStandard,
	}
	project := &model.Project{
		Name:        "project-" + suffix,
		Description: "duplicate binding test",
	}

	require.NoError(t, store.Users().Create(ctx, user))
	require.NoError(t, store.Projects().Create(ctx, project))

	_, err := service.ReplaceProjectBindings(ctx, user.ID, []ProjectBindingInput{
		{ProjectID: project.ID, ProjectRole: model.ProjectRoleAdmin},
		{ProjectID: project.ID, ProjectRole: model.ProjectRoleReadOnly},
	})
	require.Error(t, err)

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.StatusCode)
	require.Equal(t, "duplicate_project_binding", appErr.Code)
}
