package config

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration to support human-readable string notation
// ("15m", "168h", "30s") in both JSON and YAML configuration files.
type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		dur, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("config: parse duration %q: %w", s, err)
		}
		d.Duration = dur
		return nil
	}

	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("config: duration must be a string (\"15m\") or integer nanoseconds")
	}
	d.Duration = time.Duration(n)
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return d.Duration.String(), nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err == nil {
		d.Duration = dur
		return nil
	}

	var n int64
	if err = value.Decode(&n); err == nil {
		d.Duration = time.Duration(n)
		return nil
	}

	return fmt.Errorf("config: parse duration %q: expected format like \"15m\", \"168h\", \"30s\"", value.Value)
}
