package bootstrap

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
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
		Database: envOrDefault("TEST_MYSQL_DATABASE", "cdn_manage_bootstrap_service_test"),
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

func TestServiceRunCreatesSuperAdminAndAuditLog(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(
		store.Users(),
		store.AuditLogs(),
		repository.NewGormTxManager(db),
		config.SuperAdminConfig{
			Email:    "bootstrap-" + uniqueSuffix() + "@example.com",
			Password: "Password123!",
		},
	)

	err := service.Run(context.Background())
	require.NoError(t, err)

	users, err := store.Users().List(context.Background(), repository.UserFilter{})
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, model.PlatformRoleSuperAdmin, users[0].PlatformRole)
	require.Equal(t, model.UserStatusActive, users[0].Status)
	require.NotEmpty(t, users[0].PasswordHash)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(users[0].PasswordHash), []byte("Password123!")))

	logs, err := store.AuditLogs().List(context.Background(), repository.AuditLogFilter{})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, users[0].ID, logs[0].ActorUserID)
	require.Equal(t, "system.bootstrap_super_admin", logs[0].Action)
	require.Equal(t, model.AuditResultSuccess, logs[0].Result)
	require.Equal(t, bootstrapRequestID, logs[0].RequestID)
}

func TestServiceRunSkipsWhenUsersAlreadyExist(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	ctx := context.Background()
	suffix := uniqueSuffix()

	passwordHash, err := hashPassword("ExistingPassword123!")
	require.NoError(t, err)
	require.NoError(t, store.Users().Create(ctx, &model.User{
		Username:     "existing-" + suffix,
		Email:        "existing-" + suffix + "@example.com",
		PasswordHash: passwordHash,
		Status:       model.UserStatusActive,
		PlatformRole: model.PlatformRoleAdmin,
	}))

	service := NewService(
		store.Users(),
		store.AuditLogs(),
		repository.NewGormTxManager(db),
		config.SuperAdminConfig{
			Email:    "bootstrap-" + suffix + "@example.com",
			Password: "Password123!",
		},
	)

	require.NoError(t, service.Run(ctx))

	users, err := store.Users().List(ctx, repository.UserFilter{})
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, "existing-"+suffix+"@example.com", users[0].Email)

	logs, err := store.AuditLogs().List(ctx, repository.AuditLogFilter{})
	require.NoError(t, err)
	require.Len(t, logs, 0)
}
