package projects

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
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

func TestServiceRenameBucketObjectUsesProviderBoundary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
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

func TestServiceRefreshURLsUsesProviderBoundary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
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
			PurgeScope:   "url",
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
}

func TestServiceRefreshDirectoriesUsesProviderBoundary(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
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
			PurgeScope:   "directory",
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
}

func TestServiceSyncResourcesMapsProviderErrors(t *testing.T) {
	db := newTestDB(t)
	store := repository.NewGormStore(db)
	service := NewService(store.Projects(), repository.NewGormTxManager(db), secure.NewCredentialCipher("projects-service-test-key"))
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
