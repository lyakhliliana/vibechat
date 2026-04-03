package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"vibechat/internal/infrastructure/storage"
	cfgpkg "vibechat/utils/config"
	"vibechat/utils/logger"
)

// MigrateConfig is a minimal config for the migrate tool.
// It intentionally excludes JWT, delivery, and hasher settings
// so the migrate binary does not require application secrets.
type MigrateConfig struct {
	Storage storage.Config `json:"storage" yaml:"storage"`
	Logger  logger.Config  `json:"logger"  yaml:"logger"`
}

func LoadMigrate(path string) (*MigrateConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: open file `%s`: %w", path, err)
	}
	defer f.Close()

	var cfg MigrateConfig
	switch filepath.Ext(path) {
	case ".yaml", ".yml":
		if err = yaml.NewDecoder(f).Decode(&cfg); err != nil {
			return nil, fmt.Errorf("config: decode yaml: %w", err)
		}
	case ".json":
		if err = json.NewDecoder(f).Decode(&cfg); err != nil {
			return nil, fmt.Errorf("config: decode json: %w", err)
		}
	default:
		return nil, fmt.Errorf("config: unsupported extension %q", filepath.Ext(path))
	}

	cfg.SetDefaults()

	if err = cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *MigrateConfig) SetDefaults() {
	c.Storage.SetDefaults()
	c.Logger.SetDefaults()
}

func (c *MigrateConfig) Validate() error {
	ve := &cfgpkg.ValidationError{}
	ve.AddFrom("storage", c.Storage.Validate())
	ve.AddFrom("logger", c.Logger.Validate())
	return ve.Err()
}
