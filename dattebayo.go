package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const dattebayoBaseURL = "https://dattebayo-api.onrender.com"

// stringSliceFlexible decodifica JSON sia come array di stringhe sia come singola stringa
// (l'API Dattebayo a volte manda "classification" in un modo o nell'altro).
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

// --- Request/response del tuo endpoint /naruto/chat ---

type NarutoChatRequest struct {
	Message string `json:"message"`
}

type NarutoChatResponse struct {
	Reply string `json:"reply"`
}

// --- JSON Dattebayo (personaggi) ---

type dattebayoCharactersResponse struct {
	Characters []dattebayoCharacter `json:"characters"`
	Total      int                  `json:"total"`
}

type dattebayoCharacter struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	Images   []string `json:"images"`
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
	Family map[string]string `json:"family"`
	// Lista molto lunga nell'API; inclusa in bozza solo se l'utente chiede tecniche/jutsu
	Jutsu []string `json:"jutsu"`
}

// --- JSON Dattebayo (clan) ---

type dattebayoClansResponse struct {
	Clans []dattebayoClan `json:"clans"`
	Total int             `json:"total"`
}

type dattebayoClan struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Characters []int  `json:"characters"`
}

// --- HTTP verso Dattebayo ---

func dattebayoGET(collection, name string, limit int) ([]byte, error) {
	u, err := url.Parse(dattebayoBaseURL + "/" + collection)
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

	slog.Info("dattebayo: GET", "collection", collection, "name", name, "url", u.String())
	start := time.Now()

	resp, err := http.Get(u.String())
	if err != nil {
		slog.Error("dattebayo: request failed", "error", err, "duration", time.Since(start))
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	slog.Info("dattebayo: response", "status", resp.StatusCode, "body_len", len(body), "duration", time.Since(start))
	return body, err
}

// --- Router messaggio utente ---

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

// shortFactualQuery rileva domande che vogliono risposta secca (una parola / pochissime parole).
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

// includeJutsuInDraft: se true, nella bozza si allegano le tecniche dall'API (lista troncata).
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

// --- Bozza per OpenAI ---

func draftFromCharacters(list []dattebayoCharacter, userQuery string) string {
	if len(list) == 0 {
		return "Nessun personaggio trovato nell'API Dattebayo per questa ricerca."
	}
	var b strings.Builder
	b.WriteString("Dati API Dattebayo (personaggi). Riassumi in italiano usando SOLO questi dati.\n\n")
	max := 3
	if len(list) < max {
		max = len(list)
	}
	for i := 0; i < max; i++ {
		c := list[i]
		b.WriteString(fmt.Sprintf("- %s (id %d)\n", c.Name, c.ID))
		if c.Personal.Clan != "" {
			b.WriteString(fmt.Sprintf("  Clan: %s\n", c.Personal.Clan))
		}
		if len(c.Personal.Affiliation) > 0 {
			b.WriteString(fmt.Sprintf("  Affiliazione: %s\n", strings.Join([]string(c.Personal.Affiliation), ", ")))
		}
		if c.Personal.Sex != "" {
			b.WriteString(fmt.Sprintf("  Sesso: %s\n", c.Personal.Sex))
		}
		if c.Personal.Birthdate != "" {
			b.WriteString(fmt.Sprintf("  Compleanno: %s\n", c.Personal.Birthdate))
		}
		if len(c.Personal.Classification) > 0 {
			b.WriteString(fmt.Sprintf("  Classificazione: %s\n", strings.Join([]string(c.Personal.Classification), ", ")))
		}
		r := c.Rank.NinjaRank
		if r.PartI != "" || r.PartII != "" || r.Gaiden != "" {
			b.WriteString(fmt.Sprintf("  Rango ninja: Part I=%s, Part II=%s, Gaiden=%s\n", r.PartI, r.PartII, r.Gaiden))
		}
		if c.Debut.Anime != "" {
			b.WriteString(fmt.Sprintf("  Debut anime: %s\n", c.Debut.Anime))
		}
		if c.Debut.Manga != "" {
			b.WriteString(fmt.Sprintf("  Debut manga: %s\n", c.Debut.Manga))
		}
		if len(c.Images) > 0 {
			b.WriteString(fmt.Sprintf("  Immagine (prima URL): %s\n", c.Images[0]))
		}
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

func draftFromClans(list []dattebayoClan) string {
	if len(list) == 0 {
		return "Nessun clan trovato nell'API Dattebayo per questa ricerca."
	}
	var b strings.Builder
	b.WriteString("Dati API Dattebayo (clan). Riassumi in italiano usando SOLO questi dati.\n\n")
	for _, cl := range list {
		b.WriteString(fmt.Sprintf("- Clan %s (id %d), personaggi collegati (id numerici): %d totali\n",
			cl.Name, cl.ID, len(cl.Characters)))
	}
	return b.String()
}

// runNarutoChatPipeline esegue Dattebayo → bozza → OpenAI (stessa logica di POST /naruto/chat).
// Usata da HTTP e WebSocket per evitare duplicazione.
func runNarutoChatPipeline(message string) (string, error) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", fmt.Errorf("message is required")
	}

	collection := detectCollection(msg)
	term := extractSearchTerm(msg)
	slog.Info("naruto: pipeline start", "collection", collection, "term", term, "factual", shortFactualQuery(msg))

	raw, err := dattebayoGET(collection, term, 5)
	if err != nil {
		return "", fmt.Errorf("dattebayo request failed: %w", err)
	}

	var draft string
	switch collection {
	case "clans":
		var parsed dattebayoClansResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return "", fmt.Errorf("failed parsing clans json: %w", err)
		}
		draft = draftFromClans(parsed.Clans)
	default:
		var parsed dattebayoCharactersResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return "", fmt.Errorf("failed parsing characters json: %w", err)
		}
		draft = draftFromCharacters(parsed.Characters, msg)
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

	userContent := "Domanda esatta dell'utente: " + msg + "\n\nDati API Dattebayo (unica fonte per fatti):\n" + draft
	return openAIChat(system, userContent)
}

// godModeSystemPrompt definisce come risponde la god mode (tutto in OpenAI, senza Dattebayo).
// Priorità: 1) variabile d'ambiente GOD_MODE_SYSTEM (testo completo del system prompt)
//           2) altrimenti GOD_MODE_STYLE: default | breve | elenco | didattico
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
	slog.Info("naruto: god mode", "message_len", len(msg))
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

	reply, err := runNarutoChatPipeline(req.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(NarutoChatResponse{Reply: reply})
}
