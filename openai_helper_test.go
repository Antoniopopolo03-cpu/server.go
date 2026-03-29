package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIChat_MissingKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	_, err := openAIChat("system", "user")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected OPENAI_API_KEY error, got %v", err)
	}
}

func TestOpenAIChat_Success(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("expected Bearer auth header")
		}

		var req openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if len(req.Messages) != 2 {
			t.Errorf("expected 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("expected system role, got %s", req.Messages[0].Role)
		}

		resp := openAIChatResponse{
			Choices: []struct {
				Message openAIChatMessage `json:"message"`
			}{
				{Message: openAIChatMessage{Role: "assistant", Content: "mock reply"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mock.Close()

	// Patch the OpenAI URL by using a custom transport
	// Since openAIChat uses a hardcoded URL, we need to intercept at transport level
	origTransport := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{target: mock.URL, transport: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "test-model")

	reply, err := openAIChat("you are a bot", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "mock reply" {
		t.Fatalf("expected 'mock reply', got %q", reply)
	}
}

func TestOpenAIChat_APIError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer mock.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{target: mock.URL, transport: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	t.Setenv("OPENAI_API_KEY", "test-key")

	_, err := openAIChat("system", "user")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "openai error") {
		t.Fatalf("expected openai error, got %v", err)
	}
}

func TestOpenAIChat_EmptyChoices(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIChatResponse{Choices: nil}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mock.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{target: mock.URL, transport: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	t.Setenv("OPENAI_API_KEY", "test-key")

	_, err := openAIChat("system", "user")
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestOpenAIChat_DefaultModel(t *testing.T) {
	var capturedModel string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		capturedModel = req.Model

		resp := openAIChatResponse{
			Choices: []struct {
				Message openAIChatMessage `json:"message"`
			}{
				{Message: openAIChatMessage{Content: "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mock.Close()

	origTransport := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{target: mock.URL, transport: origTransport}
	defer func() { http.DefaultTransport = origTransport }()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "")

	openAIChat("sys", "usr")
	if capturedModel != "gpt-4o-mini" {
		t.Fatalf("expected default model 'gpt-4o-mini', got %q", capturedModel)
	}
}

// redirectTransport intercepts all HTTP requests and sends them to a local test server.
type redirectTransport struct {
	target    string
	transport http.RoundTripper
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(rt.target, "http://")
	return rt.transport.RoundTrip(req)
}
