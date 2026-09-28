package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

type OpenAICompatible struct {
	URL    string
	Model  string
	APIKey string
	Client *http.Client
}

type request struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type response struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func (o *OpenAICompatible) ID() string { return "openai-compatible:" + o.Model }

func (o *OpenAICompatible) Embed(texts []string) ([][]float64, error) {
	if o.URL == "" || o.Model == "" {
		return nil, fmt.Errorf("embedding URL and model are required")
	}
	const batchSize = 32
	out := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		chunk, err := o.embedChunk(texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
	}
	return out, nil
}

func (o *OpenAICompatible) embedChunk(texts []string) ([][]float64, error) {
	body, err := json.Marshal(request{Model: o.Model, Input: texts})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	client := o.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding endpoint returned %s", resp.Status)
	}
	var decoded response
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if len(decoded.Data) != len(texts) {
		return nil, fmt.Errorf("embedding endpoint returned %d vectors for %d inputs", len(decoded.Data), len(texts))
	}
	sort.Slice(decoded.Data, func(i, j int) bool { return decoded.Data[i].Index < decoded.Data[j].Index })
	vectors := make([][]float64, len(decoded.Data))
	for idx, item := range decoded.Data {
		if len(item.Embedding) == 0 {
			return nil, fmt.Errorf("embedding %d is empty", idx)
		}
		vectors[idx] = item.Embedding
	}
	return vectors, nil
}
