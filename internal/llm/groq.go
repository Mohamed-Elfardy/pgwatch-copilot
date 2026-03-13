package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const groqURL = "https://api.groq.com/openai/v1/chat/completions"

type GroqProvider struct {
	apiKey string
}

func NewGroqProvider(apiKey string) *GroqProvider {
	return &GroqProvider{apiKey: apiKey}
}

func (g *GroqProvider) Name() string {
	return "groq"
}

func (g *GroqProvider) Complete(ctx context.Context, system string, messages []Message) (*Response, error) {
	var chatMessages []map[string]string

	if system != "" {
		chatMessages = append(chatMessages, map[string]string{
			"role":    "system",
			"content": system,
		})
	}

	for _, m := range messages {
		chatMessages = append(chatMessages, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model":       "llama-3.3-70b-versatile",
		"messages":    chatMessages,
		"temperature": 0.2,
		"max_tokens":  1024,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", groqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("groq request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse groq response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("groq error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response from groq")
	}

	return &Response{Content: strings.TrimSpace(result.Choices[0].Message.Content)}, nil
}
