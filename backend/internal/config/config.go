package config

import "errors"

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
	RequestLimit int              `yaml:"request_limit"`
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
	case c.RequestLimit <= 0:
		return errors.New("request limit must be positive")
	}
	return nil
}
