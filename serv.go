// @title           API Server Go
// @version         1.0
// @description     Documentazione API con Swagger
// @host            localhost:3000
// @BasePath        /
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	_ "server/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

type LLMRequest struct {
	Prompt string `json:"prompt"`
}

type LLMResponse struct {
	Answer string `json:"answer"`
}

type openAIChatRequest struct {
	Model    string              `json:"model"`
	Messages []openAIChatMessage `json:"messages"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIChatMessage `json:"message"`
	} `json:"choices"`
}

// handler per 2.1 root endpoint
// @Summary      Health check
// @Description  Risponde "status healthy" sulla root
// @Tags         system
// @Produce      plain
// @Success      200  {string}  string  "status healthy"
// @Router       / [get]
func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "status healthy")
}

// handler per 2.2 saluta il server
// @Summary      Saluta il server
// @Description  Risponde "ciao" o "ciao {nome}"
// @Tags         system
// @Produce      plain
// @Success      200  {string}  string  "ciao"  "ciao {nome}"
// @Router       /saluta [get]
func salutaHandler(w http.ResponseWriter, r *http.Request) {
	nome := r.URL.Query().Get("nome")
	if nome == "" {
		fmt.Fprintf(w, "ciao")
	} else {
		fmt.Fprintf(w, "ciao %s", nome)
	}
}

// @Summary      Ask LLM
// @Description  Invia un prompt all'LLM e ritorna la risposta
// @Tags         llm
// @Accept       json
// @Produce      json
// @Param        body  body  LLMRequest  true  "Prompt body"
// @Success      200   {object}  LLMResponse
// @Router       /llm [post]
func llmHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var reqBody LLMRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		slog.Warn("llm: invalid json body", "error", err)
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if reqBody.Prompt == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		return
	}

	slog.Info("llm: request", "prompt_len", len(reqBody.Prompt))

	if os.Getenv("MOCK_LLM") == "true" {
		slog.Info("llm: mock mode")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LLMResponse{Answer: "[MOCK] Ho ricevuto il prompt: " + reqBody.Prompt})
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		slog.Error("llm: missing OPENAI_API_KEY")
		http.Error(w, "missing OPENAI_API_KEY", http.StatusInternalServerError)
		return
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	payload := openAIChatRequest{
		Model: model,
		Messages: []openAIChatMessage{
			{Role: "user", Content: reqBody.Prompt},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("llm: marshal payload", "error", err)
		http.Error(w, "failed to build llm request", http.StatusInternalServerError)
		return
	}

	start := time.Now()
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(payloadBytes))
	if err != nil {
		slog.Error("llm: create request", "error", err)
		http.Error(w, "failed to create llm request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("llm: openai call failed", "error", err, "duration", time.Since(start))
		http.Error(w, "failed calling llm api", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	duration := time.Since(start)

	if resp.StatusCode >= 300 {
		slog.Error("llm: openai error", "status", resp.StatusCode, "body", string(respBody), "duration", duration)
		http.Error(w, "llm api error: "+string(respBody), http.StatusBadGateway)
		return
	}

	var llmResp openAIChatResponse
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		slog.Error("llm: parse response", "error", err)
		http.Error(w, "failed parsing llm response", http.StatusInternalServerError)
		return
	}

	if len(llmResp.Choices) == 0 {
		slog.Warn("llm: empty choices from openai")
		http.Error(w, "empty llm response", http.StatusBadGateway)
		return
	}

	slog.Info("llm: success", "model", model, "answer_len", len(llmResp.Choices[0].Message.Content), "duration", duration)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LLMResponse{Answer: llmResp.Choices[0].Message.Content})
}

// BestemmiaHandler handler per una 2.3 bestemmia endpoint
// @Summary      Bestemmia il server
// @Description  Risponde "Porco Dio" o "ciao {nome} che voi Porco Dio"
// @Tags         system
// @Produce      plain
// @Success      200  {string}  string  "Porco Dio"  "ciao {nome} che voi Porco Dio"
// @Router       /saluta/con-bestemmia [get]
func BestemmiaHandler(w http.ResponseWriter, r *http.Request) {
	nome := r.URL.Query().Get("nome")
	if nome == "" {
		fmt.Fprintf(w, "Porco Dio")
	} else {
		fmt.Fprintf(w, "ciao %s che voi Porco Dio", nome)
	}
}
func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration", time.Since(start).Round(time.Millisecond),
			"remote", r.RemoteAddr,
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func main() {
	_ = godotenv.Load()

	mux := http.NewServeMux()

	mux.HandleFunc("/swagger/*", httpSwagger.WrapHandler)
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/saluta/con-bestemmia", BestemmiaHandler)
	mux.HandleFunc("/saluta", salutaHandler)
	mux.HandleFunc("/llm", llmHandler)
	mux.HandleFunc("/naruto/chat", narutoChatHandler)
	mux.HandleFunc("/ws/chat", chatWebSocketHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	slog.Info("server starting", "port", port, "mock_llm", os.Getenv("MOCK_LLM") == "true")
	log.Fatal(http.ListenAndServe(":"+port, logMiddleware(mux)))
}
