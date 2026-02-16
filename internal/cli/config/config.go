package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBaseURL        = "http://127.0.0.1:8080"
	DefaultTimeout        = 10 * time.Second
	DefaultTokenStatePath = "configs/cli_state.json"
)

// Config holds CLI configuration.
type Config struct {
	BaseURL        string        `yaml:"baseURL"`
	Timeout        time.Duration `yaml:"timeout"`
	TokenStatePath string        `yaml:"tokenStatePath"`
	PrettyJSON     *bool         `yaml:"prettyJSON"`
}

func Load(path string) (Config, error) {
	cfg := Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config file failed: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config file failed: %w", err)
	}
	applyDefaults(&cfg)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.TokenStatePath == "" {
		cfg.TokenStatePath = DefaultTokenStatePath
	}
	if cfg.PrettyJSON == nil {
		value := true
		cfg.PrettyJSON = &value
	}
}
