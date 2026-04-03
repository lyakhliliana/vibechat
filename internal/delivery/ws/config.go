package ws

type Config struct {
	// MaxMessageSize caps the size of a single incoming WebSocket frame in bytes.
	MaxMessageSize int64 `json:"max_message_size" yaml:"max_message_size"`
}

var DefaultConfig = Config{
	MaxMessageSize: 4096,
}

func (c *Config) SetDefaults() {
	if c.MaxMessageSize == 0 {
		c.MaxMessageSize = DefaultConfig.MaxMessageSize
	}
}
