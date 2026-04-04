package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	infraDB "github.com/baihua19941101/cdnManage/internal/infra/db"
	"github.com/baihua19941101/cdnManage/internal/model"
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
		Database: envOrDefault("TEST_MYSQL_DATABASE", "cdn_manage_repository_test"),
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

func TestUserRepositoryEnforcesUniqueEmail(t *testing.T) {
	db := newTestDB(t)
	repo := NewGormStore(db).Users()
	ctx := context.Background()
	suffix := uniqueSuffix()

	first := &model.User{
		Username:     "admin-" + suffix,
		Email:        "admin-" + suffix + "@example.com",
		PasswordHash: "hash-1",
		Status:       model.UserStatusActive,
		PlatformRole: model.PlatformRoleAdmin,
	}
	second := &model.User{
		Username:     "admin-2-" + suffix,
		Email:        first.Email,
		PasswordHash: "hash-2",
		Status:       model.UserStatusActive,
		PlatformRole: model.PlatformRoleAdmin,
	}

	require.NoError(t, repo.Create(ctx, first))
	require.Error(t, repo.Create(ctx, second))
}

func TestUserProjectRoleRepositoryListsBindingsByUser(t *testing.T) {
	db := newTestDB(t)
	store := NewGormStore(db)
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
		Name:        "cdn-demo-" + suffix,
		Description: "demo project",
	}

	require.NoError(t, store.Users().Create(ctx, user))
	require.NoError(t, store.Projects().Create(ctx, project))
	require.NoError(t, store.UserProjectRoles().Create(ctx, &model.UserProjectRole{
		UserID:      user.ID,
		ProjectID:   project.ID,
		ProjectRole: model.ProjectRoleAdmin,
	}))

	bindings, err := store.UserProjectRoles().ListByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	require.Equal(t, project.ID, bindings[0].ProjectID)
	require.Equal(t, model.ProjectRoleAdmin, bindings[0].ProjectRole)
	require.Equal(t, project.Name, bindings[0].Project.Name)
}

func TestProjectRepositoryPreloadsBucketsAndCDNs(t *testing.T) {
	db := newTestDB(t)
	store := NewGormStore(db)
	ctx := context.Background()
	suffix := uniqueSuffix()

	project := &model.Project{
		Name:        "cdn-assets-" + suffix,
		Description: "static assets",
	}
	require.NoError(t, store.Projects().Create(ctx, project))

	expectedBucketName := "assets-prod-" + suffix
	expectedCDNEndpoint := "https://cdn-" + suffix + ".example.com"

	require.NoError(t, store.ProjectBuckets().Create(ctx, &model.ProjectBucket{
		ProjectID:            project.ID,
		ProviderType:         "aliyun",
		BucketName:           expectedBucketName,
		Region:               "cn-hangzhou",
		CredentialCiphertext: "cipher-bucket",
		IsPrimary:            true,
	}))
	require.NoError(t, store.ProjectCDNs().Create(ctx, &model.ProjectCDN{
		ProjectID:    project.ID,
		ProviderType: "aliyun",
		CDNEndpoint:  expectedCDNEndpoint,
		PurgeScope:   "url",
		IsPrimary:    true,
	}))

	loaded, err := store.Projects().GetByID(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, loaded.Buckets, 1)
	require.Len(t, loaded.CDNs, 1)
	require.Equal(t, expectedBucketName, loaded.Buckets[0].BucketName)
	require.Equal(t, expectedCDNEndpoint, loaded.CDNs[0].CDNEndpoint)
}
