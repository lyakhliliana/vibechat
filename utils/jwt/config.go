package jwt

import (
	"time"

	"vibechat/utils/config"
)

var DefaultConfig = Config{
	AccessTokenTTL:  config.Duration{Duration: 15 * time.Minute},
	RefreshTokenTTL: config.Duration{Duration: 7 * 24 * time.Hour},
}

type Config struct {
	AccessSecret    string          `json:"access_secret"    yaml:"access_secret"`
	RefreshSecret   string          `json:"refresh_secret"   yaml:"refresh_secret"`
	AccessTokenTTL  config.Duration `json:"access_token_ttl" yaml:"access_token_ttl"`
	RefreshTokenTTL config.Duration `json:"refresh_token_ttl" yaml:"refresh_token_ttl"`
}

func (c *Config) SetDefaults() {
	if c.AccessTokenTTL.Duration == 0 {
		c.AccessTokenTTL = DefaultConfig.AccessTokenTTL
	}
	if c.RefreshTokenTTL.Duration == 0 {
		c.RefreshTokenTTL = DefaultConfig.RefreshTokenTTL
	}
}

func (c *Config) Validate() error {
	ve := &config.ValidationError{}

	if c.AccessSecret == "" {
		ve.Add("access_secret", "required")
	} else if len(c.AccessSecret) < 32 {
		ve.Addf("access_secret", "must be ≥ 32 bytes for HS256 security, got %d", len(c.AccessSecret))
	}

	if c.RefreshSecret == "" {
		ve.Add("refresh_secret", "required")
	} else if len(c.RefreshSecret) < 32 {
		ve.Addf("refresh_secret", "must be ≥ 32 bytes for HS256 security, got %d", len(c.RefreshSecret))
	}

	if c.AccessSecret != "" && c.RefreshSecret != "" && c.AccessSecret == c.RefreshSecret {
		ve.Add("refresh_secret", "must differ from access_secret")
	}

	if c.AccessTokenTTL.Duration <= 0 {
		ve.Add("access_token_ttl", "must be positive")
	}
	if c.RefreshTokenTTL.Duration <= 0 {
		ve.Add("refresh_token_ttl", "must be positive")
	}
	if c.AccessTokenTTL.Duration > 0 && c.RefreshTokenTTL.Duration > 0 &&
		c.RefreshTokenTTL.Duration <= c.AccessTokenTTL.Duration {
		ve.Add("refresh_token_ttl", "must be greater than AccessTokenTTL")
	}

	return ve.Err()
}
