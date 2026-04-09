package projects

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
	infraDB "github.com/baihua19941101/cdnManage/internal/infra/db"
	"github.com/baihua19941101/cdnManage/internal/infra/secure"
	"github.com/baihua19941101/cdnManage/internal/model"
	"github.com/baihua19941101/cdnManage/internal/provider"
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

func registerTestProviders(t *testing.T, service *Service) {
	t.Helper()

	types := []provider.Type{
		provider.TypeAliyun,
		provider.TypeTencentCloud,
		provider.TypeHuaweiCloud,
		provider.TypeQiniu,
	}

	for _, providerType := range types {
		require.NoError(t, service.RegisterObjectStorageProvider(&fakeObjectStorageProvider{providerType: providerType}))
		require.NoError(t, service.RegisterCDNProvider(&fakeCDNProvider{providerType: providerType}))
	}
}

func TestServiceCreateRejectsOutOfRangeBindingCounts(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	registerTestProviders(t, service)
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
				{ProviderType: "aliyun", CDNEndpoint: "https://cdn-a-" + suffix + ".example.com", Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`, PurgeScope: "url", IsPrimary: true},
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
				{ProviderType: "aliyun", CDNEndpoint: "https://cdn-1-" + suffix + ".example.com", Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`, PurgeScope: "url", IsPrimary: true},
				{ProviderType: "aliyun", CDNEndpoint: "https://cdn-2-" + suffix + ".example.com", Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`, PurgeScope: "url", IsPrimary: false},
				{ProviderType: "aliyun", CDNEndpoint: "https://cdn-3-" + suffix + ".example.com", Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`, PurgeScope: "url", IsPrimary: false},
			},
		})
		require.Error(t, err)

		appErr := &httpresp.AppError{}
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, 400, appErr.StatusCode)
		require.Equal(t, "invalid_cdn_count", appErr.Code)
	})
}

func TestServiceCreateAllowsMixedProviderBindings(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "mixed-provider-create-" + suffix,
		Description: "allow mixed provider bindings in one project",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-aliyun-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
			{
				ProviderType: model.ProviderTypeTencent,
				BucketName:   "bucket-tencent-" + suffix,
				Region:       "ap-guangzhou",
				Credential:   `{"accessKeyId":"AKID_TEST_B","accessKeySecret":"secret"}`,
				IsPrimary:    false,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-aliyun-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    true,
			},
			{
				ProviderType: model.ProviderTypeTencent,
				CDNEndpoint:  "https://cdn-tencent-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"AKID_TEST_B","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    false,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, project.Buckets, 2)
	require.Len(t, project.CDNs, 2)

	bucketProviders := map[string]struct{}{}
	for _, bucket := range project.Buckets {
		bucketProviders[bucket.ProviderType] = struct{}{}
	}
	require.Contains(t, bucketProviders, model.ProviderTypeAliyun)
	require.Contains(t, bucketProviders, model.ProviderTypeTencent)

	cdnProviders := map[string]struct{}{}
	for _, cdn := range project.CDNs {
		cdnProviders[cdn.ProviderType] = struct{}{}
	}
	require.Contains(t, cdnProviders, model.ProviderTypeAliyun)
	require.Contains(t, cdnProviders, model.ProviderTypeTencent)
}

func TestServiceUpdateCDNsAllowsMixedProviderBindings(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "mixed-provider-update-cdn-" + suffix,
		Description: "update cdn with mixed providers",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-main-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-main-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)

	updatedCDNs, err := service.UpdateCDNs(ctx, project.ID, []ProjectCDNInput{
		{
			ProviderType: model.ProviderTypeAliyun,
			CDNEndpoint:  "https://cdn-aliyun-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		},
		{
			ProviderType: model.ProviderTypeTencent,
			CDNEndpoint:  "https://cdn-tencent-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"AKID_TEST_B","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    false,
		},
	})
	require.NoError(t, err)
	require.Len(t, updatedCDNs, 2)

	cdnProviders := map[string]struct{}{}
	for _, cdn := range updatedCDNs {
		cdnProviders[cdn.ProviderType] = struct{}{}
	}
	require.Contains(t, cdnProviders, model.ProviderTypeAliyun)
	require.Contains(t, cdnProviders, model.ProviderTypeTencent)
}

func TestServiceCreateRejectsUnregisteredBucketProvider(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	ctx := context.Background()
	suffix := uniqueSuffix()

	_, err := service.Create(ctx, CreateProjectInput{
		Name:        "unregistered-bucket-provider-" + suffix,
		Description: "reject unregistered object storage provider",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{},
	})
	require.Error(t, err)

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.StatusCode)
	require.Equal(t, "provider_not_registered", appErr.Code)

	details, ok := appErr.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "buckets", details["bindingType"])
	require.Equal(t, "buckets[0].providerType", details["bindingPath"])
	require.Equal(t, model.ProviderTypeAliyun, details["providerType"])
	require.Equal(t, "object_storage", details["providerService"])
}

func TestServiceCreateRejectsUnregisteredCDNProvider(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	ctx := context.Background()
	suffix := uniqueSuffix()

	require.NoError(t, service.RegisterObjectStorageProvider(&fakeObjectStorageProvider{
		providerType: provider.TypeAliyun,
	}))

	_, err := service.Create(ctx, CreateProjectInput{
		Name:        "unregistered-cdn-provider-" + suffix,
		Description: "reject unregistered cdn provider",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    true,
			},
		},
	})
	require.Error(t, err)

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.StatusCode)
	require.Equal(t, "provider_not_registered", appErr.Code)

	details, ok := appErr.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "cdns", details["bindingType"])
	require.Equal(t, "cdns[0].providerType", details["bindingPath"])
	require.Equal(t, model.ProviderTypeAliyun, details["providerType"])
	require.Equal(t, "cdn", details["providerService"])
}

func TestServiceUpdateRejectsUnregisteredBucketProvider(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	ctx := context.Background()
	suffix := uniqueSuffix()

	require.NoError(t, service.RegisterObjectStorageProvider(&fakeObjectStorageProvider{
		providerType: provider.TypeAliyun,
	}))
	require.NoError(t, service.RegisterCDNProvider(&fakeCDNProvider{
		providerType: provider.TypeAliyun,
	}))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "registered-provider-update-base-" + suffix,
		Description: "base project for update provider registration validation",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-base-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-base-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)

	_, err = service.Update(ctx, project.ID, UpdateProjectInput{
		Name:        "registered-provider-update-base-" + suffix,
		Description: "update should reject unregistered provider",
		Buckets: []ProjectBucketInput{
			{
				ProviderType:        model.ProviderTypeTencent,
				BucketName:          "bucket-unregistered-" + suffix,
				Region:              "ap-guangzhou",
				CredentialOperation: "REPLACE",
				Credential:          `{"accessKeyId":"AKID_TEST_B","accessKeySecret":"secret"}`,
				IsPrimary:           true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType:        model.ProviderTypeAliyun,
				CDNEndpoint:         "https://cdn-base-" + suffix + ".example.com",
				CredentialOperation: "REPLACE",
				Credential:          `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:          "url",
				IsPrimary:           true,
			},
		},
	})
	require.Error(t, err)

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.StatusCode)
	require.Equal(t, "provider_not_registered", appErr.Code)

	details, ok := appErr.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "buckets", details["bindingType"])
	require.Equal(t, "buckets[0].providerType", details["bindingPath"])
	require.Equal(t, model.ProviderTypeTencent, details["providerType"])
	require.Equal(t, "object_storage", details["providerService"])
}

func TestServiceUpdateKeepsCredentialForExistingBindings(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "credential-keep-update-" + suffix,
		Description: "update existing bindings without replacing credentials",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-keep-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-keep-" + suffix + ".example.com",
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:   "directory",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, project.Buckets, 1)
	require.Len(t, project.CDNs, 1)

	storedBefore, err := store.Projects().GetByID(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, storedBefore.Buckets, 1)
	require.Len(t, storedBefore.CDNs, 1)

	originalBucketCipher := storedBefore.Buckets[0].CredentialCiphertext
	originalCDNCipher := storedBefore.CDNs[0].CredentialCiphertext

	_, err = service.Update(ctx, project.ID, UpdateProjectInput{
		Name:        "credential-keep-update-" + suffix + "-v2",
		Description: "updated with keep operation",
		Buckets: []ProjectBucketInput{
			{
				ID:                  storedBefore.Buckets[0].ID,
				ProviderType:        model.ProviderTypeAliyun,
				BucketName:          storedBefore.Buckets[0].BucketName,
				Region:              "cn-shanghai",
				CredentialOperation: "KEEP",
				IsPrimary:           true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ID:                  storedBefore.CDNs[0].ID,
				ProviderType:        model.ProviderTypeAliyun,
				CDNEndpoint:         storedBefore.CDNs[0].CDNEndpoint,
				Region:              "cn-shanghai",
				CredentialOperation: "KEEP",
				IsPrimary:           true,
			},
		},
	})
	require.NoError(t, err)

	storedAfter, err := store.Projects().GetByID(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, storedAfter.Buckets, 1)
	require.Len(t, storedAfter.CDNs, 1)
	require.Equal(t, "cn-shanghai", storedAfter.Buckets[0].Region)
	require.Equal(t, "cn-shanghai", storedAfter.CDNs[0].Region)
	require.Equal(t, "directory", storedAfter.CDNs[0].PurgeScope)
	require.Equal(t, originalBucketCipher, storedAfter.Buckets[0].CredentialCiphertext)
	require.Equal(t, originalCDNCipher, storedAfter.CDNs[0].CredentialCiphertext)
}

func TestServiceUpdateReplacesCredentialForExistingBindings(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	cipher := secure.NewCredentialCipher("projects-service-test-key")
	service := NewService(store.Projects(), repository.NewGormTxManager(db), cipher)
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "credential-replace-update-" + suffix,
		Description: "update existing bindings with replace operation",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-replace-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret-a"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-replace-" + suffix + ".example.com",
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret-a"}`,
				PurgeScope:   "directory",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)

	storedBefore, err := store.Projects().GetByID(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, storedBefore.Buckets, 1)
	require.Len(t, storedBefore.CDNs, 1)

	originalBucketCipher := storedBefore.Buckets[0].CredentialCiphertext
	originalCDNCipher := storedBefore.CDNs[0].CredentialCiphertext
	replacedBucketCredential := `{"accessKeyId":"LTAI_TEST_B","accessKeySecret":"secret-b"}`
	replacedCDNCredential := `{"accessKeyId":"LTAI_TEST_B","accessKeySecret":"secret-b"}`

	_, err = service.Update(ctx, project.ID, UpdateProjectInput{
		Name:        "credential-replace-update-" + suffix + "-v2",
		Description: "updated with replace operation",
		Buckets: []ProjectBucketInput{
			{
				ID:                  storedBefore.Buckets[0].ID,
				ProviderType:        model.ProviderTypeAliyun,
				BucketName:          storedBefore.Buckets[0].BucketName,
				Region:              "cn-shanghai",
				CredentialOperation: "REPLACE",
				Credential:          replacedBucketCredential,
				IsPrimary:           true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ID:                  storedBefore.CDNs[0].ID,
				ProviderType:        model.ProviderTypeAliyun,
				CDNEndpoint:         storedBefore.CDNs[0].CDNEndpoint,
				Region:              "cn-shanghai",
				CredentialOperation: "REPLACE",
				Credential:          replacedCDNCredential,
				IsPrimary:           true,
			},
		},
	})
	require.NoError(t, err)

	storedAfter, err := store.Projects().GetByID(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, storedAfter.Buckets, 1)
	require.Len(t, storedAfter.CDNs, 1)
	require.Equal(t, "cn-shanghai", storedAfter.Buckets[0].Region)
	require.Equal(t, "cn-shanghai", storedAfter.CDNs[0].Region)
	require.Equal(t, "directory", storedAfter.CDNs[0].PurgeScope)
	require.NotEqual(t, originalBucketCipher, storedAfter.Buckets[0].CredentialCiphertext)
	require.NotEqual(t, originalCDNCipher, storedAfter.CDNs[0].CredentialCiphertext)

	decryptedBucketCredential, err := cipher.Decrypt(storedAfter.Buckets[0].CredentialCiphertext)
	require.NoError(t, err)
	require.Equal(t, replacedBucketCredential, decryptedBucketCredential)

	decryptedCDNCredential, err := cipher.Decrypt(storedAfter.CDNs[0].CredentialCiphertext)
	require.NoError(t, err)
	require.Equal(t, replacedCDNCredential, decryptedCDNCredential)
}

func TestServiceUpdateReturnsBindingDetailsWhenCredentialNotFoundForKeep(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "credential-not-found-keep-" + suffix,
		Description: "keep operation without historical credential should fail",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-not-found-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-not-found-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, db.Model(&model.ProjectBucket{}).
		Where("id = ?", project.Buckets[0].ID).
		Update("credential_ciphertext", "").Error)

	_, err = service.Update(ctx, project.ID, UpdateProjectInput{
		Name:        project.Name,
		Description: project.Description,
		Buckets: []ProjectBucketInput{
			{
				ID:                  project.Buckets[0].ID,
				ProviderType:        project.Buckets[0].ProviderType,
				BucketName:          project.Buckets[0].BucketName,
				Region:              project.Buckets[0].Region,
				CredentialOperation: "KEEP",
				IsPrimary:           true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ID:                  project.CDNs[0].ID,
				ProviderType:        project.CDNs[0].ProviderType,
				CDNEndpoint:         project.CDNs[0].CDNEndpoint,
				Region:              project.CDNs[0].Region,
				CredentialOperation: "KEEP",
				PurgeScope:          project.CDNs[0].PurgeScope,
				IsPrimary:           true,
			},
		},
	})
	require.Error(t, err)

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.StatusCode)
	require.Equal(t, "credential_not_found_for_keep", appErr.Code)

	details, ok := appErr.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "buckets", details["bindingType"])
	require.Equal(t, 0, details["bindingIndex"])
	require.Equal(t, "buckets[0].credentialOperation", details["bindingPath"])
}

func TestServiceUpdateRejectsProviderChangeWhenCredentialOperationKeep(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "provider-change-keep-" + suffix,
		Description: "provider change with keep should fail",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-provider-change-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-provider-change-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)

	_, err = service.Update(ctx, project.ID, UpdateProjectInput{
		Name:        project.Name,
		Description: project.Description,
		Buckets: []ProjectBucketInput{
			{
				ID:                  project.Buckets[0].ID,
				ProviderType:        model.ProviderTypeTencent,
				BucketName:          project.Buckets[0].BucketName,
				Region:              project.Buckets[0].Region,
				CredentialOperation: "KEEP",
				IsPrimary:           true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ID:                  project.CDNs[0].ID,
				ProviderType:        project.CDNs[0].ProviderType,
				CDNEndpoint:         project.CDNs[0].CDNEndpoint,
				Region:              project.CDNs[0].Region,
				CredentialOperation: "KEEP",
				PurgeScope:          project.CDNs[0].PurgeScope,
				IsPrimary:           true,
			},
		},
	})
	require.Error(t, err)

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.StatusCode)
	require.Equal(t, "provider_change_requires_credential_replace", appErr.Code)

	details, ok := appErr.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "buckets", details["bindingType"])
	require.Equal(t, 0, details["bindingIndex"])
	require.Equal(t, "buckets[0].credentialOperation", details["bindingPath"])
}

func TestServiceUpdateRejectsNewBindingWhenCredentialOperationKeep(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "new-binding-keep-" + suffix,
		Description: "new binding with keep should fail",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-existing-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-existing-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)

	_, err = service.Update(ctx, project.ID, UpdateProjectInput{
		Name:        project.Name,
		Description: project.Description,
		Buckets: []ProjectBucketInput{
			{
				ID:                  project.Buckets[0].ID,
				ProviderType:        project.Buckets[0].ProviderType,
				BucketName:          project.Buckets[0].BucketName,
				Region:              project.Buckets[0].Region,
				CredentialOperation: "KEEP",
				IsPrimary:           true,
			},
			{
				ProviderType:        model.ProviderTypeTencent,
				BucketName:          "bucket-new-" + suffix,
				Region:              "ap-guangzhou",
				CredentialOperation: "KEEP",
				IsPrimary:           false,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ID:                  project.CDNs[0].ID,
				ProviderType:        project.CDNs[0].ProviderType,
				CDNEndpoint:         project.CDNs[0].CDNEndpoint,
				Region:              project.CDNs[0].Region,
				CredentialOperation: "KEEP",
				PurgeScope:          project.CDNs[0].PurgeScope,
				IsPrimary:           true,
			},
		},
	})
	require.Error(t, err)

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.StatusCode)
	require.Equal(t, "credential_missing_for_new_binding", appErr.Code)

	details, ok := appErr.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "buckets", details["bindingType"])
	require.Equal(t, 1, details["bindingIndex"])
	require.Equal(t, "buckets[1].credentialOperation", details["bindingPath"])
}

func TestServiceUpdateCDNsRejectsUnregisteredCDNProvider(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	ctx := context.Background()
	suffix := uniqueSuffix()

	require.NoError(t, service.RegisterObjectStorageProvider(&fakeObjectStorageProvider{
		providerType: provider.TypeAliyun,
	}))
	require.NoError(t, service.RegisterCDNProvider(&fakeCDNProvider{
		providerType: provider.TypeAliyun,
	}))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "registered-cdn-update-base-" + suffix,
		Description: "base project for update cdn provider registration validation",
		Buckets: []ProjectBucketInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				BucketName:   "bucket-base-" + suffix,
				Region:       "cn-hangzhou",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				IsPrimary:    true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: model.ProviderTypeAliyun,
				CDNEndpoint:  "https://cdn-base-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"LTAI_TEST_A","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)

	_, err = service.UpdateCDNs(ctx, project.ID, []ProjectCDNInput{
		{
			ProviderType: model.ProviderTypeTencent,
			CDNEndpoint:  "https://cdn-unregistered-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"AKID_TEST_B","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		},
	})
	require.Error(t, err)

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.StatusCode)
	require.Equal(t, "provider_not_registered", appErr.Code)

	details, ok := appErr.Details.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "cdns", details["bindingType"])
	require.Equal(t, "cdns[0].providerType", details["bindingPath"])
	require.Equal(t, model.ProviderTypeTencent, details["providerType"])
	require.Equal(t, "cdn", details["providerService"])
}

func TestServiceCreateEncryptsCredentialAndGetByIDReturnsMaskedCredential(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
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
				Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
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

func TestServiceValidateBucketConnectionMapsDetectionErrors(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db))
	registerTestProviders(t, service)
	ctx := context.Background()

	t.Run("unsupported_provider_hint_maps_to_provider_not_supported", func(t *testing.T) {
		credential := `{"accessKeyId":"LTAI123456","customFields":{"providerType":"not_supported"}}`
		_, err := service.ValidateBucketConnection(ctx, ProjectBucketInput{
			BucketName: "assets-oss",
			Credential: credential,
		})
		require.Error(t, err)

		appErr := &httpresp.AppError{}
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, 400, appErr.StatusCode)
		require.Equal(t, "provider_not_supported", appErr.Code)
	})

	t.Run("unknown_access_key_pattern_maps_to_provider_connection_failed", func(t *testing.T) {
		credential := `{"accessKeyId":""}`
		_, err := service.ValidateBucketConnection(ctx, ProjectBucketInput{
			BucketName: "bucket-without-access-key",
			Credential: credential,
		})
		require.Error(t, err)

		appErr := &httpresp.AppError{}
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, 400, appErr.StatusCode)
		require.Equal(t, "provider_connection_failed", appErr.Code)
	})

	t.Run("unknown_pattern_maps_to_provider_connection_failed", func(t *testing.T) {
		_, err := service.ValidateBucketConnection(ctx, ProjectBucketInput{
			BucketName: "unknown-bucket",
			Credential: "RANDOM_ACCESS_KEY_PATTERN",
		})
		require.Error(t, err)

		appErr := &httpresp.AppError{}
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, 400, appErr.StatusCode)
		require.Equal(t, "provider_connection_failed", appErr.Code)
	})
}

func TestMapProviderDetectionErrorInvalidCredentials(t *testing.T) {
	err := mapProviderDetectionError(provider.NewError(
		provider.TypeUnknown,
		provider.ServiceObjectStorage,
		"detect_provider",
		provider.ErrCodeInvalidCredentials,
		"invalid credentials",
		false,
		nil,
	))

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 400, appErr.StatusCode)
	require.Equal(t, "invalid_bucket_credential", appErr.Code)
}

func TestServiceListBucketObjectsUsesProviderBoundary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	providerStub := &fakeObjectStorageProvider{
		providerType: provider.TypeAliyun,
		objects: []provider.ObjectInfo{
			{
				Key:          "assets/app.js",
				ETag:         "etag-1",
				ContentType:  "application/javascript",
				Size:         1024,
				LastModified: time.Now().UTC(),
			},
		},
	}
	require.NoError(t, service.RegisterObjectStorageProvider(providerStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "objects-list-" + suffix,
		Description: "storage list test",
		Buckets: []ProjectBucketInput{
			{
				BucketName: "bucket-list-" + suffix,
				Region:     "cn-hangzhou",
				Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
				IsPrimary:  true,
			},
		},
		CDNs: []ProjectCDNInput{
			{
				ProviderType: "aliyun",
				CDNEndpoint:  "https://cdn-list-" + suffix + ".example.com",
				Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
				PurgeScope:   "url",
				IsPrimary:    true,
			},
		},
	})
	require.NoError(t, err)

	objects, err := service.ListBucketObjects(ctx, project.ID, ListBucketObjectsInput{
		Prefix:  "assets/",
		MaxKeys: 100,
	})
	require.NoError(t, err)
	require.Len(t, objects, 1)
	require.Equal(t, "assets/app.js", objects[0].Key)
	require.Equal(t, int64(1024), objects[0].Size)
	require.Equal(t, "bucket-list-"+suffix, providerStub.lastRequest.Bucket)
	require.Equal(t, "assets/", providerStub.lastRequest.Prefix)
	require.Equal(t, 100, providerStub.lastRequest.MaxKeys)
}

func TestServiceUploadBucketObjectUsesProviderBoundary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	providerStub := &fakeObjectStorageProvider{providerType: provider.TypeAliyun}
	require.NoError(t, service.RegisterObjectStorageProvider(providerStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "objects-upload-" + suffix,
		Description: "storage upload test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-upload-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-upload-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	err = service.UploadBucketObject(ctx, project.ID, UploadBucketObjectInput{
		Key:         "assets/logo.png",
		ContentType: "image/png",
		Size:        4,
		Content:     strings.NewReader("data"),
	})
	require.NoError(t, err)
	require.Equal(t, "bucket-upload-"+suffix, providerStub.lastUpload.Bucket)
	require.Equal(t, "assets/logo.png", providerStub.lastUpload.Key)
	require.Equal(t, "image/png", providerStub.lastUpload.ContentType)
}

func TestServiceDeleteBucketObjectUsesProviderBoundary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	providerStub := &fakeObjectStorageProvider{providerType: provider.TypeAliyun}
	require.NoError(t, service.RegisterObjectStorageProvider(providerStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "objects-delete-" + suffix,
		Description: "storage delete test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-delete-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-delete-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	err = service.DeleteBucketObject(ctx, project.ID, DeleteBucketObjectInput{Key: "assets/old.js"})
	require.NoError(t, err)
	require.Equal(t, "bucket-delete-"+suffix, providerStub.lastDelete.Bucket)
	require.Equal(t, "assets/old.js", providerStub.lastDelete.Key)
}

func TestServiceDeleteBucketObjectsUsesFileParallelismForFileKeys(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	service.ConfigureDeleteFileParallelism(3)
	ctx := context.Background()
	suffix := uniqueSuffix()

	providerStub := &directoryDeleteWorkerPoolProvider{
		deleteCalls: make(map[string]int),
	}
	require.NoError(t, service.RegisterObjectStorageProvider(providerStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "objects-batch-delete-file-parallel-" + suffix,
		Description: "storage batch delete file parallel test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-batch-delete-file-parallel-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-batch-delete-file-parallel-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	keys := []string{"assets/a.js", "assets/b.js", "assets/c.js", "assets/d.js"}
	results, err := service.DeleteBucketObjects(ctx, project.ID, DeleteBucketObjectsInput{
		Keys: keys,
	})
	require.NoError(t, err)
	require.Len(t, results, len(keys))

	for idx, key := range keys {
		require.Equal(t, key, results[idx].Key)
		require.Equal(t, "file", results[idx].TargetType)
		require.True(t, results[idx].Success)
		require.Equal(t, 1, results[idx].DeletedObjects)
		require.Equal(t, 0, results[idx].FailedObjects)
		require.Equal(t, "", results[idx].ErrorCode)
		require.Equal(t, "object deleted", results[idx].Message)
	}
	require.Equal(t, len(keys), providerStub.uniqueDeleteCallCount())
	require.GreaterOrEqual(t, providerStub.maxConcurrency(), 2)
}

func TestServiceDeleteBucketObjectsKeepsFileStatsAndErrorsStableUnderParallelism(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	service.ConfigureDeleteFileParallelism(4)
	ctx := context.Background()
	suffix := uniqueSuffix()

	failedKeys := map[string]struct{}{
		"assets/b.js": {},
		"assets/d.js": {},
	}
	providerStub := &directoryDeleteWorkerPoolProvider{
		deleteErrors: map[string]error{
			"assets/b.js": provider.NewError(
				provider.TypeAliyun,
				provider.ServiceObjectStorage,
				"delete_object",
				provider.ErrCodeOperationFailed,
				"delete failed",
				false,
				nil,
			),
			"assets/d.js": provider.NewError(
				provider.TypeAliyun,
				provider.ServiceObjectStorage,
				"delete_object",
				provider.ErrCodeOperationFailed,
				"delete failed",
				false,
				nil,
			),
		},
		deleteCalls: make(map[string]int),
	}
	require.NoError(t, service.RegisterObjectStorageProvider(providerStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "objects-batch-delete-file-errors-" + suffix,
		Description: "storage batch delete file errors test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-batch-delete-file-errors-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-batch-delete-file-errors-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	keys := []string{"assets/a.js", "assets/b.js", "assets/c.js", "assets/d.js"}
	results, err := service.DeleteBucketObjects(ctx, project.ID, DeleteBucketObjectsInput{
		Keys: keys,
	})
	require.NoError(t, err)
	require.Len(t, results, len(keys))

	successCount := 0
	failureCount := 0
	for idx, key := range keys {
		result := results[idx]
		require.Equal(t, key, result.Key)
		require.Equal(t, "file", result.TargetType)
		if _, shouldFail := failedKeys[key]; shouldFail {
			failureCount++
			require.False(t, result.Success)
			require.Equal(t, "provider_operation_failed", result.ErrorCode)
			require.Contains(t, result.Message, "storage provider operation failed")
			require.Equal(t, 0, result.DeletedObjects)
			require.Equal(t, 1, result.FailedObjects)
		} else {
			successCount++
			require.True(t, result.Success)
			require.Equal(t, "", result.ErrorCode)
			require.Equal(t, "object deleted", result.Message)
			require.Equal(t, 1, result.DeletedObjects)
			require.Equal(t, 0, result.FailedObjects)
		}
	}
	require.Equal(t, 2, successCount)
	require.Equal(t, 2, failureCount)
	require.Equal(t, len(keys), providerStub.uniqueDeleteCallCount())
	require.GreaterOrEqual(t, providerStub.maxConcurrency(), 2)
}

func TestServiceDeleteBucketObjectsUsesBatchParallelismForMixedKeysAndKeepsResultOrderStable(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	service.ConfigureDeleteParallelism(1)
	service.ConfigureDeleteFileParallelism(1)
	service.ConfigureDeleteBatchParallelism(3)
	ctx := context.Background()
	suffix := uniqueSuffix()

	providerStub := &directoryDeleteWorkerPoolProvider{
		objects: []provider.ObjectInfo{
			{Key: "assets/dir-a/a.js", IsDir: false},
			{Key: "assets/dir-a/b.js", IsDir: false},
			{Key: "assets/dir-b/c.js", IsDir: false},
		},
		deleteErrors: map[string]error{
			"assets/file-b.js": provider.NewError(
				provider.TypeAliyun,
				provider.ServiceObjectStorage,
				"delete_object",
				provider.ErrCodeOperationFailed,
				"delete failed",
				false,
				nil,
			),
			"assets/dir-b/c.js": provider.NewError(
				provider.TypeAliyun,
				provider.ServiceObjectStorage,
				"delete_object",
				provider.ErrCodeOperationFailed,
				"delete failed",
				false,
				nil,
			),
		},
		deleteCalls: make(map[string]int),
	}
	require.NoError(t, service.RegisterObjectStorageProvider(providerStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "objects-batch-delete-mixed-parallel-" + suffix,
		Description: "storage batch delete mixed parallel test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-batch-delete-mixed-parallel-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-batch-delete-mixed-parallel-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	keys := []string{"assets/file-a.js", "assets/dir-a/", "assets/file-b.js", "assets/dir-b/"}
	results, err := service.DeleteBucketObjects(ctx, project.ID, DeleteBucketObjectsInput{
		Keys: keys,
	})
	require.NoError(t, err)
	require.Len(t, results, len(keys))

	require.Equal(t, "assets/file-a.js", results[0].Key)
	require.Equal(t, "file", results[0].TargetType)
	require.True(t, results[0].Success)
	require.Equal(t, 1, results[0].DeletedObjects)
	require.Equal(t, 0, results[0].FailedObjects)
	require.Equal(t, "object deleted", results[0].Message)

	require.Equal(t, "assets/dir-a/", results[1].Key)
	require.Equal(t, "directory", results[1].TargetType)
	require.True(t, results[1].Success)
	require.Equal(t, 2, results[1].DeletedObjects)
	require.Equal(t, 0, results[1].FailedObjects)
	require.Equal(t, "directory deleted", results[1].Message)

	require.Equal(t, "assets/file-b.js", results[2].Key)
	require.Equal(t, "file", results[2].TargetType)
	require.False(t, results[2].Success)
	require.Equal(t, "provider_operation_failed", results[2].ErrorCode)
	require.Equal(t, 0, results[2].DeletedObjects)
	require.Equal(t, 1, results[2].FailedObjects)
	require.Contains(t, results[2].Message, "storage provider operation failed")

	require.Equal(t, "assets/dir-b/", results[3].Key)
	require.Equal(t, "directory", results[3].TargetType)
	require.False(t, results[3].Success)
	require.Equal(t, "provider_operation_failed", results[3].ErrorCode)
	require.Equal(t, 0, results[3].DeletedObjects)
	require.Equal(t, 1, results[3].FailedObjects)
	require.Contains(t, results[3].Message, "directory delete partially failed")
	require.Contains(t, results[3].Message, "assets/dir-b/c.js:")

	require.Equal(t, 5, providerStub.uniqueDeleteCallCount())
	require.GreaterOrEqual(t, providerStub.maxConcurrency(), 2)
}

func TestDeleteSingleFileObjectMapsTimeoutErrorCode(t *testing.T) {
	service := NewService(nil, nil)
	service.deleteTimeout = 20 * time.Millisecond

	providerStub := &directoryDeleteWorkerPoolProvider{
		deleteCalls:          make(map[string]int),
		waitDeleteForContext: true,
	}

	deleteCtx, cancel := service.withDeleteProviderTimeout(context.Background())
	defer cancel()

	result := service.deleteSingleFileObject(deleteCtx, providerStub, provider.DeleteObjectRequest{
		Bucket: "bucket",
		Region: "region",
	}, "assets/timeout.js")

	require.False(t, result.Success)
	require.Equal(t, "file", result.TargetType)
	require.Equal(t, "delete_request_timeout", result.ErrorCode)
	require.Equal(t, "delete request timed out", result.Message)
	require.Equal(t, 0, result.DeletedObjects)
	require.Equal(t, 1, result.FailedObjects)
}

func TestDeleteSingleFileAndDirectoryMapCanceledErrorCode(t *testing.T) {
	service := NewService(nil, nil)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	fileProvider := &directoryDeleteWorkerPoolProvider{
		deleteCalls:          make(map[string]int),
		waitDeleteForContext: true,
	}
	fileResult := service.deleteSingleFileObject(canceledCtx, fileProvider, provider.DeleteObjectRequest{
		Bucket: "bucket",
		Region: "region",
	}, "assets/file.js")
	require.False(t, fileResult.Success)
	require.Equal(t, "delete_request_canceled", fileResult.ErrorCode)
	require.Equal(t, "delete request was canceled", fileResult.Message)

	directoryProvider := &directoryDeleteWorkerPoolProvider{
		waitListForContext: true,
		deleteCalls:        make(map[string]int),
	}
	directoryResult := service.deleteDirectoryRecursively(canceledCtx, directoryProvider, provider.DeleteObjectRequest{
		Bucket: "bucket",
		Region: "region",
		Key:    "assets/",
	})
	require.False(t, directoryResult.Success)
	require.Equal(t, "directory", directoryResult.TargetType)
	require.Equal(t, "delete_request_canceled", directoryResult.ErrorCode)
	require.Equal(t, "delete request was canceled", directoryResult.Message)
}

func TestServiceRenameBucketObjectUsesProviderBoundary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	providerStub := &fakeObjectStorageProvider{providerType: provider.TypeAliyun}
	require.NoError(t, service.RegisterObjectStorageProvider(providerStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "objects-rename-" + suffix,
		Description: "storage rename test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-rename-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-rename-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	result, err := service.RenameBucketObject(ctx, project.ID, RenameBucketObjectInput{
		SourceKey: "assets/old.css",
		TargetKey: "assets/new.css",
	})
	require.NoError(t, err)
	require.Equal(t, "bucket-rename-"+suffix, providerStub.lastRename.Bucket)
	require.Equal(t, "assets/old.css", providerStub.lastRename.SourceKey)
	require.Equal(t, "assets/new.css", providerStub.lastRename.TargetKey)
	require.Equal(t, "file", result.TargetType)
	require.True(t, result.Success)
	require.Equal(t, 1, result.MigratedObjects)
	require.Equal(t, 0, result.FailedObjects)
}

func TestServiceRenameBucketDirectoryMigratesObjects(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	providerStub := &fakeObjectStorageProvider{
		providerType: provider.TypeAliyun,
		objects: []provider.ObjectInfo{
			{Key: "assets/app.js", IsDir: false},
			{Key: "assets/images/", IsDir: true},
			{Key: "assets/images/logo.png", IsDir: false},
		},
	}
	require.NoError(t, service.RegisterObjectStorageProvider(providerStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "objects-rename-directory-" + suffix,
		Description: "storage rename directory test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-rename-directory-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-rename-directory-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	result, err := service.RenameBucketObject(ctx, project.ID, RenameBucketObjectInput{
		SourceKey: "assets/",
		TargetKey: "release",
	})
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, "directory", result.TargetType)
	require.Equal(t, "assets/", result.SourceKey)
	require.Equal(t, "release/", result.TargetKey)
	require.Equal(t, 2, result.MigratedObjects)
	require.Equal(t, 0, result.FailedObjects)
	require.Empty(t, result.FailureReasons)
	require.Len(t, providerStub.renameRequests, 2)
	require.Equal(t, "assets/app.js", providerStub.renameRequests[0].SourceKey)
	require.Equal(t, "release/app.js", providerStub.renameRequests[0].TargetKey)
	require.Equal(t, "assets/images/logo.png", providerStub.renameRequests[1].SourceKey)
	require.Equal(t, "release/images/logo.png", providerStub.renameRequests[1].TargetKey)
}

func TestServiceRenameBucketDirectoryReturnsPartialFailureSummary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	providerStub := &fakeObjectStorageProvider{
		providerType: provider.TypeAliyun,
		objects: []provider.ObjectInfo{
			{Key: "assets/app.js", IsDir: false},
			{Key: "assets/images/logo.png", IsDir: false},
		},
		renameErrors: map[string]error{
			"assets/images/logo.png": provider.NewError(
				provider.TypeAliyun,
				provider.ServiceObjectStorage,
				"rename_object",
				provider.ErrCodeOperationFailed,
				"rename failed",
				false,
				nil,
			),
		},
	}
	require.NoError(t, service.RegisterObjectStorageProvider(providerStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "objects-rename-directory-partial-" + suffix,
		Description: "storage rename directory partial failure test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-rename-directory-partial-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-rename-directory-partial-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	result, err := service.RenameBucketObject(ctx, project.ID, RenameBucketObjectInput{
		SourceKey: "assets/",
		TargetKey: "release/",
	})
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Equal(t, "provider_operation_failed", result.ErrorCode)
	require.Equal(t, 1, result.MigratedObjects)
	require.Equal(t, 1, result.FailedObjects)
	require.NotEmpty(t, result.FailureReasons)
	require.Contains(t, result.FailureReasons[0], "assets/images/logo.png")
	require.Len(t, providerStub.renameRequests, 2)
}

func TestDeleteDirectoryRecursivelyUsesWorkerPoolAndKeepsStats(t *testing.T) {
	service := NewService(nil, nil)
	service.ConfigureDeleteParallelism(3)

	providerStub := &directoryDeleteWorkerPoolProvider{
		objects: []provider.ObjectInfo{
			{Key: "assets/a.js", IsDir: false},
			{Key: "assets/b.js", IsDir: false},
			{Key: "assets/sub/", IsDir: true},
			{Key: "assets/sub/c.js", IsDir: false},
			{Key: "assets/b.js", IsDir: false},
		},
		deleteCalls: make(map[string]int),
	}

	result := service.deleteDirectoryRecursively(context.Background(), providerStub, provider.DeleteObjectRequest{
		Bucket: "bucket",
		Region: "region",
		Key:    "assets/",
	})

	require.True(t, result.Success)
	require.Equal(t, "directory", result.TargetType)
	require.Equal(t, 3, result.DeletedObjects)
	require.Equal(t, 0, result.FailedObjects)
	require.Equal(t, "directory deleted", result.Message)
	require.Equal(t, 3, providerStub.uniqueDeleteCallCount())
	require.Equal(t, 1, providerStub.deleteCalls["assets/a.js"])
	require.Equal(t, 1, providerStub.deleteCalls["assets/b.js"])
	require.Equal(t, 1, providerStub.deleteCalls["assets/sub/c.js"])
	require.GreaterOrEqual(t, providerStub.maxConcurrency(), 2)
}

func TestDeleteDirectoryRecursivelyKeepsFailureSummaryStableUnderConcurrency(t *testing.T) {
	service := NewService(nil, nil)
	service.ConfigureDeleteParallelism(4)

	providerStub := &directoryDeleteWorkerPoolProvider{
		objects: []provider.ObjectInfo{
			{Key: "assets/d.js", IsDir: false},
			{Key: "assets/c.js", IsDir: false},
			{Key: "assets/b.js", IsDir: false},
			{Key: "assets/a.js", IsDir: false},
			{Key: "assets/c.js", IsDir: false},
		},
		deleteErrors: map[string]error{
			"assets/a.js": provider.NewError(
				provider.TypeAliyun,
				provider.ServiceObjectStorage,
				"delete_object",
				provider.ErrCodeOperationFailed,
				"delete failed",
				false,
				nil,
			),
			"assets/c.js": provider.NewError(
				provider.TypeAliyun,
				provider.ServiceObjectStorage,
				"delete_object",
				provider.ErrCodeOperationFailed,
				"delete failed",
				false,
				nil,
			),
		},
		deleteCalls: make(map[string]int),
	}

	result := service.deleteDirectoryRecursively(context.Background(), providerStub, provider.DeleteObjectRequest{
		Bucket: "bucket",
		Region: "region",
		Key:    "assets/",
	})

	require.False(t, result.Success)
	require.Equal(t, "provider_operation_failed", result.ErrorCode)
	require.Equal(t, 2, result.DeletedObjects)
	require.Equal(t, 2, result.FailedObjects)
	require.Contains(t, result.Message, "directory delete partially failed:")
	require.Contains(t, result.Message, "assets/a.js:")
	require.Contains(t, result.Message, "assets/c.js:")
	require.Less(t, strings.Index(result.Message, "assets/a.js:"), strings.Index(result.Message, "assets/c.js:"))
	require.Equal(t, 4, providerStub.uniqueDeleteCallCount())
}

func TestDeleteDirectoryRecursivelyMapsTimeoutErrorCode(t *testing.T) {
	service := NewService(nil, nil)
	service.deleteTimeout = 20 * time.Millisecond

	providerStub := &directoryDeleteWorkerPoolProvider{
		waitListForContext: true,
		deleteCalls:        make(map[string]int),
	}

	result := service.deleteDirectoryRecursively(context.Background(), providerStub, provider.DeleteObjectRequest{
		Bucket: "bucket",
		Region: "region",
		Key:    "assets/",
	})

	require.False(t, result.Success)
	require.Equal(t, "directory", result.TargetType)
	require.Equal(t, "delete_request_timeout", result.ErrorCode)
	require.Equal(t, "delete request timed out", result.Message)
}

func TestServiceRefreshURLsUsesProviderBoundary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	cdnProviderStub := &fakeCDNProvider{
		providerType: provider.TypeAliyun,
		refreshURLsResult: provider.TaskResult{
			TaskID:      "refresh-url-task-" + suffix,
			Status:      "accepted",
			SubmittedAt: time.Now().UTC(),
		},
	}
	require.NoError(t, service.RegisterCDNProvider(cdnProviderStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "cdn-refresh-url-" + suffix,
		Description: "cdn refresh url test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-cdn-url-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-url-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "directory",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	result, err := service.RefreshURLs(ctx, project.ID, RefreshURLsInput{
		URLs: []string{" https://cdn.example.com/assets/app.js ", "", "https://cdn.example.com/assets/app.css"},
	})
	require.NoError(t, err)
	require.Equal(t, cdnProviderStub.refreshURLsResult.TaskID, result.TaskID)
	require.Equal(t, "https://cdn-url-"+suffix+".example.com", cdnProviderStub.lastRefreshURLs.Endpoint)
	require.Equal(t, []string{"https://cdn.example.com/assets/app.js", "https://cdn.example.com/assets/app.css"}, cdnProviderStub.lastRefreshURLs.URLs)
	require.Equal(t, "LTAI_TEST", cdnProviderStub.lastRefreshURLs.Credential.AccessKeyID)
	require.Empty(t, cdnProviderStub.lastRefreshDirectories.Directories)
}

func TestServiceRefreshDirectoriesUsesProviderBoundary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	cdnProviderStub := &fakeCDNProvider{
		providerType: provider.TypeAliyun,
		refreshDirectoriesResult: provider.TaskResult{
			TaskID:      "refresh-directory-task-" + suffix,
			Status:      "accepted",
			SubmittedAt: time.Now().UTC(),
		},
	}
	require.NoError(t, service.RegisterCDNProvider(cdnProviderStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "cdn-refresh-directory-" + suffix,
		Description: "cdn refresh directory test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-cdn-directory-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-directory-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	result, err := service.RefreshDirectories(ctx, project.ID, RefreshDirectoriesInput{
		Directories: []string{" /assets/ ", "", "/images/"},
	})
	require.NoError(t, err)
	require.Equal(t, cdnProviderStub.refreshDirectoriesResult.TaskID, result.TaskID)
	require.Equal(t, "https://cdn-directory-"+suffix+".example.com", cdnProviderStub.lastRefreshDirectories.Endpoint)
	require.Equal(t, []string{"/assets/", "/images/"}, cdnProviderStub.lastRefreshDirectories.Directories)
	require.Equal(t, "LTAI_TEST", cdnProviderStub.lastRefreshDirectories.Credential.AccessKeyID)
	require.Empty(t, cdnProviderStub.lastRefreshURLs.URLs)
}

func TestServiceSyncResourcesMapsProviderErrors(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
	registerTestProviders(t, service)
	ctx := context.Background()
	suffix := uniqueSuffix()

	cdnProviderStub := &fakeCDNProvider{
		providerType: provider.TypeAliyun,
		syncResourcesErr: provider.NewError(
			provider.TypeAliyun,
			provider.ServiceCDN,
			"sync_latest_resources",
			provider.ErrCodeTimeout,
			"provider timed out",
			true,
			nil,
		),
	}
	require.NoError(t, service.RegisterCDNProvider(cdnProviderStub))

	project, err := service.Create(ctx, CreateProjectInput{
		Name:        "cdn-sync-failure-" + suffix,
		Description: "cdn sync failure test",
		Buckets: []ProjectBucketInput{{
			BucketName: "bucket-cdn-sync-" + suffix,
			Region:     "cn-hangzhou",
			Credential: `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			IsPrimary:  true,
		}},
		CDNs: []ProjectCDNInput{{
			ProviderType: "aliyun",
			CDNEndpoint:  "https://cdn-sync-" + suffix + ".example.com",
			Credential:   `{"accessKeyId":"LTAI_TEST","accessKeySecret":"secret"}`,
			PurgeScope:   "url",
			IsPrimary:    true,
		}},
	})
	require.NoError(t, err)

	_, err = service.SyncResources(ctx, project.ID, SyncResourcesInput{
		Paths: []string{"assets/app.js"},
	})
	require.Error(t, err)

	appErr := &httpresp.AppError{}
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 504, appErr.StatusCode)
	require.Equal(t, "cdn_request_timeout", appErr.Code)
	require.Equal(t, "https://cdn-sync-"+suffix+".example.com", cdnProviderStub.lastSyncResources.Endpoint)
	require.Equal(t, "bucket-cdn-sync-"+suffix, cdnProviderStub.lastSyncResources.Bucket)
	require.Equal(t, []string{"assets/app.js"}, cdnProviderStub.lastSyncResources.Paths)
	require.True(t, cdnProviderStub.lastSyncResources.InvalidateCDN)
}

type fakeObjectStorageProvider struct {
	providerType   provider.Type
	objects        []provider.ObjectInfo
	listErrors     map[string]error
	renameErrors   map[string]error
	lastRequest    provider.ListObjectsRequest
	lastUpload     provider.UploadObjectRequest
	lastDelete     provider.DeleteObjectRequest
	lastRename     provider.RenameObjectRequest
	renameRequests []provider.RenameObjectRequest
}

func (f *fakeObjectStorageProvider) Type() provider.Type {
	return f.providerType
}

func (f *fakeObjectStorageProvider) Detect(_ context.Context, _ provider.CredentialPayload, _ string) (provider.Type, error) {
	return f.providerType, nil
}

func (f *fakeObjectStorageProvider) ListObjects(_ context.Context, req provider.ListObjectsRequest) ([]provider.ObjectInfo, error) {
	f.lastRequest = req
	normalizedPrefix := strings.TrimSpace(req.Prefix)
	if f.listErrors != nil {
		if err, ok := f.listErrors[normalizedPrefix]; ok {
			return nil, err
		}
	}

	filtered := make([]provider.ObjectInfo, 0, len(f.objects))
	for _, object := range f.objects {
		if normalizedPrefix == "" || strings.HasPrefix(object.Key, normalizedPrefix) {
			filtered = append(filtered, object)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	startIndex := 0
	marker := strings.TrimSpace(req.Marker)
	if marker != "" {
		found := false
		for idx := range filtered {
			if filtered[idx].Key == marker {
				startIndex = idx + 1
				found = true
				break
			}
		}
		if !found {
			return nil, nil
		}
	}
	if startIndex >= len(filtered) {
		return nil, nil
	}

	maxKeys := req.MaxKeys
	if maxKeys <= 0 {
		maxKeys = len(filtered)
	}
	endIndex := startIndex + maxKeys
	if endIndex > len(filtered) {
		endIndex = len(filtered)
	}
	return append([]provider.ObjectInfo(nil), filtered[startIndex:endIndex]...), nil
}

func (f *fakeObjectStorageProvider) UploadObject(_ context.Context, req provider.UploadObjectRequest) error {
	f.lastUpload = req
	if req.Content != nil {
		_, _ = io.Copy(ioutil.Discard, req.Content)
	}
	return nil
}

func (f *fakeObjectStorageProvider) DownloadObject(_ context.Context, _ provider.DownloadObjectRequest) (io.ReadCloser, provider.ObjectMeta, error) {
	return nil, provider.ObjectMeta{}, nil
}

func (f *fakeObjectStorageProvider) DeleteObject(_ context.Context, req provider.DeleteObjectRequest) error {
	f.lastDelete = req
	return nil
}

func (f *fakeObjectStorageProvider) RenameObject(_ context.Context, req provider.RenameObjectRequest) error {
	f.lastRename = req
	f.renameRequests = append(f.renameRequests, req)
	if f.renameErrors != nil {
		if err, ok := f.renameErrors[req.SourceKey]; ok {
			return err
		}
	}
	return nil
}

type fakeCDNProvider struct {
	providerType             provider.Type
	refreshURLsResult        provider.TaskResult
	refreshDirectoriesResult provider.TaskResult
	syncResourcesResult      provider.TaskResult
	syncResourcesErr         error
	lastRefreshURLs          provider.RefreshURLsRequest
	lastRefreshDirectories   provider.RefreshDirectoriesRequest
	lastSyncResources        provider.SyncResourcesRequest
}

type directoryDeleteWorkerPoolProvider struct {
	objects              []provider.ObjectInfo
	deleteErrors         map[string]error
	deleteCalls          map[string]int
	waitListForContext   bool
	waitDeleteForContext bool
	mu                   sync.Mutex
	inFlight             int
	maxInFlight          int
}

func (p *directoryDeleteWorkerPoolProvider) Type() provider.Type {
	return provider.TypeAliyun
}

func (p *directoryDeleteWorkerPoolProvider) Detect(_ context.Context, _ provider.CredentialPayload, _ string) (provider.Type, error) {
	return provider.TypeAliyun, nil
}

func (p *directoryDeleteWorkerPoolProvider) ListObjects(ctx context.Context, req provider.ListObjectsRequest) ([]provider.ObjectInfo, error) {
	if p.waitListForContext {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	filtered := make([]provider.ObjectInfo, 0, len(p.objects))
	for _, object := range p.objects {
		if req.Prefix == "" || strings.HasPrefix(object.Key, req.Prefix) {
			filtered = append(filtered, object)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	start := 0
	marker := strings.TrimSpace(req.Marker)
	if marker != "" {
		found := false
		for i := range filtered {
			if filtered[i].Key == marker {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return nil, nil
		}
	}
	if start >= len(filtered) {
		return nil, nil
	}

	maxKeys := req.MaxKeys
	if maxKeys <= 0 {
		maxKeys = len(filtered)
	}
	end := start + maxKeys
	if end > len(filtered) {
		end = len(filtered)
	}
	return append([]provider.ObjectInfo(nil), filtered[start:end]...), nil
}

func (p *directoryDeleteWorkerPoolProvider) UploadObject(_ context.Context, _ provider.UploadObjectRequest) error {
	return nil
}

func (p *directoryDeleteWorkerPoolProvider) DownloadObject(_ context.Context, _ provider.DownloadObjectRequest) (io.ReadCloser, provider.ObjectMeta, error) {
	return nil, provider.ObjectMeta{}, nil
}

func (p *directoryDeleteWorkerPoolProvider) DeleteObject(ctx context.Context, req provider.DeleteObjectRequest) error {
	if p.waitDeleteForContext {
		<-ctx.Done()
		return ctx.Err()
	}

	p.mu.Lock()
	p.inFlight++
	if p.inFlight > p.maxInFlight {
		p.maxInFlight = p.inFlight
	}
	p.mu.Unlock()

	time.Sleep(10 * time.Millisecond)

	p.mu.Lock()
	defer p.mu.Unlock()
	p.inFlight--
	p.deleteCalls[req.Key]++
	if p.deleteErrors != nil {
		if err, ok := p.deleteErrors[req.Key]; ok {
			return err
		}
	}
	return nil
}

func (p *directoryDeleteWorkerPoolProvider) RenameObject(_ context.Context, _ provider.RenameObjectRequest) error {
	return nil
}

func (p *directoryDeleteWorkerPoolProvider) uniqueDeleteCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.deleteCalls)
}

func (p *directoryDeleteWorkerPoolProvider) maxConcurrency() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.maxInFlight
}

func (f *fakeCDNProvider) Type() provider.Type {
	return f.providerType
}

func (f *fakeCDNProvider) RefreshURLs(_ context.Context, req provider.RefreshURLsRequest) (provider.TaskResult, error) {
	f.lastRefreshURLs = req
	return f.refreshURLsResult, nil
}

func (f *fakeCDNProvider) RefreshDirectories(_ context.Context, req provider.RefreshDirectoriesRequest) (provider.TaskResult, error) {
	f.lastRefreshDirectories = req
	return f.refreshDirectoriesResult, nil
}

func (f *fakeCDNProvider) SyncLatestResources(_ context.Context, req provider.SyncResourcesRequest) (provider.TaskResult, error) {
	f.lastSyncResources = req
	if f.syncResourcesErr != nil {
		return provider.TaskResult{}, f.syncResourcesErr
	}
	return f.syncResourcesResult, nil
}
