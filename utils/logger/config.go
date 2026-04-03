package logger

import (
	"strings"

	"vibechat/utils/config"
)

type Type string

const (
	TypeStdout  Type = "stdout"
	TypeConsole Type = "console"
	TypeFile    Type = "file"
)

type FileConfig struct {
	Path       string `json:"path"        yaml:"path"`
	Owner      string `json:"owner"       yaml:"owner"`
	Group      string `json:"group"       yaml:"group"`
	Mode       uint32 `json:"mode"        yaml:"mode"`
	MaxBackups int    `json:"max_backups" yaml:"max_backups"`
	MaxSizeMB  int    `json:"max_size"    yaml:"max_size"`
}

var DefaultConfig = Config{
	Type:  TypeStdout,
	Level: "info",
}

type Config struct {
	Type  Type        `json:"type"  yaml:"type"`
	Level string      `json:"level" yaml:"level"`
	File  *FileConfig `json:"file"  yaml:"file"`
}

func (c *Config) SetDefaults() {
	if c.Type == "" {
		c.Type = TypeStdout
	}
	if c.Level == "" {
		c.Level = "info"
	}
	if c.File != nil {
		if c.File.Mode == 0 {
			c.File.Mode = 0644
		}
		if c.File.MaxSizeMB == 0 {
			c.File.MaxSizeMB = 256
		}
		if c.File.MaxBackups == 0 {
			c.File.MaxBackups = 3
		}
	}
}

var validLevels = map[string]bool{
	"trace": true, "debug": true, "info": true,
	"warn": true, "error": true, "fatal": true, "panic": true,
}

func (c *Config) Validate() error {
	ve := &config.ValidationError{}

	if c.Level != "" && !validLevels[strings.ToLower(c.Level)] {
		ve.Addf("level", "unknown value %q; valid: trace debug info warn error fatal panic", c.Level)
	}

	switch c.Type {
	case TypeStdout, TypeConsole:
	case TypeFile:
		if c.File == nil {
			ve.Add("file", "required for type file")
		} else if c.File.Path == "" {
			ve.Add("file.path", "required")
		}
	default:
		ve.Addf("type", "unknown value %q; valid: stdout console file", c.Type)
	}

	return ve.Err()
}
