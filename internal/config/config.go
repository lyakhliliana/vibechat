package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"vibechat/internal/delivery"
	"vibechat/internal/infrastructure/cache"
	"vibechat/internal/infrastructure/storage"
	cfgpkg "vibechat/utils/config"
	"vibechat/utils/hasher"
	"vibechat/utils/jwt"
	"vibechat/utils/logger"
)

var DefaultConfig = AppConfig{
	Storage:  storage.DefaultConfig,
	JWT:      jwt.DefaultConfig,
	Logger:   logger.DefaultConfig,
	Hasher:   hasher.DefaultConfig,
	Delivery: delivery.DefaultConfig,
	Profile: ProfileConfig{
		Addr: "localhost:8082",
	},
}

type AppConfig struct {
	Storage  storage.Config  `json:"storage"  yaml:"storage"`
	Cache    *cache.Config   `json:"cache"    yaml:"cache"` // optional; nil disables caching
	JWT      jwt.Config      `json:"jwt"      yaml:"jwt"`
	Logger   logger.Config   `json:"logger"   yaml:"logger"`
	Hasher   hasher.Config   `json:"hasher"   yaml:"hasher"`
	Delivery delivery.Config `json:"delivery" yaml:"delivery"`
	Profile  ProfileConfig   `json:"profile"  yaml:"profile"`
}

type ProfileConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Addr    string `json:"addr"    yaml:"addr"`
}

func Load(path string) (*AppConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("config: open file `%s`: %w", path, err)
	}
	defer f.Close()

	var cfg AppConfig
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
		return nil, fmt.Errorf("config: unsupported extension `%q`", filepath.Ext(path))
	}

	cfg.SetDefaults()

	if err = cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *ProfileConfig) SetDefaults() {
	if c.Addr == "" {
		c.Addr = "localhost:8082"
	}
}

func (c *AppConfig) SetDefaults() {
	c.Storage.SetDefaults()
	if c.Cache != nil {
		c.Cache.SetDefaults()
	}
	c.JWT.SetDefaults()
	c.Logger.SetDefaults()
	c.Hasher.SetDefaults()
	c.Delivery.SetDefaults()
	c.Profile.SetDefaults()
}

func (c *AppConfig) Validate() error {
	ve := &cfgpkg.ValidationError{}

	ve.AddFrom("storage", c.Storage.Validate())
	if c.Cache != nil {
		ve.AddFrom("cache", c.Cache.Validate())
	}
	ve.AddFrom("jwt", c.JWT.Validate())
	ve.AddFrom("logger", c.Logger.Validate())
	ve.AddFrom("hasher", c.Hasher.Validate())
	ve.AddFrom("delivery", c.Delivery.Validate())

	return ve.Err()
}
