package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// curl http://127.0.0.1:11435/v1/rerank \                                                                                                                 lit src/refer @DarkBlue
// -H "Content-Type: application/json" \
//
//	-d '{
//		"model": "whatever",
//			"query": "What is Corona disease?",
//			"top_n": 3,
//			"documents": [
//				"Corona is a Mexican brand of beer produced by Grupo Modelo in Mexico and exported to markets around the world.",
//			"it is a bear",
//			"COVID-19 is a contagious illness caused by the a virus SARS-CoV-2."
//			]
//	}'
//
// {
//   "model": "whatever",
//   "object": "list",
//   "usage": {
//     "prompt_tokens": 76,
//     "total_tokens": 76
//   },
//   "results": [
//     {"index": 0, "relevance_score": -4.017609119415283},
//     {"index": 1, "relevance_score": -11.028653144836426},
//     {"index": 2, "relevance_score": -0.7565364241600037}
//   ]
// }

type Response struct {
	Model  string `json:"model"`
	Object string `json:"object"`
	Usage  struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

func RerankDocuments(query string, documents []Document, top int) ([]Document, error) {
	requestDocuments := []string{}
	for _, doc := range documents {
		requestDocuments = append(requestDocuments, doc.Content)
	}

	// Create a new HTTP request
	data := map[string]any{
		"model":     "",
		"query":     query,
		"top_n":     top,
		"documents": requestDocuments,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %v", err)
	}

	fmt.Println(string(jsonData))

	resp, err := http.Post(RerankerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %v", resp.Status)
	}

	var rerankResp Response
	err = json.NewDecoder(resp.Body).Decode(&rerankResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	rerankedDocuments := []Document{}
	for _, result := range rerankResp.Results {
		rerankedDocuments = append(rerankedDocuments, documents[result.Index])
	}

	return rerankedDocuments, nil
}
