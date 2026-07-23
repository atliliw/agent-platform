package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEmbedBatchUsesConfiguredModel locks the fix where EmbedBatch hardcoded
// "text-embedding-3-small". The request must now carry the configured EmbeddingModel.
func TestEmbedBatchUsesConfiguredModel(t *testing.T) {
	// Arrange
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)
		if m, ok := req["model"].(string); ok {
			gotModel = m
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": []float64{0.1, 0.2, 0.3}, "index": 0},
			},
		})
	}))
	defer srv.Close()

	client := NewOpenAIClient(Config{
		BaseURL:        srv.URL,
		EmbeddingModel: "my-custom-embed-model",
	})

	// Act
	results, err := client.EmbedBatch(context.Background(), []string{"hello"})

	// Assert
	if err != nil {
		t.Fatalf("EmbedBatch returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 embedding, got %d", len(results))
	}
	if gotModel != "my-custom-embed-model" {
		t.Errorf("request model = %q, want %q (config EmbeddingModel must be used)", gotModel, "my-custom-embed-model")
	}
}

// TestEmbedBatchDefaultsToSmallModel locks the fallback: when EmbeddingModel is
// empty, the request falls back to "text-embedding-3-small".
func TestEmbedBatchDefaultsToSmallModel(t *testing.T) {
	// Arrange
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		_ = json.Unmarshal(body, &req)
		if m, ok := req["model"].(string); ok {
			gotModel = m
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": []float64{0.1}, "index": 0},
			},
		})
	}))
	defer srv.Close()

	client := NewOpenAIClient(Config{BaseURL: srv.URL}) // EmbeddingModel intentionally empty

	// Act
	_, err := client.EmbedBatch(context.Background(), []string{"hello"})

	// Assert
	if err != nil {
		t.Fatalf("EmbedBatch returned error: %v", err)
	}
	if gotModel != "text-embedding-3-small" {
		t.Errorf("request model = %q, want fallback %q", gotModel, "text-embedding-3-small")
	}
}
