package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type NarutoDBProvider struct {
	client       *http.Client
	baseURL      string
	maxRetries   int
	retryBackoff time.Duration
}

func NewNarutoDBProvider() *NarutoDBProvider {
	return &NarutoDBProvider{
		client:       &http.Client{Timeout: 12 * time.Second},
		baseURL:      "https://narutodb.xyz",
		maxRetries:   2,
		retryBackoff: 350 * time.Millisecond,
	}
}

func (p *NarutoDBProvider) Name() string {
	return "narutodb"
}

func (p *NarutoDBProvider) SearchCharacters(ctx context.Context, req SearchRequest) ([]CanonicalCharacter, error) {
	raw, err := p.get(ctx, "/api/character", req.Query, req.Limit)
	if err != nil {
		return nil, err
	}

	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	items := extractObjectArray(root)
	out := make([]CanonicalCharacter, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		c := CanonicalCharacter{
			ID:         anyToString(it["id"]),
			Name:       anyToString(it["name"]),
			Jutsu:      anyToStringSlice(it["jutsu"]),
			Source:     p.Name(),
			DebutAnime: anyToStringFromNested(it["debut"], "anime"),
			DebutManga: anyToStringFromNested(it["debut"], "manga"),
			ImageURL:   firstString(anyToStringSlice(it["images"])),
		}
		if personal, ok := it["personal"].(map[string]any); ok {
			c.Clan = anyToString(personal["clan"])
			c.Affiliation = anyToStringSlice(personal["affiliation"])
			c.Sex = anyToString(personal["sex"])
			c.Birthdate = anyToString(personal["birthdate"])
			c.Classification = anyToStringSlice(personal["classification"])
		}
		out = append(out, c)
	}
	return out, nil
}

func (p *NarutoDBProvider) SearchClans(ctx context.Context, req SearchRequest) ([]CanonicalClan, error) {
	// Endpoint clan non standardizzato in tutte le versioni, quindi in fase 1-2
	// lasciamo la ricerca clan a Dattebayo e ritorniamo vuoto qui.
	return nil, nil
}

func (p *NarutoDBProvider) get(ctx context.Context, path, name string, limit int) ([]byte, error) {
	if limit <= 0 {
		limit = 5
	}
	u, err := url.Parse(strings.TrimRight(p.baseURL, "/") + path)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("page", "1")
	q.Set("limit", strconv.Itoa(limit))
	if strings.TrimSpace(name) != "" {
		q.Set("name", name)
	}
	u.RawQuery = q.Encode()

	var lastErr error
	attempts := p.maxRetries + 1
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		// Riduce i casi in cui il provider risponde con challenge HTML invece che JSON.
		req.Header.Set("User-Agent", "server.go-naruto-bot/1.0 (+https://github.com/Antoniopopolo03-cpu/server.go)")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 300 {
				lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			} else if isHTMLResponse(resp.Header.Get("Content-Type"), body) {
				lastErr = fmt.Errorf("anti-bot html challenge detected")
			} else {
				return body, nil
			}
		}
		if attempt < attempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(p.retryBackoff * time.Duration(attempt)):
			}
		}
	}
	return nil, fmt.Errorf("narutodb request failed after retries: %w", lastErr)
}

func extractObjectArray(root any) []map[string]any {
	switch v := root.(type) {
	case []any:
		return toMapArray(v)
	case map[string]any:
		for _, key := range []string{"characters", "data", "items", "results"} {
			if arr, ok := v[key].([]any); ok {
				return toMapArray(arr)
			}
		}
	}
	return nil
}

func toMapArray(arr []any) []map[string]any {
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case int:
		return strconv.Itoa(t)
	default:
		return ""
	}
}

func anyToStringSlice(v any) []string {
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, it := range t {
			if s := anyToString(it); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		return []string{t}
	default:
		return nil
	}
}

func anyToStringFromNested(v any, key string) string {
	if m, ok := v.(map[string]any); ok {
		return anyToString(m[key])
	}
	return ""
}

func firstString(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

func isHTMLResponse(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") {
		return true
	}
	trimmed := strings.ToLower(strings.TrimSpace(string(body)))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}
