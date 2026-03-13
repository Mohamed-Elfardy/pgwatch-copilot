package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const geminiURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"

type GeminiProvider struct {
	apiKey string
}

func NewGeminiProvider(apiKey string) *GeminiProvider {
	return &GeminiProvider{apiKey: apiKey}
}

func (g *GeminiProvider) Name() string {
	return "gemini"
}

func (g *GeminiProvider) Complete(ctx context.Context, system string, messages []Message) (*Response, error) {
	// Build contents array from messages
	var contents []map[string]interface{}

	// Add system message as first user message (Gemini doesn't have system role)
	if system != "" {
		contents = append(contents, map[string]interface{}{
			"role":  "user",
			"parts": []map[string]string{{"text": "[SYSTEM INSTRUCTIONS]\n" + system}},
		})
		contents = append(contents, map[string]interface{}{
			"role":  "model",
			"parts": []map[string]string{{"text": "Understood. I will follow these instructions."}},
		})
	}

	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": []map[string]string{{"text": m.Content}},
		})
	}

	body, _ := json.Marshal(map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     0.2,
			"maxOutputTokens": 1024,
		},
	})

	url := fmt.Sprintf("%s?key=%s", geminiURL, g.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse gemini response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("gemini error: %s", result.Error.Message)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from gemini")
	}

	return &Response{Content: result.Candidates[0].Content.Parts[0].Text}, nil
}
