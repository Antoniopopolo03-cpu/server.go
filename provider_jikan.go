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

type JikanProvider struct {
	client  *http.Client
	baseURL string
}

func NewJikanProvider() *JikanProvider {
	return &JikanProvider{
		client:  &http.Client{Timeout: 12 * time.Second},
		baseURL: "https://api.jikan.moe/v4",
	}
}

func (p *JikanProvider) Name() string {
	return "jikan"
}

func (p *JikanProvider) SearchCharacters(ctx context.Context, req SearchRequest) ([]CanonicalCharacter, error) {
	raw, err := p.getCharacters(ctx, req.Query, req.Limit)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Data []struct {
			MalID  int    `json:"mal_id"`
			Name   string `json:"name"`
			About  string `json:"about"`
			Images struct {
				JPG struct {
					ImageURL string `json:"image_url"`
				} `json:"jpg"`
			} `json:"images"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse jikan json: %w", err)
	}

	out := make([]CanonicalCharacter, 0, len(parsed.Data))
	for _, ch := range parsed.Data {
		if strings.TrimSpace(ch.Name) == "" {
			continue
		}
		item := CanonicalCharacter{
			ID:       strconv.Itoa(ch.MalID),
			Name:     ch.Name,
			ImageURL: ch.Images.JPG.ImageURL,
			Source:   p.Name(),
		}
		about := strings.TrimSpace(ch.About)
		if about != "" {
			// Campo di supporto per dare più contesto testuale senza inventare dati strutturati.
			item.Classification = []string{"bio_from_jikan"}
		}
		out = append(out, item)
	}
	return out, nil
}

func (p *JikanProvider) SearchClans(ctx context.Context, req SearchRequest) ([]CanonicalClan, error) {
	// Jikan non espone clan Naruto come risorsa diretta.
	return nil, nil
}

func (p *JikanProvider) getCharacters(ctx context.Context, query string, limit int) ([]byte, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 25 {
		limit = 25
	}
	u, err := url.Parse(strings.TrimRight(p.baseURL, "/") + "/characters")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("page", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
