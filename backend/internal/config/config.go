package config

import "errors"

const (
	defaultServerPort   = 8080
	defaultRedisPort    = 6379
	defaultMySQLPort    = 3306
	defaultJWTIssuer    = "cdn-management-platform"
	defaultRequestLimit = 100
)

// ServerConfig contains network settings for the HTTP server.
type ServerConfig struct {
	Port int
}

// DatabaseConfig models persistent storage settings.
type DatabaseConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	Database     string
	MaxOpenConns int
	MaxIdleConns int
}

// RedisConfig holds cache/session connection metadata.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
}

// JWTConfig contains token signing secrets and metadata.
type JWTConfig struct {
	Secret          string
	Issuer          string
	LifespanSeconds int
}

// SessionConfig captures server-side session secret.
type SessionConfig struct {
	Secret string
}

// EncryptionConfig stores the application-level encryption key.
type EncryptionConfig struct {
	Key string
}

// SuperAdminConfig defines the required info for the auto-provisioned super admin.
type SuperAdminConfig struct {
	Email    string
	Password string
}

// AppConfig aggregates all configurable properties.
type AppConfig struct {
	Server       ServerConfig
	MySQL        DatabaseConfig
	Redis        RedisConfig
	JWT          JWTConfig
	Session      SessionConfig
	Encryption   EncryptionConfig
	SuperAdmin   SuperAdminConfig
	RequestLimit int
}

// NewDefault returns a config instance populated with safe defaults.
func NewDefault() *AppConfig {
	return &AppConfig{
		Server: ServerConfig{
			Port: defaultServerPort,
		},
		MySQL: DatabaseConfig{
			Port:         defaultMySQLPort,
			MaxOpenConns: 20,
			MaxIdleConns: 5,
		},
		Redis: RedisConfig{
			Port: defaultRedisPort,
		},
		JWT: JWTConfig{
			Issuer:          defaultJWTIssuer,
			LifespanSeconds: 3600,
		},
		RequestLimit: defaultRequestLimit,
	}
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
	case c.JWT.Secret == "":
		return errors.New("jwt secret is required")
	case c.Session.Secret == "":
		return errors.New("session secret is required")
	case c.Encryption.Key == "":
		return errors.New("encryption key is required")
	case c.SuperAdmin.Email == "":
		return errors.New("super admin email is required")
	case c.SuperAdmin.Password == "":
		return errors.New("super admin password is required")
	}
	return nil
}
