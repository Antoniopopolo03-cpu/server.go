package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const dattebayoBaseURL = "https://dattebayo-api.onrender.com"

// stringSliceFlexible decodifica JSON sia come array di stringhe sia come singola stringa.
type stringSliceFlexible []string

func (s *stringSliceFlexible) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var one string
		if err := json.Unmarshal(data, &one); err != nil {
			return err
		}
		*s = []string{one}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	*s = arr
	return nil
}

type dattebayoCharactersResponse struct {
	Characters []dattebayoCharacter `json:"characters"`
}

type dattebayoCharacter struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Images   []string `json:"images"`
	Jutsu    []string `json:"jutsu"`
	Personal struct {
		Clan           string              `json:"clan"`
		Affiliation    stringSliceFlexible `json:"affiliation"`
		Sex            string              `json:"sex"`
		Birthdate      string              `json:"birthdate"`
		Classification stringSliceFlexible `json:"classification"`
	} `json:"personal"`
	Rank struct {
		NinjaRank struct {
			PartI  string `json:"Part I"`
			PartII string `json:"Part II"`
			Gaiden string `json:"Gaiden"`
		} `json:"ninjaRank"`
	} `json:"rank"`
	Debut struct {
		Anime string `json:"anime"`
		Manga string `json:"manga"`
	} `json:"debut"`
}

type dattebayoClansResponse struct {
	Clans []dattebayoClan `json:"clans"`
}

type dattebayoClan struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Characters []int  `json:"characters"`
}

type DattebayoProvider struct {
	client  *http.Client
	baseURL string
}

func NewDattebayoProvider() *DattebayoProvider {
	return &DattebayoProvider{
		client:  &http.Client{Timeout: 12 * time.Second},
		baseURL: dattebayoBaseURL,
	}
}

func (p *DattebayoProvider) Name() string {
	return "dattebayo"
}

func (p *DattebayoProvider) SearchCharacters(ctx context.Context, req SearchRequest) ([]CanonicalCharacter, error) {
	raw, err := p.get(ctx, "characters", req.Query, req.Limit)
	if err != nil {
		return nil, err
	}
	var parsed dattebayoCharactersResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse characters json: %w", err)
	}
	out := make([]CanonicalCharacter, 0, len(parsed.Characters))
	for _, c := range parsed.Characters {
		item := CanonicalCharacter{
			ID:             fmt.Sprintf("%d", c.ID),
			Name:           c.Name,
			Clan:           c.Personal.Clan,
			Affiliation:    []string(c.Personal.Affiliation),
			Sex:            c.Personal.Sex,
			Birthdate:      c.Personal.Birthdate,
			Classification: []string(c.Personal.Classification),
			RankPartI:      c.Rank.NinjaRank.PartI,
			RankPartII:     c.Rank.NinjaRank.PartII,
			RankGaiden:     c.Rank.NinjaRank.Gaiden,
			DebutAnime:     c.Debut.Anime,
			DebutManga:     c.Debut.Manga,
			Jutsu:          c.Jutsu,
			Source:         p.Name(),
		}
		if len(c.Images) > 0 {
			item.ImageURL = c.Images[0]
		}
		out = append(out, item)
	}
	return out, nil
}

func (p *DattebayoProvider) SearchClans(ctx context.Context, req SearchRequest) ([]CanonicalClan, error) {
	raw, err := p.get(ctx, "clans", req.Query, req.Limit)
	if err != nil {
		return nil, err
	}
	var parsed dattebayoClansResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse clans json: %w", err)
	}
	out := make([]CanonicalClan, 0, len(parsed.Clans))
	for _, c := range parsed.Clans {
		out = append(out, CanonicalClan{
			ID:             fmt.Sprintf("%d", c.ID),
			Name:           c.Name,
			CharacterCount: len(c.Characters),
			Source:         p.Name(),
		})
	}
	return out, nil
}

func (p *DattebayoProvider) get(ctx context.Context, collection, name string, limit int) ([]byte, error) {
	if limit <= 0 {
		limit = 5
	}
	u, err := url.Parse(p.baseURL + "/" + collection)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("page", "1")
	q.Set("limit", fmt.Sprintf("%d", limit))
	if strings.TrimSpace(name) != "" {
		q.Set("name", name)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
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
