package config

import "testing"

func TestAppConfigApplyDefaultsBackwardCompatible(t *testing.T) {
	cfg := &AppConfig{}

	cfg.ApplyDefaults()

	if cfg.Upload.MaxFileSizeMB != DefaultMaxUploadFileSizeMB {
		t.Fatalf("unexpected upload max_file_size_mb: got %d want %d", cfg.Upload.MaxFileSizeMB, DefaultMaxUploadFileSizeMB)
	}
	if cfg.Upload.ArchiveParallelism != DefaultArchiveParallelism {
		t.Fatalf("unexpected upload archive_parallelism: got %d want %d", cfg.Upload.ArchiveParallelism, DefaultArchiveParallelism)
	}
	if cfg.Upload.FileParallelism != DefaultUploadFileParallelism {
		t.Fatalf("unexpected upload file_parallelism: got %d want %d", cfg.Upload.FileParallelism, DefaultUploadFileParallelism)
	}

	if cfg.Delete.Parallelism != DefaultDeleteParallelism {
		t.Fatalf("unexpected delete parallelism: got %d want %d", cfg.Delete.Parallelism, DefaultDeleteParallelism)
	}
	if cfg.Delete.BatchParallelism != DefaultDeleteBatchParallelism {
		t.Fatalf("unexpected delete batch_parallelism: got %d want %d", cfg.Delete.BatchParallelism, DefaultDeleteBatchParallelism)
	}
	if cfg.Delete.FileParallelism != DefaultDeleteFileParallelism {
		t.Fatalf("unexpected delete file_parallelism: got %d want %d", cfg.Delete.FileParallelism, DefaultDeleteFileParallelism)
	}
	if cfg.Delete.RequestTimeoutSeconds != DefaultDeleteRequestTimeoutSec {
		t.Fatalf("unexpected delete request_timeout_seconds: got %d want %d", cfg.Delete.RequestTimeoutSeconds, DefaultDeleteRequestTimeoutSec)
	}
	if cfg.Delete.ListPageSize != DefaultDeleteListPageSize {
		t.Fatalf("unexpected delete list_page_size: got %d want %d", cfg.Delete.ListPageSize, DefaultDeleteListPageSize)
	}
}

func TestApplyDefaultsClampRange(t *testing.T) {
	cfg := &AppConfig{
		Upload: UploadConfig{
			ArchiveParallelism: -1,
			FileParallelism:    100,
		},
		Delete: DeleteConfig{
			Parallelism:           -2,
			BatchParallelism:      100,
			FileParallelism:       -3,
			RequestTimeoutSeconds: 999,
			ListPageSize:          1,
		},
	}

	cfg.ApplyDefaults()

	if cfg.Upload.ArchiveParallelism != MinArchiveParallelism {
		t.Fatalf("unexpected upload archive_parallelism clamp: got %d want %d", cfg.Upload.ArchiveParallelism, MinArchiveParallelism)
	}
	if cfg.Upload.FileParallelism != MaxUploadFileParallelism {
		t.Fatalf("unexpected upload file_parallelism clamp: got %d want %d", cfg.Upload.FileParallelism, MaxUploadFileParallelism)
	}
	if cfg.Delete.Parallelism != MinDeleteParallelism {
		t.Fatalf("unexpected delete parallelism clamp: got %d want %d", cfg.Delete.Parallelism, MinDeleteParallelism)
	}
	if cfg.Delete.BatchParallelism != MaxDeleteBatchParallelism {
		t.Fatalf("unexpected delete batch_parallelism clamp: got %d want %d", cfg.Delete.BatchParallelism, MaxDeleteBatchParallelism)
	}
	if cfg.Delete.FileParallelism != MinDeleteFileParallelism {
		t.Fatalf("unexpected delete file_parallelism clamp: got %d want %d", cfg.Delete.FileParallelism, MinDeleteFileParallelism)
	}
	if cfg.Delete.RequestTimeoutSeconds != MaxDeleteRequestTimeoutSec {
		t.Fatalf("unexpected delete request_timeout_seconds clamp: got %d want %d", cfg.Delete.RequestTimeoutSeconds, MaxDeleteRequestTimeoutSec)
	}
	if cfg.Delete.ListPageSize != MinDeleteListPageSize {
		t.Fatalf("unexpected delete list_page_size clamp: got %d want %d", cfg.Delete.ListPageSize, MinDeleteListPageSize)
	}
}
