package projects

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	infraDB "github.com/baihua19941101/cdnManage/internal/infra/db"
	"github.com/baihua19941101/cdnManage/internal/infra/secure"
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
		Database: envOrDefault("TEST_MYSQL_DATABASE", "cdn_manage_projects_service_test"),
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

func TestServiceCreateRejectsOutOfRangeBindingCounts(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	ctx := context.Background()
	suffix := uniqueSuffix()

	t.Run("rejects_more_than_two_buckets", func(t *testing.T) {
		_, err := service.Create(ctx, CreateProjectInput{
			Name:        "bucket-overflow-" + suffix,
			Description: "bucket overflow test",
			Buckets: []ProjectBucketInput{
				{ProviderType: "aliyun", BucketName: "bucket-a-" + suffix, Region: "cn-hangzhou", CredentialCiphertext: "cipher-a", IsPrimary: true},
				{ProviderType: "aliyun", BucketName: "bucket-b-" + suffix, Region: "cn-hangzhou", CredentialCiphertext: "cipher-b", IsPrimary: false},
				{ProviderType: "aliyun", BucketName: "bucket-c-" + suffix, Region: "cn-hangzhou", CredentialCiphertext: "cipher-c", IsPrimary: false},
			},
			CDNs: []ProjectCDNInput{
				{ProviderType: "aliyun", CDNEndpoint: "https://cdn-a-" + suffix + ".example.com", PurgeScope: "url", IsPrimary: true},
			},
		})
		require.Error(t, err)

		appErr := &httpresp.AppError{}
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, 400, appErr.StatusCode)
		require.Equal(t, "invalid_bucket_count", appErr.Code)
	})

	t.Run("rejects_more_than_two_cdns", func(t *testing.T) {
		_, err := service.Create(ctx, CreateProjectInput{
			Name:        "cdn-overflow-" + suffix,
			Description: "cdn overflow test",
			Buckets: []ProjectBucketInput{
				{ProviderType: "aliyun", BucketName: "bucket-main-" + suffix, Region: "cn-hangzhou", CredentialCiphertext: "cipher-main", IsPrimary: true},
			},
			CDNs: []ProjectCDNInput{
				{ProviderType: "aliyun", CDNEndpoint: "https://cdn-1-" + suffix + ".example.com", PurgeScope: "url", IsPrimary: true},
				{ProviderType: "aliyun", CDNEndpoint: "https://cdn-2-" + suffix + ".example.com", PurgeScope: "url", IsPrimary: false},
				{ProviderType: "aliyun", CDNEndpoint: "https://cdn-3-" + suffix + ".example.com", PurgeScope: "url", IsPrimary: false},
			},
		})
		require.Error(t, err)

		appErr := &httpresp.AppError{}
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, 400, appErr.StatusCode)
		require.Equal(t, "invalid_cdn_count", appErr.Code)
	})
}

func TestServiceCreateEncryptsCredentialAndGetByIDReturnsMaskedCredential(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	ctx := context.Background()
	suffix := uniqueSuffix()
	plaintext := "ak-secret-" + suffix

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "credential-mask-" + suffix,
		Description: "credential security test",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: "aliyun",
				BucketName:   "bucket-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   plaintext,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: "aliyun",
				CDNEndpoint:  "https://cdn-" + suffix + ".example.com",
				PurgeScope:   "url",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, project.Buckets, 1)
	require.NotEmpty(t, project.Buckets[0].CredentialCiphertext)
	require.NotEqual(t, plaintext, project.Buckets[0].CredentialCiphertext)
	require.Contains(t, project.Buckets[0].CredentialCiphertext, "****")

	stored, err := store.Projects().GetByID(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, stored.Buckets, 1)
	require.NotEqual(t, plaintext, stored.Buckets[0].CredentialCiphertext)
	require.True(t, strings.HasPrefix(stored.Buckets[0].CredentialCiphertext, "v1:"))
}
