package configloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/baihua19941101/cdnManage/internal/config"
	"gopkg.in/yaml.v3"
)

var defaultConfigPaths = []string{
	"config.yaml",
	filepath.Join("backend", "config.yaml"),
}

// Load reads application configuration from a YAML file.
func Load() (*config.AppConfig, error) {
	configPath, err := resolveConfigPath()
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", configPath, err)
	}

	cfg := &config.AppConfig{}
	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("decode config file %q: %w", configPath, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func resolveConfigPath() (string, error) {
	for _, candidate := range defaultConfigPaths {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("config file not found, expected one of: %s", strings.Join(defaultConfigPaths, ", "))
}
