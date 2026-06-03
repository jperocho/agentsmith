// Package config reads ~/.agentsmith/config.json: global settings such as the
// default install mode (plan section 3). A missing file yields defaults.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Config holds user-wide settings.
type Config struct {
	// DefaultMode is the install mode used when --mode is not given.
	// "" means auto: symlink where supported, else copy.
	DefaultMode string `json:"defaultMode,omitempty"`
}

// Load reads config at path; a missing file returns zero-value defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	switch c.DefaultMode {
	case "", "copy", "symlink":
	default:
		return nil, fmt.Errorf("invalid defaultMode %q in config (use copy or symlink)", c.DefaultMode)
	}
	return &c, nil
}
