package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// openAIChat chiama OpenAI e restituisce solo il testo dell'assistente.
// Usa openAIChatRequest, openAIChatMessage, openAIChatResponse definiti in serv.go
func openAIChat(systemPrompt, userPrompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("missing OPENAI_API_KEY")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	slog.Info("openai: calling", "model", model, "system_len", len(systemPrompt), "user_len", len(userPrompt))

	body := openAIChatRequest{
		Model: model,
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	start := time.Now()
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("openai: request failed", "error", err, "duration", time.Since(start))
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	duration := time.Since(start)

	if resp.StatusCode >= 300 {
		slog.Error("openai: api error", "status", resp.StatusCode, "body", string(raw), "duration", duration)
		return "", fmt.Errorf("openai error: %s", string(raw))
	}

	var out openAIChatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		slog.Warn("openai: empty choices")
		return "", fmt.Errorf("empty openai choices")
	}

	slog.Info("openai: success", "model", model, "answer_len", len(out.Choices[0].Message.Content), "duration", duration)
	return out.Choices[0].Message.Content, nil
}
