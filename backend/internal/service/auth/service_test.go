package auth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/baihua19941101/cdnManage/internal/config"
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
		Database: envOrDefault("TEST_MYSQL_DATABASE", "cdn_manage_auth_service_test"),
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

func TestServiceLoginSucceeds(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	ctx := context.Background()
	suffix := uniqueSuffix()

	passwordHash, err := HashPassword("Password123!")
	require.NoError(t, err)
	user := &model.User{
		Username:     "operator-" + suffix,
		Email:        "operator-" + suffix + "@example.com",
		PasswordHash: passwordHash,
		Status:       model.UserStatusActive,
		PlatformRole: model.PlatformRoleAdmin,
	}
	require.NoError(t, store.Users().Create(ctx, user))

	service := NewService(
		store.Users(),
		repository.NewGormTxManager(db),
		NewTokenManager(config.JWTConfig{Secret: "test-secret", Issuer: "test-issuer", LifespanSeconds: 3600}),
	)

	result, err := service.Login(ctx, user.Email, "Password123!")
	require.NoError(t, err)
	require.NotEmpty(t, result.AccessToken)
	require.Equal(t, user.ID, result.User.ID)
	require.Equal(t, user.Email, result.User.Email)
}

func TestServiceChangePasswordSucceeds(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	ctx := context.Background()
	suffix := uniqueSuffix()

	oldPasswordHash, err := HashPassword("OldPassword123!")
	require.NoError(t, err)
	user := &model.User{
		Username:     "maintainer-" + suffix,
		Email:        "maintainer-" + suffix + "@example.com",
		PasswordHash: oldPasswordHash,
		Status:       model.UserStatusActive,
		PlatformRole: model.PlatformRoleAdmin,
	}
	require.NoError(t, store.Users().Create(ctx, user))

	service := NewService(
		store.Users(),
		repository.NewGormTxManager(db),
		NewTokenManager(config.JWTConfig{Secret: "test-secret", Issuer: "test-issuer", LifespanSeconds: 3600}),
	)

	require.NoError(t, service.ChangePassword(ctx, user.ID, "OldPassword123!", "NewPassword123!"))

	updatedUser, err := store.Users().GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.NoError(t, ComparePassword(updatedUser.PasswordHash, "NewPassword123!"))
	require.Error(t, ComparePassword(updatedUser.PasswordHash, "OldPassword123!"))
}
