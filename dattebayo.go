package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const dattebayoBaseURL = "https://dattebayo-api.onrender.com"

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
		Clan           string   `json:"clan"`
		Affiliation    []string `json:"affiliation"`
		Sex            string   `json:"sex"`
		Birthdate      string   `json:"birthdate"`
		Classification []string `json:"classification"`
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

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
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

// --- Bozza per OpenAI ---

func draftFromCharacters(list []dattebayoCharacter) string {
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
			b.WriteString(fmt.Sprintf("  Affiliazione: %s\n", strings.Join(c.Personal.Affiliation, ", ")))
		}
		if c.Personal.Sex != "" {
			b.WriteString(fmt.Sprintf("  Sesso: %s\n", c.Personal.Sex))
		}
		if c.Personal.Birthdate != "" {
			b.WriteString(fmt.Sprintf("  Compleanno: %s\n", c.Personal.Birthdate))
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

	collection := detectCollection(req.Message)
	term := extractSearchTerm(req.Message)

	raw, err := dattebayoGET(collection, term, 5)
	if err != nil {
		http.Error(w, "dattebayo request failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	var draft string
	switch collection {
	case "clans":
		var parsed dattebayoClansResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			http.Error(w, "failed parsing clans json", http.StatusInternalServerError)
			return
		}
		draft = draftFromClans(parsed.Clans)
	default:
		var parsed dattebayoCharactersResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			http.Error(w, "failed parsing characters json", http.StatusInternalServerError)
			return
		}
		draft = draftFromCharacters(parsed.Characters)
	}

	var system string
	if shortFactualQuery(req.Message) {
		system = `Sei un assistente che risponde a domande puntuali su personaggi Naruto.
Rispondi in ITALIANO con UNA SOLA parola o al massimo TRE parole (es. "biondo", "azzurri", "non presente nei dati").
Niente frasi, niente spiegazioni, niente punti finali, niente virgolette.
Usa SOLO le informazioni presenti nel testo dati dall'utente (dati API).
Se l'informazione richiesta NON compare chiaramente in quei dati, rispondi esattamente: non presente nei dati`
	} else {
		system = `Sei un assistente per informazioni su Naruto (universo anime/manga).
Rispondi SEMPRE in italiano, in modo chiaro e leggibile (circa 6-14 righe).
Usa SOLO le informazioni presenti nel testo dati dall'utente (dati API Dattebayo).
Se i dati dicono che non ci sono risultati, spiegalo chiaramente.
Non inventare fatti non presenti nei dati.
Non mostrare JSON nella risposta finale.`
	}

	// Opzionale: rendi esplicita la domanda nell'input del modello
	userContent := "Domanda utente: " + req.Message + "\n\nDati API:\n" + draft

	reply, err := openAIChat(system, userContent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(NarutoChatResponse{Reply: reply})
}
