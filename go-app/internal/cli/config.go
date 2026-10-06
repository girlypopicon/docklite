package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const defaultHost = "http://localhost:3000"

type Config struct {
	CurrentServer string                  `json:"current_server"`
	Servers       map[string]ServerConfig `json:"servers"`
}

type ServerConfig struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

func LoadConfig() (*Config, string, error) {
	path, err := configPath()
	if err != nil {
		return nil, "", err
	}

	file, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := &Config{
				CurrentServer: "default",
				Servers: map[string]ServerConfig{
					"default": {
						Host:  defaultHost,
						Token: "",
					},
				},
			}
			return cfg, path, nil
		}
		return nil, "", err
	}

	var cfg Config
	if err := json.Unmarshal(file, &cfg); err != nil {
		return nil, "", err
	}
	if cfg.Servers == nil || len(cfg.Servers) == 0 {
		cfg.Servers = map[string]ServerConfig{
			"default": {
				Host:  defaultHost,
				Token: "",
			},
		}
		if cfg.CurrentServer == "" {
			cfg.CurrentServer = "default"
		}
	}
	return &cfg, path, nil
}

func SaveConfig(cfg *Config, path string) error {
	if cfg == nil {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}

func configPath() (string, error) {
	// DOCKLITE_CONFIG lets scripts and tests point at a different file.
	if override := os.Getenv("DOCKLITE_CONFIG"); override != "" {
		return override, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "docklite", "config.json"), nil
}

func DefaultHost() string {
	return defaultHost
}
