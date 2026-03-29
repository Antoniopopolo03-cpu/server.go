package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDetectCollection(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"personaggio Naruto", "characters"},
		{"clan Uchiha", "clans"},
		{"il clan Hyuga", "clans"},
		{"chi è Sasuke", "characters"},
		{"CLAN segreto", "clans"},
		{"", "characters"},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := detectCollection(tt.msg)
			if got != tt.want {
				t.Fatalf("detectCollection(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

func TestExtractSearchTerm(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"personaggio Naruto", "Naruto"},
		{"Personaggio naruto", "naruto"},
		{"il personaggio Sasuke", "Sasuke"},
		{"clan Uchiha", "Uchiha"},
		{"il clan Hyuga", "Hyuga"},
		{"chi è Itachi", "Itachi"},
		{"chi e Kakashi", "Kakashi"},
		{"Naruto", "Naruto"},
		{"  spazi  ", "spazi"},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := extractSearchTerm(tt.msg)
			if got != tt.want {
				t.Fatalf("extractSearchTerm(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

func TestShortFactualQuery(t *testing.T) {
	positive := []string{
		"colore capelli Naruto",
		"quanti anni ha Sasuke",
		"occhi di Kakashi",
		"altezza Naruto",
		"sesso di Hinata",
	}
	for _, msg := range positive {
		if !shortFactualQuery(msg) {
			t.Errorf("shortFactualQuery(%q) should be true", msg)
		}
	}

	negative := []string{
		"raccontami di Naruto",
		"chi è Sasuke",
		"parlami del team 7",
	}
	for _, msg := range negative {
		if shortFactualQuery(msg) {
			t.Errorf("shortFactualQuery(%q) should be false", msg)
		}
	}
}

func TestIncludeJutsuInDraft(t *testing.T) {
	positive := []string{
		"tecniche di Naruto",
		"jutsu di Sasuke",
		"rasengan",
		"clone ombra",
		"cosa sa fare Kakashi",
	}
	for _, msg := range positive {
		if !includeJutsuInDraft(msg) {
			t.Errorf("includeJutsuInDraft(%q) should be true", msg)
		}
	}

	negative := []string{
		"chi è Naruto",
		"clan Uchiha",
		"compleanno di Sakura",
	}
	for _, msg := range negative {
		if includeJutsuInDraft(msg) {
			t.Errorf("includeJutsuInDraft(%q) should be false", msg)
		}
	}
}

func TestDraftFromCharacters_Empty(t *testing.T) {
	got := draftFromCharacters(nil, "test")
	if !strings.Contains(got, "Nessun personaggio") {
		t.Fatalf("expected 'Nessun personaggio' message, got %q", got)
	}
}

func TestDraftFromCharacters_WithData(t *testing.T) {
	chars := []dattebayoCharacter{
		{
			ID:   1,
			Name: "Naruto Uzumaki",
			Personal: struct {
				Clan           string              `json:"clan"`
				Affiliation    stringSliceFlexible `json:"affiliation"`
				Sex            string              `json:"sex"`
				Birthdate      string              `json:"birthdate"`
				Classification stringSliceFlexible `json:"classification"`
			}{
				Clan:        "Uzumaki",
				Affiliation: []string{"Konoha"},
				Sex:         "Male",
				Birthdate:   "October 10",
			},
			Images: []string{"https://example.com/naruto.png"},
		},
	}

	got := draftFromCharacters(chars, "chi è Naruto")
	if !strings.Contains(got, "Naruto Uzumaki") {
		t.Fatalf("expected character name in draft, got %q", got)
	}
	if !strings.Contains(got, "Uzumaki") {
		t.Fatalf("expected clan in draft, got %q", got)
	}
	if !strings.Contains(got, "Male") {
		t.Fatalf("expected sex in draft, got %q", got)
	}
}

func TestDraftFromCharacters_WithJutsu(t *testing.T) {
	chars := []dattebayoCharacter{
		{
			ID:    1,
			Name:  "Naruto",
			Jutsu: []string{"Rasengan", "Shadow Clone", "Sage Mode"},
		},
	}

	got := draftFromCharacters(chars, "tecniche di Naruto")
	if !strings.Contains(got, "Rasengan") {
		t.Fatalf("expected jutsu in draft when query includes 'tecniche', got %q", got)
	}
}

func TestDraftFromCharacters_NoJutsuWhenNotAsked(t *testing.T) {
	chars := []dattebayoCharacter{
		{
			ID:    1,
			Name:  "Naruto",
			Jutsu: []string{"Rasengan"},
		},
	}

	got := draftFromCharacters(chars, "chi è Naruto")
	if strings.Contains(got, "Rasengan") {
		t.Fatalf("should NOT include jutsu when query doesn't ask for it, got %q", got)
	}
}

func TestDraftFromCharacters_MaxThree(t *testing.T) {
	chars := make([]dattebayoCharacter, 5)
	for i := range chars {
		chars[i] = dattebayoCharacter{ID: i, Name: "Char"}
	}

	got := draftFromCharacters(chars, "test")
	if !strings.Contains(got, "altri 2 risultati") {
		t.Fatalf("expected truncation message for 5 chars, got %q", got)
	}
}

func TestDraftFromClans_Empty(t *testing.T) {
	got := draftFromClans(nil)
	if !strings.Contains(got, "Nessun clan") {
		t.Fatalf("expected 'Nessun clan' message, got %q", got)
	}
}

func TestDraftFromClans_WithData(t *testing.T) {
	clans := []dattebayoClan{
		{ID: 1, Name: "Uchiha", Characters: make([]int, 10)},
		{ID: 2, Name: "Hyuga", Characters: make([]int, 5)},
	}

	got := draftFromClans(clans)
	if !strings.Contains(got, "Uchiha") || !strings.Contains(got, "Hyuga") {
		t.Fatalf("expected clan names in draft, got %q", got)
	}
}

func TestStringSliceFlexible_Array(t *testing.T) {
	var s stringSliceFlexible
	if err := json.Unmarshal([]byte(`["a","b"]`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s) != 2 || s[0] != "a" || s[1] != "b" {
		t.Fatalf("expected [a b], got %v", s)
	}
}

func TestStringSliceFlexible_String(t *testing.T) {
	var s stringSliceFlexible
	if err := json.Unmarshal([]byte(`"single"`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s) != 1 || s[0] != "single" {
		t.Fatalf("expected [single], got %v", s)
	}
}

func TestStringSliceFlexible_Null(t *testing.T) {
	var s stringSliceFlexible
	if err := json.Unmarshal([]byte(`null`), &s); err != nil {
		t.Fatal(err)
	}
	if s != nil {
		t.Fatalf("expected nil, got %v", s)
	}
}

func TestNarutoChatHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/naruto/chat", nil)
	w := httptest.NewRecorder()
	narutoChatHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestNarutoChatHandler_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/naruto/chat", strings.NewReader("bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	narutoChatHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNarutoChatHandler_EmptyMessage(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/naruto/chat", strings.NewReader(`{"message":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	narutoChatHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGodModeSystemPrompt_Default(t *testing.T) {
	t.Setenv("GOD_MODE_SYSTEM", "")
	t.Setenv("GOD_MODE_STYLE", "")

	p := godModeSystemPrompt()
	if !strings.Contains(p, "Naruto") {
		t.Fatalf("default prompt should mention Naruto, got %q", p)
	}
}

func TestGodModeSystemPrompt_CustomEnv(t *testing.T) {
	t.Setenv("GOD_MODE_SYSTEM", "custom system prompt")
	p := godModeSystemPrompt()
	if p != "custom system prompt" {
		t.Fatalf("expected custom prompt, got %q", p)
	}
}

func TestGodModeSystemPrompt_Styles(t *testing.T) {
	styles := map[string]string{
		"breve":     "BREVISSIME",
		"short":     "BREVISSIME",
		"elenco":    "elenco puntato",
		"bullet":    "elenco puntato",
		"didattico": "didattico",
	}
	for style, keyword := range styles {
		t.Run(style, func(t *testing.T) {
			t.Setenv("GOD_MODE_SYSTEM", "")
			t.Setenv("GOD_MODE_STYLE", style)
			p := godModeSystemPrompt()
			if !strings.Contains(p, keyword) {
				t.Fatalf("style %q should produce prompt with %q, got %q", style, keyword, p)
			}
		})
	}
}
