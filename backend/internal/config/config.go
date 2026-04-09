package config

import "errors"

const (
	DefaultMaxUploadFileSizeMB   int64 = 20
	DefaultArchiveParallelism          = 4
	MinArchiveParallelism              = 2
	MaxArchiveParallelism              = 8
	DefaultUploadFileParallelism       = 4
	MinUploadFileParallelism           = 1
	MaxUploadFileParallelism           = 32

	DefaultDeleteParallelism       = 2
	MinDeleteParallelism           = 1
	MaxDeleteParallelism           = 16
	DefaultDeleteBatchParallelism  = 4
	MinDeleteBatchParallelism      = 1
	MaxDeleteBatchParallelism      = 32
	DefaultDeleteFileParallelism   = 8
	MinDeleteFileParallelism       = 1
	MaxDeleteFileParallelism       = 64
	DefaultDeleteRequestTimeoutSec = 30
	MinDeleteRequestTimeoutSec     = 5
	MaxDeleteRequestTimeoutSec     = 300
	DefaultDeleteListPageSize      = 100
	MinDeleteListPageSize          = 10
	MaxDeleteListPageSize          = 1000

	bytesPerMB int64 = 1024 * 1024
)

// ServerConfig contains network settings for the HTTP server.
type ServerConfig struct {
	Port int `yaml:"port"`
}

// DatabaseConfig models persistent storage settings.
type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// RedisConfig holds cache/session connection metadata.
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
}

// JWTConfig contains token signing secrets and metadata.
type JWTConfig struct {
	Secret          string `yaml:"secret"`
	Issuer          string `yaml:"issuer"`
	LifespanSeconds int    `yaml:"lifespan_seconds"`
}

// SessionConfig captures server-side session secret.
type SessionConfig struct {
	Secret string `yaml:"secret"`
}

// EncryptionConfig stores the application-level encryption key.
type EncryptionConfig struct {
	Key string `yaml:"key"`
}

// SuperAdminConfig defines the required info for the auto-provisioned super admin.
type SuperAdminConfig struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
}

// CORSConfig defines cross-origin request policy.
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	ExposeHeaders    []string `yaml:"expose_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAgeSeconds    int      `yaml:"max_age_seconds"`
}

// UploadConfig defines upload validation limits.
type UploadConfig struct {
	MaxFileSizeMB      int64 `yaml:"max_file_size_mb"`
	ArchiveParallelism int   `yaml:"archive_parallelism"`
	FileParallelism    int   `yaml:"file_parallelism"`
}

func (c *UploadConfig) ApplyDefaults() {
	if c.MaxFileSizeMB == 0 {
		c.MaxFileSizeMB = DefaultMaxUploadFileSizeMB
	}
	if c.ArchiveParallelism == 0 {
		c.ArchiveParallelism = DefaultArchiveParallelism
	}
	if c.ArchiveParallelism < MinArchiveParallelism {
		c.ArchiveParallelism = MinArchiveParallelism
	}
	if c.ArchiveParallelism > MaxArchiveParallelism {
		c.ArchiveParallelism = MaxArchiveParallelism
	}
	if c.FileParallelism == 0 {
		c.FileParallelism = DefaultUploadFileParallelism
	}
	if c.FileParallelism < MinUploadFileParallelism {
		c.FileParallelism = MinUploadFileParallelism
	}
	if c.FileParallelism > MaxUploadFileParallelism {
		c.FileParallelism = MaxUploadFileParallelism
	}
}

func (c UploadConfig) MaxFileSizeBytes() int64 {
	maxFileSizeMB := c.MaxFileSizeMB
	if maxFileSizeMB <= 0 {
		maxFileSizeMB = DefaultMaxUploadFileSizeMB
	}
	return maxFileSizeMB * bytesPerMB
}

// DeleteConfig defines delete operation limits.
type DeleteConfig struct {
	Parallelism           int `yaml:"parallelism"`
	BatchParallelism      int `yaml:"batch_parallelism"`
	FileParallelism       int `yaml:"file_parallelism"`
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds"`
	ListPageSize          int `yaml:"list_page_size"`
}

func (c *DeleteConfig) ApplyDefaults() {
	if c.Parallelism == 0 {
		c.Parallelism = DefaultDeleteParallelism
	}
	if c.Parallelism < MinDeleteParallelism {
		c.Parallelism = MinDeleteParallelism
	}
	if c.Parallelism > MaxDeleteParallelism {
		c.Parallelism = MaxDeleteParallelism
	}

	if c.BatchParallelism == 0 {
		c.BatchParallelism = DefaultDeleteBatchParallelism
	}
	if c.BatchParallelism < MinDeleteBatchParallelism {
		c.BatchParallelism = MinDeleteBatchParallelism
	}
	if c.BatchParallelism > MaxDeleteBatchParallelism {
		c.BatchParallelism = MaxDeleteBatchParallelism
	}

	if c.FileParallelism == 0 {
		c.FileParallelism = DefaultDeleteFileParallelism
	}
	if c.FileParallelism < MinDeleteFileParallelism {
		c.FileParallelism = MinDeleteFileParallelism
	}
	if c.FileParallelism > MaxDeleteFileParallelism {
		c.FileParallelism = MaxDeleteFileParallelism
	}

	if c.RequestTimeoutSeconds == 0 {
		c.RequestTimeoutSeconds = DefaultDeleteRequestTimeoutSec
	}
	if c.RequestTimeoutSeconds < MinDeleteRequestTimeoutSec {
		c.RequestTimeoutSeconds = MinDeleteRequestTimeoutSec
	}
	if c.RequestTimeoutSeconds > MaxDeleteRequestTimeoutSec {
		c.RequestTimeoutSeconds = MaxDeleteRequestTimeoutSec
	}

	if c.ListPageSize == 0 {
		c.ListPageSize = DefaultDeleteListPageSize
	}
	if c.ListPageSize < MinDeleteListPageSize {
		c.ListPageSize = MinDeleteListPageSize
	}
	if c.ListPageSize > MaxDeleteListPageSize {
		c.ListPageSize = MaxDeleteListPageSize
	}
}

// AppConfig aggregates all configurable properties.
type AppConfig struct {
	Server       ServerConfig     `yaml:"server"`
	MySQL        DatabaseConfig   `yaml:"mysql"`
	Redis        RedisConfig      `yaml:"redis"`
	JWT          JWTConfig        `yaml:"jwt"`
	Session      SessionConfig    `yaml:"session"`
	Encryption   EncryptionConfig `yaml:"encryption"`
	SuperAdmin   SuperAdminConfig `yaml:"super_admin"`
	CORS         CORSConfig       `yaml:"cors"`
	Upload       UploadConfig     `yaml:"upload"`
	Delete       DeleteConfig     `yaml:"delete"`
	RequestLimit int              `yaml:"request_limit"`
}

func (c *AppConfig) ApplyDefaults() {
	c.Upload.ApplyDefaults()
	c.Delete.ApplyDefaults()
}

// Validate ensures required fields are present.
func (c *AppConfig) Validate() error {
	switch {
	case c.Server.Port <= 0:
		return errors.New("server port must be positive")
	case c.MySQL.Host == "":
		return errors.New("mysql host is required")
	case c.MySQL.User == "":
		return errors.New("mysql user is required")
	case c.MySQL.Password == "":
		return errors.New("mysql password is required")
	case c.MySQL.Database == "":
		return errors.New("mysql database is required")
	case c.Redis.Host == "":
		return errors.New("redis host is required")
	case c.Redis.Port <= 0:
		return errors.New("redis port must be positive")
	case c.JWT.Secret == "":
		return errors.New("jwt secret is required")
	case c.JWT.Issuer == "":
		return errors.New("jwt issuer is required")
	case c.JWT.LifespanSeconds <= 0:
		return errors.New("jwt lifespan_seconds must be positive")
	case c.Session.Secret == "":
		return errors.New("session secret is required")
	case c.Encryption.Key == "":
		return errors.New("encryption key is required")
	case c.SuperAdmin.Email == "":
		return errors.New("super admin email is required")
	case c.SuperAdmin.Password == "":
		return errors.New("super admin password is required")
	case len(c.CORS.AllowOrigins) == 0:
		return errors.New("cors allow_origins is required")
	case len(c.CORS.AllowMethods) == 0:
		return errors.New("cors allow_methods is required")
	case len(c.CORS.AllowHeaders) == 0:
		return errors.New("cors allow_headers is required")
	case c.CORS.MaxAgeSeconds < 0:
		return errors.New("cors max_age_seconds must be greater than or equal to 0")
	case c.Upload.MaxFileSizeMB < 0:
		return errors.New("upload max_file_size_mb must be greater than or equal to 0")
	case c.RequestLimit <= 0:
		return errors.New("request limit must be positive")
	}
	return nil
}
