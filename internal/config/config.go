package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	EmbeddingBaseURL string `json:"embedding_base_url"`
}

func LoadConfig() (*Config, error) {
	// Default config
	cfg := &Config{
		EmbeddingBaseURL: "http://localhost:11434",
	}

	// Get config file path
	configDir, err := os.UserConfigDir()
	if err != nil {
		return cfg, nil // Return defaults if can't get config dir
	}

	configPath := filepath.Join(configDir, "lit", "config.json")
	
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return cfg, nil // Return defaults if no config file
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, nil // Return defaults if can't read file
	}

	// Parse config
	if err := json.Unmarshal(data, cfg); err != nil {
		return cfg, nil // Return defaults if can't parse
	}

	return cfg, nil
}
