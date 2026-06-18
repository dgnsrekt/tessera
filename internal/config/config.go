// Package config loads and persists tessera's user settings at
// ~/.config/tessera/config.toml: the matrix endpoint, poll interval, and the
// custom input/output labels (the device protocol has no naming of its own).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the on-disk settings document.
type Config struct {
	Host         string   `toml:"host"`
	Port         int      `toml:"port"`
	PollInterval float64  `toml:"poll_interval"`
	Inputs       []string `toml:"inputs"`
	Outputs      []string `toml:"outputs"`
}

// Default returns the settings written on first run.
func Default() Config {
	return Config{
		Host:         "10.10.0.1",
		Port:         5000,
		PollInterval: 1.0,
		Inputs:       []string{"Input 1", "Input 2", "Input 3", "Input 4"},
		Outputs:      []string{"Output 1", "Output 2", "Output 3", "Output 4"},
	}
}

// Dir is the config directory, honoring XDG_CONFIG_HOME, else ~/.config.
func Dir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "tessera")
}

// Path is the full path to config.toml.
func Path() string { return filepath.Join(Dir(), "config.toml") }

// Load reads the config, writing defaults on first run. Unknown/missing fields
// fall back to their defaults.
func Load() (Config, error) {
	cfg := Default()
	path := Path()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, Save(cfg)
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	if len(cfg.Inputs) == 0 {
		cfg.Inputs = Default().Inputs
	}
	if len(cfg.Outputs) == 0 {
		cfg.Outputs = Default().Outputs
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = Default().PollInterval
	}
	return cfg, nil
}

// Save writes the config, creating the directory if needed.
func Save(cfg Config) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	f, err := os.Create(Path())
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// NumInputs is the configured input count.
func (c Config) NumInputs() int { return len(c.Inputs) }

// NumOutputs is the configured output count.
func (c Config) NumOutputs() int { return len(c.Outputs) }

// InputLabel returns the 1-based input label, or a generic fallback.
func (c Config) InputLabel(i int) string {
	if i >= 1 && i <= len(c.Inputs) {
		return c.Inputs[i-1]
	}
	return fmt.Sprintf("Input %d", i)
}

// OutputLabel returns the 1-based output label, or a generic fallback.
func (c Config) OutputLabel(i int) string {
	if i >= 1 && i <= len(c.Outputs) {
		return c.Outputs[i-1]
	}
	return fmt.Sprintf("Output %d", i)
}

// PollDuration is PollInterval as a time.Duration (minimum 250ms).
func (c Config) PollDuration() time.Duration {
	d := time.Duration(c.PollInterval * float64(time.Second))
	return max(d, 250*time.Millisecond)
}
