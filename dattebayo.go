package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type NarutoChatRequest struct {
	Message string `json:"message"`
}

type NarutoChatResponse struct {
	Reply string `json:"reply"`
}

func detectCollection(msg string) string {
	l := strings.ToLower(msg)
	if strings.Contains(l, "clan") {
		return "clans"
	}
	return "characters"
}

func extractSearchTerm(msg string) string {
	m := strings.TrimSpace(msg)
	low := strings.ToLower(m)
	prefixes := []string{
		"personaggio ", "personaggi ", "il personaggio ",
		"clan ", "il clan ", "sul clan ", "del clan ", "chi è ", "chi e ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(low, p) {
			return strings.TrimSpace(m[len(p):])
		}
	}
	return m
}

// shortFactualQuery rileva domande che vogliono risposta secca.
func shortFactualQuery(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	keywords := []string{
		"colore", "capelli", "capello", "occhi", "occhio", "altezza", "peso",
		"età", "eta", "compleanno", "quanti anni", "sesso", "maschio", "femmina",
	}
	for _, k := range keywords {
		if strings.Contains(m, k) {
			return true
		}
	}
	return false
}

// includeJutsuInDraft: se true, nella bozza si allegano le tecniche API (lista troncata).
func includeJutsuInDraft(userQuery string) bool {
	m := strings.ToLower(userQuery)
	keys := []string{
		"tecnic", "jutsu", "rasengan", "clone", "ombra", "combatt", "lotta", "abilità", "abilita", "sa fare", "cosa sa",
	}
	for _, k := range keys {
		if strings.Contains(m, k) {
			return true
		}
	}
	return false
}

func draftFromCharacters(list []CanonicalCharacter, userQuery string, source string) string {
	if len(list) == 0 {
		return "Nessun personaggio trovato nelle API per questa ricerca."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Dati API aggregate (source principale: %s). Riassumi in italiano usando SOLO questi dati.\n\n", source))

	max := 3
	if len(list) < max {
		max = len(list)
	}
	for i := 0; i < max; i++ {
		c := list[i]
		b.WriteString(fmt.Sprintf("- %s (id %s, source %s)\n", c.Name, c.ID, c.Source))
		if c.Clan != "" {
			b.WriteString(fmt.Sprintf("  Clan: %s\n", c.Clan))
		}
		if len(c.Affiliation) > 0 {
			b.WriteString(fmt.Sprintf("  Affiliazione: %s\n", strings.Join(c.Affiliation, ", ")))
		}
		if c.Sex != "" {
			b.WriteString(fmt.Sprintf("  Sesso: %s\n", c.Sex))
		}
		if c.Birthdate != "" {
			b.WriteString(fmt.Sprintf("  Compleanno: %s\n", c.Birthdate))
		}
		if len(c.Classification) > 0 {
			b.WriteString(fmt.Sprintf("  Classificazione: %s\n", strings.Join(c.Classification, ", ")))
		}
		if c.RankPartI != "" || c.RankPartII != "" || c.RankGaiden != "" {
			b.WriteString(fmt.Sprintf("  Rango ninja: Part I=%s, Part II=%s, Gaiden=%s\n", c.RankPartI, c.RankPartII, c.RankGaiden))
		}
		if c.DebutAnime != "" {
			b.WriteString(fmt.Sprintf("  Debut anime: %s\n", c.DebutAnime))
		}
		if c.DebutManga != "" {
			b.WriteString(fmt.Sprintf("  Debut manga: %s\n", c.DebutManga))
		}
		// Evita di includere URL nella bozza, così la risposta finale non espone link API/immagini.
		if includeJutsuInDraft(userQuery) && len(c.Jutsu) > 0 {
			jLimit := 45
			if len(c.Jutsu) < jLimit {
				jLimit = len(c.Jutsu)
			}
			b.WriteString(fmt.Sprintf("  Tecniche/jutsu registrate nell'API (prime %d di %d):\n", jLimit, len(c.Jutsu)))
			for j := 0; j < jLimit; j++ {
				b.WriteString(fmt.Sprintf("    • %s\n", c.Jutsu[j]))
			}
			if len(c.Jutsu) > jLimit {
				b.WriteString(fmt.Sprintf("    ... altre %d tecniche presenti solo nell'API\n", len(c.Jutsu)-jLimit))
			}
		}
		b.WriteString("\n")
	}
	if len(list) > max {
		b.WriteString(fmt.Sprintf("... altri %d risultati non mostrati qui.\n", len(list)-max))
	}
	return b.String()
}

func draftFromClans(list []CanonicalClan, source string) string {
	if len(list) == 0 {
		return "Nessun clan trovato nelle API per questa ricerca."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Dati API aggregate (source principale: %s). Riassumi in italiano usando SOLO questi dati.\n\n", source))
	for _, cl := range list {
		b.WriteString(fmt.Sprintf("- Clan %s (id %s, source %s), personaggi collegati: %d totali\n",
			cl.Name, cl.ID, cl.Source, cl.CharacterCount))
	}
	return b.String()
}

func newNarutoRegistry() *ProviderRegistry {
	return NewProviderRegistry(
		NewDattebayoProvider(),
		NewNarutoDBProvider(),
		NewJikanProvider(),
	)
}

// runNarutoChatPipelineWithSource esegue provider multipli -> bozza -> OpenAI
// e ritorna anche il provider principale usato per i dati (solo uso interno).
func runNarutoChatPipelineWithSource(message string) (string, string, error) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", "", fmt.Errorf("message is required")
	}

	collection := detectCollection(msg)
	term := extractSearchTerm(msg)
	registry := newNarutoRegistry()
	ctx := context.Background()

	var draft string
	var providerUsed string
	switch collection {
	case "clans":
		clans, source, err := registry.SearchClansFirst(ctx, SearchRequest{Query: term, Limit: 5})
		if err != nil {
			return "", "", fmt.Errorf("provider clan search failed: %w", err)
		}
		draft = draftFromClans(clans, source)
		providerUsed = source
	default:
		chars, source, err := registry.SearchCharactersFirst(ctx, SearchRequest{Query: term, Limit: 5})
		if err != nil {
			return "", "", fmt.Errorf("provider character search failed: %w", err)
		}
		draft = draftFromCharacters(chars, msg, source)
		providerUsed = source
	}

	var system string
	if shortFactualQuery(msg) {
		system = `Sei un assistente che risponde a domande puntuali su personaggi Naruto.
Rispondi in ITALIANO con UNA SOLA parola o al massimo TRE parole (es. "biondo", "azzurri", "non presente nei dati").
Niente frasi, niente spiegazioni, niente punti finali, niente virgolette.
Usa SOLO le informazioni presenti nel testo dati dall'utente (dati API).
Se l'informazione richiesta NON compare chiaramente in quei dati, rispondi esattamente: non presente nei dati`
	} else {
		system = `Sei un assistente per informazioni su Naruto (universo anime/manga).
Rispondi SEMPRE in italiano.

PRECISIONE:
- Rispondi in modo DIRETTO alla domanda dell'utente nelle prime 1-2 frasi.
- Usa SOLO il blocco "Dati API" qui sotto (nessuna conoscenza esterna se quei dati bastano).
- Se compare la sezione "Tecniche/jutsu registrate nell'API" e l'utente chiede tecniche o capacità di combattimento,
  elenca o commenta SOLO tecniche presenti in quell'elenco (puoi scegliere le più rilevanti per la domanda).
- NON dire che mancano informazioni sulle tecniche se quell'elenco è presente e non vuoto.
- Se davvero non ci sono dati utili (nessun personaggio, elenco vuoto, ecc.), dillo in una frase breve senza inventare.
Non mostrare JSON nella risposta finale.`
	}

	userContent := "Domanda esatta dell'utente: " + msg + "\n\nDati API aggregate (unica fonte per fatti):\n" + draft
	reply, err := openAIChat(system, userContent)
	if err != nil {
		return "", providerUsed, err
	}
	return reply, providerUsed, nil
}

// runNarutoChatPipeline mantiene retrocompatibilità per i callsite che non usano provider_used.
func runNarutoChatPipeline(message string) (string, error) {
	reply, _, err := runNarutoChatPipelineWithSource(message)
	return reply, err
}

// godModeSystemPrompt definisce come risponde la god mode (tutto in OpenAI, senza Dattebayo).
// Priorità: 1) variabile d'ambiente GOD_MODE_SYSTEM (testo completo del system prompt)
// 2) altrimenti GOD_MODE_STYLE: default | breve | elenco | didattico
func godModeSystemPrompt() string {
	if s := strings.TrimSpace(os.Getenv("GOD_MODE_SYSTEM")); s != "" {
		return s
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GOD_MODE_STYLE"))) {
	case "breve", "short":
		return `Sei un esperto Naruto. Rispondi SEMPRE in italiano.
Risposte BREVISSIME: massimo 3-5 frasi totali, vai dritto al punto.
Puoi usare conoscenza generale (non sei limitato a Dattebayo).
Se la domanda è secca (es. un solo dato), rispondi anche con una sola frase.`
	case "elenco", "bullet", "punti":
		return `Sei un esperto Naruto. Rispondi SEMPRE in italiano.
Struttura la risposta con elenco puntato (2-8 punti) quando ha senso.
Puoi usare conoscenza generale. Sii chiaro e ordinato.`
	case "didattico", "lungo":
		return `Sei un esperto Naruto (anime, manga, lore). Rispondi SEMPRE in italiano.
Spiega in modo didattico: contesto, nomi, timeline se serve, ma resta coerente.
Puoi usare conoscenza generale. Se qualcosa è incerto o varia tra versioni, dillo.`
	default:
		return `Sei un assistente esperto dell'universo Naruto (anime, manga, lore).
Rispondi SEMPRE in italiano, in modo chiaro.
In questa modalità NON sei limitato ai dati dell'API Dattebayo: puoi usare la tua conoscenza generale.
Se qualcosa è incerto o varia tra versioni, indicalo brevemente.`
	}
}

// runNarutoChatGod salta Dattebayo: OpenAI risponde con conoscenza generale (modalità "god").
func runNarutoChatGod(message string) (string, error) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", fmt.Errorf("message is required")
	}
	return openAIChat(godModeSystemPrompt(), msg)
}

// @Summary      Chat Naruto (Dattebayo + OpenAI)
// @Description  Cerca su Dattebayo e risponde in italiano pulito via OpenAI
// @Tags         naruto
// @Accept       json
// @Produce      json
// @Param        body  body  NarutoChatRequest  true  "Messaggio utente"
// @Success      200   {object}  NarutoChatResponse
// @Router       /naruto/chat [post]
func narutoChatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NarutoChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	reply, _, err := runNarutoChatPipelineWithSource(req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(NarutoChatResponse{Reply: reply})
}
