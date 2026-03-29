package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// In locale accetta qualsiasi origin; in produzione restringi (es. solo il tuo dominio).
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsOutbound messaggio JSON verso il client (stesso formato concettuale di NarutoChatResponse + errori).
type wsOutbound struct {
	Reply string `json:"reply,omitempty"`
	Error string `json:"error,omitempty"`
	God   bool   `json:"god_mode,omitempty"`
}

// chatWebSocketHandler mantiene una sessione WS.
// Ogni messaggio può essere:
//   - testo libero: es. "personaggio Naruto" o "Naruto quanti anni ha"
//   - oppure JSON: {"message":"..."} (ancora supportato)
// La risposta è sempre JSON: {"reply":"..."} oppure {"error":"..."}.
//
// God mode (OpenAI senza Dattebayo): connetti con query ?god=1 oppure ?god=true
// Esempio: ws://localhost:3000/ws/chat?god=1
func chatWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	godParam := r.URL.Query().Get("god")
	godMode := godParam == "1" || strings.EqualFold(godParam, "true")

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws: upgrade failed", "error", err, "remote", r.RemoteAddr)
		return
	}
	defer conn.Close()

	slog.Info("ws: connected", "remote", r.RemoteAddr, "god_mode", godMode)

	readWait := 120 * time.Second
	_ = conn.SetReadDeadline(time.Now().Add(readWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(readWait))
		return nil
	})

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("ws: unexpected close", "error", err, "remote", r.RemoteAddr)
			} else {
				slog.Info("ws: disconnected", "remote", r.RemoteAddr)
			}
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(readWait))

		userText, ok := parseWebSocketUserText(payload)
		if !ok {
			sendWSError(conn, "messaggio vuoto: scrivi testo libero oppure {\"message\":\"...\"}")
			continue
		}

		slog.Info("ws: message received", "text_len", len(userText), "god_mode", godMode, "remote", r.RemoteAddr)
		start := time.Now()

		var reply string
		var runErr error
		if godMode {
			reply, runErr = runNarutoChatGod(userText)
		} else {
			reply, runErr = runNarutoChatPipeline(userText)
		}
		if runErr != nil {
			slog.Error("ws: pipeline error", "error", runErr, "duration", time.Since(start))
			sendWSError(conn, runErr.Error())
			continue
		}

		slog.Info("ws: reply sent", "reply_len", len(reply), "duration", time.Since(start))

		out := wsOutbound{Reply: reply, God: godMode}
		b, err := json.Marshal(out)
		if err != nil {
			sendWSError(conn, "failed to encode reply")
			continue
		}
		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
			break
		}
	}
}

// parseWebSocketUserText accetta testo puro o JSON {"message":"..."}.
func parseWebSocketUserText(payload []byte) (text string, ok bool) {
	s := strings.TrimSpace(string(payload))
	if s == "" {
		return "", false
	}
	// Formato JSON esplicito
	if strings.HasPrefix(s, "{") {
		var req NarutoChatRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			// JSON non valido: tratta tutta la riga come testo (retrocompat / edge case)
			return s, true
		}
		if strings.TrimSpace(req.Message) == "" {
			return "", false
		}
		return strings.TrimSpace(req.Message), true
	}
	// Testo libero (es. "personaggio naruto", "Naruto quanti anni ha")
	return s, true
}

func sendWSError(conn *websocket.Conn, msg string) {
	out := wsOutbound{Error: msg}
	b, _ := json.Marshal(out)
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_ = conn.WriteMessage(websocket.TextMessage, b)
}
