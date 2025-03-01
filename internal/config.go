package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	EmbeddingBaseURL string `json:"embedding_base_url"`
	EmbeddingModel   string `json:"embedding_model"`
	APIKey           string `json:"api_key,omitempty"`
	RerankerURL      string `json:"reranker_url,omitempty"`
}

func LoadConfig() (*Config, error) {
	// Default config
	cfg := &Config{
		EmbeddingBaseURL: "http://localhost:11434/api/embeddings",
		EmbeddingModel:   "nomic-embed-text",
		RerankerURL:      "http://localhost:11435/v1/rerank",
	}

	// Get config file path
	configDir, err := os.UserConfigDir()
	if err != nil {
		return cfg, nil // Return defaults if can't get config dir
	}

	// Define possible config paths
	configPaths := []string{
		filepath.Join(configDir, "refer", "config.json"),
		filepath.Join(os.Getenv("HOME"), ".config", "refer", "config.json"), // Additional path for macOS
	}

	for _, configPath := range configPaths {
		// Check if config file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue // Try next path if this file doesn't exist
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

		break // Configuration loaded successfully
	}

	// Update global variables
	BaseURL = cfg.EmbeddingBaseURL
	Model = cfg.EmbeddingModel
	APIKey = cfg.APIKey
	RerankerURL = cfg.RerankerURL

	return cfg, nil
}
