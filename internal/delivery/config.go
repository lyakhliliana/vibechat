package delivery

import (
	"slices"
	deliveryhttp "vibechat/internal/delivery/http"
	deliveryws "vibechat/internal/delivery/ws"
	"vibechat/utils/config"
)

type API string

const (
	HTTP API = "http"
	WS   API = "ws"
)

var DefaultConfig = Config{
	Enabled: []API{HTTP, WS},
	HTTP:    deliveryhttp.DefaultConfig,
	WS:      deliveryws.DefaultConfig,
}

type Config struct {
	Enabled []API               `json:"enabled" yaml:"enabled"`
	HTTP    deliveryhttp.Config `json:"http"    yaml:"http"`
	WS      deliveryws.Config   `json:"ws"      yaml:"ws"`
}

func (c *Config) IsEnabled(api API) bool {
	return slices.Contains(c.Enabled, api)
}

func (c *Config) SetDefaults() {
	if len(c.Enabled) == 0 {
		c.Enabled = DefaultConfig.Enabled
	}
	c.HTTP.SetDefaults()
	c.WS.SetDefaults()
}

func (c *Config) Validate() error {
	ve := &config.ValidationError{}
	if c.IsEnabled(HTTP) {
		ve.AddFrom("http", c.HTTP.Validate())
	}
	return ve.Err()
}
