package postgres

import "vibechat/utils/config"

var DefaultConfig = Config{
	Port:           5432,
	SSLMode:        "disable",
	MaxConns:       10,
	MinConns:       2,
	MigrationsPath: "migrations/postgres",
}

type Config struct {
	Host           string `json:"host"             yaml:"host"`
	Port           int    `json:"port"             yaml:"port"`
	User           string `json:"user"             yaml:"user"`
	Password       string `json:"password"         yaml:"password"`
	DBName         string `json:"dbname"           yaml:"dbname"`
	SSLMode        string `json:"ssl_mode"         yaml:"ssl_mode"`
	MaxConns       int32  `json:"max_conns"        yaml:"max_conns"`
	MinConns       int32  `json:"min_conns"        yaml:"min_conns"`
	MigrationsPath string `json:"migrations_path"  yaml:"migrations_path"`
}

func (c *Config) SetDefaults() {
	if c.Port == 0 {
		c.Port = DefaultConfig.Port
	}
	if c.SSLMode == "" {
		c.SSLMode = DefaultConfig.SSLMode
	}
	if c.MaxConns == 0 {
		c.MaxConns = DefaultConfig.MaxConns
	}
	if c.MinConns == 0 {
		c.MinConns = DefaultConfig.MinConns
	}
	if c.MigrationsPath == "" {
		c.MigrationsPath = DefaultConfig.MigrationsPath
	}
}

func (c *Config) Validate() error {
	ve := &config.ValidationError{}

	if c.Host == "" {
		ve.Add("host", "required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		ve.Addf("port", "must be 1–65535, got %d", c.Port)
	}
	if c.User == "" {
		ve.Add("user", "required")
	}
	if c.Password == "" {
		ve.Add("password", "required")
	}
	if c.DBName == "" {
		ve.Add("dbname", "required")
	}
	if c.MaxConns < 1 {
		ve.Addf("max_conns", "must be ≥ 1, got %d", c.MaxConns)
	}
	if c.MinConns < 0 {
		ve.Addf("min_conns", "must be ≥ 0, got %d", c.MinConns)
	}
	if c.MaxConns > 0 && c.MinConns > c.MaxConns {
		ve.Addf("min_conns", "must be ≤ max_conns (%d), got %d", c.MaxConns, c.MinConns)
	}

	return ve.Err()
}
