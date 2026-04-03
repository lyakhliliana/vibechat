package http

import (
	"time"

	"vibechat/utils/config"
)

var DefaultConfig = Config{
	Host:         "0.0.0.0",
	Port:         8080,
	ReadTimeout:  config.Duration{Duration: 5 * time.Second},
	WriteTimeout: config.Duration{Duration: 10 * time.Second},
	IdleTimeout:  config.Duration{Duration: 60 * time.Second},
	RateLimit: RateLimitConfig{
		MaxRequests: 20,
		Window:      config.Duration{Duration: 60 * time.Second},
	},
	MaxBodyBytes: 4 << 20, // 4 MB
}

type TLSConfig struct {
	CertFile string `json:"cert_file" yaml:"cert_file"`
	KeyFile  string `json:"key_file"  yaml:"key_file"`
}

type RateLimitConfig struct {
	MaxRequests int             `json:"max_requests" yaml:"max_requests"`
	Window      config.Duration `json:"window"       yaml:"window"`
}

type Config struct {
	Host         string          `json:"host"            yaml:"host"`
	Port         int             `json:"port"            yaml:"port"`
	ReadTimeout  config.Duration `json:"read_timeout"    yaml:"read_timeout"`
	WriteTimeout config.Duration `json:"write_timeout"   yaml:"write_timeout"`
	IdleTimeout  config.Duration `json:"idle_timeout"    yaml:"idle_timeout"`
	// AllowedOrigins is passed to the CORS middleware.
	// An empty slice (the default) allows all origins.
	AllowedOrigins []string `json:"allowed_origins" yaml:"allowed_origins"`
	// RateLimit applies a per-IP fixed-window limiter to the auth endpoints.
	RateLimit RateLimitConfig `json:"rate_limit"      yaml:"rate_limit"`
	// MaxBodyBytes is the maximum allowed request body size in bytes.
	MaxBodyBytes int64 `json:"max_body_bytes"  yaml:"max_body_bytes"`
	// TLS enables HTTPS when CertFile and KeyFile are set.
	TLS *TLSConfig `json:"tls"             yaml:"tls"`
}

func (c *Config) SetDefaults() {
	if c.Host == "" {
		c.Host = DefaultConfig.Host
	}
	if c.Port == 0 {
		c.Port = DefaultConfig.Port
	}
	if c.ReadTimeout.Duration == 0 {
		c.ReadTimeout = DefaultConfig.ReadTimeout
	}
	if c.WriteTimeout.Duration == 0 {
		c.WriteTimeout = DefaultConfig.WriteTimeout
	}
	if c.IdleTimeout.Duration == 0 {
		c.IdleTimeout = DefaultConfig.IdleTimeout
	}
	if c.RateLimit.MaxRequests == 0 {
		c.RateLimit = DefaultConfig.RateLimit
	}
	if c.MaxBodyBytes == 0 {
		c.MaxBodyBytes = DefaultConfig.MaxBodyBytes
	}
}

func (c *Config) TLSEnabled() bool {
	return c.TLS != nil && c.TLS.CertFile != "" && c.TLS.KeyFile != ""
}

func (c *Config) Validate() error {
	ve := &config.ValidationError{}

	if c.Port <= 0 || c.Port > 65535 {
		ve.Addf("port", "must be 1–65535, got %d", c.Port)
	}
	if c.RateLimit.MaxRequests <= 0 {
		ve.Add("rate_limit.max_requests", "must be positive")
	}
	if c.RateLimit.Window.Duration <= 0 {
		ve.Add("rate_limit.window", "must be positive")
	}
	if c.MaxBodyBytes <= 0 {
		ve.Add("max_body_bytes", "must be positive")
	}
	if c.ReadTimeout.Duration <= 0 {
		ve.Add("read_timeout", "must be positive")
	}
	if c.WriteTimeout.Duration <= 0 {
		ve.Add("write_timeout", "must be positive")
	}
	if c.IdleTimeout.Duration <= 0 {
		ve.Add("idle_timeout", "must be positive")
	}
	if c.TLS != nil {
		if c.TLS.CertFile == "" {
			ve.Add("tls.cert_file", "required when tls section is present")
		}
		if c.TLS.KeyFile == "" {
			ve.Add("tls.key_file", "required when tls section is present")
		}
	}

	return ve.Err()
}
