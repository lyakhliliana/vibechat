package cache

import (
	"time"

	"vibechat/internal/infrastructure/cache/redis"
	"vibechat/utils/config"
)

type Type string

const (
	TypeRedis Type = "redis"
	TypeMock  Type = "mock"
)

var DefaultConfig = Config{
	Type: TypeMock,
	TTL: TTLConfig{
		User:   config.Duration{Duration: 5 * time.Minute},
		Chat:   config.Duration{Duration: 10 * time.Minute},
		Member: config.Duration{Duration: 2 * time.Minute},
	},
}

// Config selects the cache backend. When Type is "redis", Redis must be non-nil.
// When Type is "mock" or empty, an in-memory store is used (dev / tests).
type Config struct {
	Type  Type          `json:"type"  yaml:"type"`
	Redis *redis.Config `json:"redis" yaml:"redis"`
	TTL   TTLConfig     `json:"ttl"   yaml:"ttl"`
}

// TTLConfig controls how long each entity type lives in cache.
type TTLConfig struct {
	User   config.Duration `json:"user"   yaml:"user"`
	Chat   config.Duration `json:"chat"   yaml:"chat"`
	Member config.Duration `json:"member" yaml:"member"`
}

func (c *Config) SetDefaults() {
	if c.Type == "" {
		c.Type = DefaultConfig.Type
	}
	if c.Redis != nil && c.Redis.Addr == "" {
		c.Redis.Addr = "localhost:6379"
	}
	if c.TTL.User.Duration == 0 {
		c.TTL.User = DefaultConfig.TTL.User
	}
	if c.TTL.Chat.Duration == 0 {
		c.TTL.Chat = DefaultConfig.TTL.Chat
	}
	if c.TTL.Member.Duration == 0 {
		c.TTL.Member = DefaultConfig.TTL.Member
	}
}

func (c *Config) Validate() error {
	ve := &config.ValidationError{}

	switch c.Type {
	case TypeRedis, TypeMock:
	default:
		ve.Addf("type", "must be %q or %q, got %q", TypeRedis, TypeMock, c.Type)
	}

	if c.Type == TypeRedis && c.Redis == nil {
		ve.Add("redis", "required when type is \"redis\"")
	}
	if c.Type == TypeRedis && c.Redis != nil && c.Redis.Addr == "" {
		ve.Add("redis.addr", "required")
	}

	return ve.Err()
}
