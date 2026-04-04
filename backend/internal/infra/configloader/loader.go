package configloader

import (
	"fmt"
	"os"
	"strconv"

	"github.com/baihua19941101/cdnManage/internal/config"
)

// Load reads environment variables and merges them with defaults.
func Load() (*config.AppConfig, error) {
	cfg := config.NewDefault()

	if v := os.Getenv("SERVER_PORT"); v != "" {
		if port, err := parseInt(v); err == nil {
			cfg.Server.Port = port
		} else {
			return nil, fmt.Errorf("invalid SERVER_PORT: %w", err)
		}
	}

	withString(&cfg.MySQL.Host, "MYSQL_HOST")
	withInt(&cfg.MySQL.Port, "MYSQL_PORT")
	withString(&cfg.MySQL.User, "MYSQL_USER")
	withString(&cfg.MySQL.Password, "MYSQL_PASSWORD")
	withString(&cfg.MySQL.Database, "MYSQL_DATABASE")
	withInt(&cfg.MySQL.MaxOpenConns, "MYSQL_MAX_OPEN_CONNS")
	withInt(&cfg.MySQL.MaxIdleConns, "MYSQL_MAX_IDLE_CONNS")

	withString(&cfg.Redis.Host, "REDIS_HOST")
	withInt(&cfg.Redis.Port, "REDIS_PORT")
	withString(&cfg.Redis.Password, "REDIS_PASSWORD")

	withString(&cfg.JWT.Secret, "JWT_SECRET")
	withInt(&cfg.JWT.LifespanSeconds, "JWT_LIFESPAN_SECONDS")
	withString(&cfg.JWT.Issuer, "JWT_ISSUER")

	withString(&cfg.Session.Secret, "SESSION_SECRET")
	withString(&cfg.Encryption.Key, "ENCRYPTION_KEY")
	withInt(&cfg.RequestLimit, "REQUEST_LIMIT")

	withString(&cfg.SuperAdmin.Email, "SUPER_ADMIN_EMAIL")
	withString(&cfg.SuperAdmin.Password, "SUPER_ADMIN_PASSWORD")

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func withString(target *string, env string) {
	if v := os.Getenv(env); v != "" {
		*target = v
	}
}

func withInt(target *int, env string) {
	if v := os.Getenv(env); v != "" {
		if parsed, err := parseInt(v); err == nil {
			*target = parsed
		}
	}
}

func parseInt(raw string) (int, error) {
	return strconv.Atoi(raw)
}
