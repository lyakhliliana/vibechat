package storage

import (
	"vibechat/internal/infrastructure/storage/mysql"
	"vibechat/internal/infrastructure/storage/postgres"
	"vibechat/utils/config"
)

type Type string

const (
	TypePostgres Type = "postgres"
	TypeMySQL    Type = "mysql"
)

var DefaultConfig = Config{
	Type:     TypePostgres,
	Postgres: &postgres.DefaultConfig,
}

type Config struct {
	Type     Type             `json:"type"     yaml:"type"`
	Postgres *postgres.Config `json:"postgres" yaml:"postgres"`
	MySQL    *mysql.Config    `json:"mysql"    yaml:"mysql"`
}

func (c *Config) SetDefaults() {
	if c.Type == "" {
		c.Type = TypePostgres
	}
	if c.Postgres != nil {
		c.Postgres.SetDefaults()
	}
	if c.MySQL != nil {
		c.MySQL.SetDefaults()
	}
}

func (c *Config) Validate() error {
	ve := &config.ValidationError{}

	switch c.Type {
	case TypePostgres:
		if c.Postgres == nil {
			ve.Add("postgres", "required for type postgres")
		} else {
			ve.AddFrom("postgres", c.Postgres.Validate())
		}
	case TypeMySQL:
		if c.MySQL == nil {
			ve.Add("mysql", "required for type mysql")
		} else {
			ve.AddFrom("mysql", c.MySQL.Validate())
		}
	default:
		ve.Addf("type", "unknown storage type %q", c.Type)
	}

	return ve.Err()
}
