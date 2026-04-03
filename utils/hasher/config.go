package hasher

import (
	"golang.org/x/crypto/bcrypt"

	"vibechat/utils/config"
)

var DefaultConfig = Config{
	Cost: bcrypt.DefaultCost,
}

type Config struct {
	Cost int `json:"cost" yaml:"cost"`
}

func (c *Config) SetDefaults() {
	if c.Cost == 0 {
		c.Cost = DefaultConfig.Cost
	}
}

func (c *Config) Validate() error {
	ve := &config.ValidationError{}

	if c.Cost < bcrypt.MinCost || c.Cost > bcrypt.MaxCost {
		ve.Addf("cost", "must be %d–%d, got %d", bcrypt.MinCost, bcrypt.MaxCost, c.Cost)
	}

	return ve.Err()
}
