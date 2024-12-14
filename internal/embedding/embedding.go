package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

var (
	BaseURL = "http://localhost:11434/api/embeddings"
	Model   = "nomic-embed-text"
)

type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"prompt"`
}

func CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Create a new HTTP request
	data := EmbeddingRequest{
		Model: Model,
		Input: text,
	}

	// Marshal the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set the content type to JSON
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Decode the response
	var embeddingResp struct {
		Embedding []float64 `json:"embedding"`
	}
	err = json.NewDecoder(resp.Body).Decode(&embeddingResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	// Convert the embedding to float32
	float32Embedding := make([]float32, len(embeddingResp.Embedding))
	for i, f := range embeddingResp.Embedding {
		float32Embedding[i] = float32(f)
	}

	return float32Embedding, nil
}
