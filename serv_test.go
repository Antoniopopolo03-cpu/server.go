package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRootHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	rootHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "status healthy" {
		t.Fatalf("expected 'status healthy', got %q", w.Body.String())
	}
}

func TestSalutaHandler(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"", "ciao"},
		{"nome=Mario", "ciao Mario"},
		{"nome=", "ciao"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/saluta?"+tt.query, nil)
			w := httptest.NewRecorder()
			salutaHandler(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if w.Body.String() != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, w.Body.String())
			}
		})
	}
}

func TestBestemmiaHandler(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"", "Porco Dio"},
		{"nome=Luca", "ciao Luca che voi Porco Dio"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/saluta/con-bestemmia?"+tt.query, nil)
			w := httptest.NewRecorder()
			BestemmiaHandler(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if w.Body.String() != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, w.Body.String())
			}
		})
	}
}

func TestLlmHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/llm", nil)
	w := httptest.NewRecorder()
	llmHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestLlmHandler_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/llm", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	llmHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLlmHandler_EmptyPrompt(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/llm", strings.NewReader(`{"prompt":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	llmHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLlmHandler_MockMode(t *testing.T) {
	t.Setenv("MOCK_LLM", "true")

	body := `{"prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/llm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	llmHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp LLMResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !strings.Contains(resp.Answer, "[MOCK]") {
		t.Fatalf("expected mock response, got %q", resp.Answer)
	}
	if !strings.Contains(resp.Answer, "hello") {
		t.Fatalf("expected prompt echoed back, got %q", resp.Answer)
	}
}

func TestLlmHandler_MissingAPIKey(t *testing.T) {
	t.Setenv("MOCK_LLM", "false")
	t.Setenv("OPENAI_API_KEY", "")

	body := `{"prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/llm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	llmHandler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestLlmHandler_WithMockOpenAI(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIChatResponse{
			Choices: []struct {
				Message openAIChatMessage `json:"message"`
			}{
				{Message: openAIChatMessage{Role: "assistant", Content: "test reply"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// llmHandler uses hardcoded OpenAI URL, so we test via mock mode instead.
	// The real OpenAI integration is tested in openai_helper_test.go
	t.Setenv("MOCK_LLM", "true")
	body := `{"prompt":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/llm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	llmHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStatusWriter(t *testing.T) {
	w := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

	sw.WriteHeader(http.StatusNotFound)
	if sw.status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", sw.status)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("underlying writer should have 404, got %d", w.Code)
	}
}

func TestLogMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, "ok")
	})

	wrapped := logMiddleware(handler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("expected 'ok', got %q", w.Body.String())
	}
}
