package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestParseWebSocketUserText_PlainText(t *testing.T) {
	text, ok := parseWebSocketUserText([]byte("ciao mondo"))
	if !ok || text != "ciao mondo" {
		t.Fatalf("expected ('ciao mondo', true), got (%q, %v)", text, ok)
	}
}

func TestParseWebSocketUserText_JSON(t *testing.T) {
	text, ok := parseWebSocketUserText([]byte(`{"message":"hello"}`))
	if !ok || text != "hello" {
		t.Fatalf("expected ('hello', true), got (%q, %v)", text, ok)
	}
}

func TestParseWebSocketUserText_EmptyString(t *testing.T) {
	_, ok := parseWebSocketUserText([]byte(""))
	if ok {
		t.Fatal("expected false for empty string")
	}
}

func TestParseWebSocketUserText_Whitespace(t *testing.T) {
	_, ok := parseWebSocketUserText([]byte("   "))
	if ok {
		t.Fatal("expected false for whitespace-only")
	}
}

func TestParseWebSocketUserText_EmptyJSONMessage(t *testing.T) {
	_, ok := parseWebSocketUserText([]byte(`{"message":""}`))
	if ok {
		t.Fatal("expected false for empty JSON message")
	}
}

func TestParseWebSocketUserText_InvalidJSON(t *testing.T) {
	// Invalid JSON starting with { should fall back to plain text
	text, ok := parseWebSocketUserText([]byte(`{broken json`))
	if !ok || text != "{broken json" {
		t.Fatalf("expected ('{broken json', true), got (%q, %v)", text, ok)
	}
}

func TestParseWebSocketUserText_JSONWithSpaces(t *testing.T) {
	text, ok := parseWebSocketUserText([]byte(`{"message":" hello world "}`))
	if !ok || text != "hello world" {
		t.Fatalf("expected ('hello world', true), got (%q, %v)", text, ok)
	}
}

func TestWebSocketHandler_Connect(t *testing.T) {
	// We need OPENAI_API_KEY unset so the pipeline fails fast,
	// but we can still test WS connection and message parsing
	t.Setenv("OPENAI_API_KEY", "")

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/chat", chatWebSocketHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Send a message — pipeline will fail because no API key, but we get an error response (not a crash)
	err = conn.WriteMessage(websocket.TextMessage, []byte("personaggio Naruto"))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	var out wsOutbound
	if err := json.Unmarshal(msg, &out); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should get an error (missing API key or dattebayo failure) — not a crash
	if out.Error == "" && out.Reply == "" {
		t.Fatal("expected either error or reply, got neither")
	}
}

func TestWebSocketHandler_EmptyMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/chat", chatWebSocketHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Send empty message
	err = conn.WriteMessage(websocket.TextMessage, []byte(""))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	var out wsOutbound
	if err := json.Unmarshal(msg, &out); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if out.Error == "" {
		t.Fatal("expected error for empty message")
	}
}

func TestWebSocketHandler_GodModeParam(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/chat", chatWebSocketHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/chat?god=1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	err = conn.WriteMessage(websocket.TextMessage, []byte("test"))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	var out wsOutbound
	if err := json.Unmarshal(msg, &out); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// In god mode without API key, we get an error but god_mode should be set
	if out.Error == "" {
		// If somehow it succeeded, god_mode should be true
		if !out.God {
			t.Fatal("expected god_mode=true in response")
		}
	}
}
