package mysql

import (
	"fmt"
	"net/url"

	"vibechat/utils/config"
)

var DefaultConfig = Config{
	Port:           3306,
	MaxConns:       10,
	MinConns:       2,
	MigrationsPath: "migrations/mysql",
}

type Config struct {
	Host           string `json:"host"             yaml:"host"`
	Port           int    `json:"port"             yaml:"port"`
	User           string `json:"user"             yaml:"user"`
	Password       string `json:"password"         yaml:"password"`
	DBName         string `json:"dbname"           yaml:"dbname"`
	MaxConns       int    `json:"max_conns"        yaml:"max_conns"`
	MinConns       int    `json:"min_conns"        yaml:"min_conns"`
	MigrationsPath string `json:"migrations_path"  yaml:"migrations_path"`
}

func (c *Config) SetDefaults() {
	if c.Port == 0 {
		c.Port = DefaultConfig.Port
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

// DSN returns the data source name for database/sql.
func (c *Config) DSN() string {
	params := url.Values{}
	params.Set("parseTime", "true")
	params.Set("charset", "utf8mb4")
	params.Set("collation", "utf8mb4_unicode_ci")
	params.Set("loc", "UTC")
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, params.Encode())
}

// MigrateDSN returns the DSN for golang-migrate (requires multiStatements=true).
func (c *Config) MigrateDSN() string {
	params := url.Values{}
	params.Set("parseTime", "true")
	params.Set("charset", "utf8mb4")
	params.Set("multiStatements", "true")
	return fmt.Sprintf("mysql://%s:%s@tcp(%s:%d)/%s?%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, params.Encode())
}
