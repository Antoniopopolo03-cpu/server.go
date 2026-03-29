# Review & Piano di Refactoring — Server Go Naruto Chat

> **Scopo di questo file**: guidare passo-passo il refactoring dell'applicazione.
> Ogni sezione spiega *cosa* fare, *perché* farlo e *come* implementarlo.
> Lo stile è didattico: si parte dal problema, si mostra la soluzione, si motiva la scelta architetturale.

---

## Indice

1. [Stato attuale e problemi](#1-stato-attuale-e-problemi)
2. [Struttura target del progetto](#2-struttura-target-del-progetto)
3. [Step 1 — Configurazione centralizzata](#3-step-1--configurazione-centralizzata)
4. [Step 2 — Interfaccia LLM Client](#4-step-2--interfaccia-llm-client)
5. [Step 3 — Separazione handler / logica / I/O](#5-step-3--separazione-handler--logica--io)
6. [Step 4 — Routing intelligente via LLM](#6-step-4--routing-intelligente-via-llm)
7. [Step 5 — Chiamate parallele e Retrieved Knowledge](#7-step-5--chiamate-parallele-e-retrieved-knowledge)
8. [Step 6 — Pulizia meccanica delle risposte API](#8-step-6--pulizia-meccanica-delle-risposte-api)
9. [Step 7 — LLM finale per la risposta utente](#9-step-7--llm-finale-per-la-risposta-utente)
10. [Riferimento: API Dattebayo completa](#10-riferimento-api-dattebayo-completa)
11. [Checklist finale](#11-checklist-finale)

---

## 1. Stato attuale e problemi

### Tabella dei debiti strutturali

| Problema | Dove | Principio violato | Impatto |
|----------|------|-------------------|---------|
| URL OpenAI hardcoded + logica duplicata | `serv.go:llmHandler` duplica `openai_helper.go:openAIChat` | DRY, Dependency Inversion | Non puoi cambiare provider LLM senza toccare più file |
| `os.Getenv()` sparsi in ogni funzione | Tutti i file | Single Source of Truth | Config non validata all'avvio, non testabile |
| Handler + logica + I/O nello stesso file | `dattebayo.go` (380 righe, 6 responsabilità) | Separation of Concerns | Impossibile estendere senza rischio di regressione |
| Tutto `package main` | Intero progetto | Encapsulation | Nessun boundary, nessun riuso, test accoppiati |
| Routing meccanico con `strings.Contains` | `dattebayo.go:detectCollection`, `extractSearchTerm` | — | Fragile, funziona solo con frasi hardcoded in italiano |

### Perché questi problemi contano

Quando tutto è nello stesso package e le dipendenze sono implicite, ogni modifica rischia di rompere
qualcosa di non correlato. Separare in package crea **contratti espliciti** (interfacce) tra componenti:
se il contratto è rispettato, puoi cambiare l'implementazione senza toccare il resto.

---

## 2. Struttura target del progetto

```
server.go/
├── main.go                     # Wiring: carica config, crea dipendenze, registra route, ListenAndServe
├── config/
│   └── config.go               # Struct Config + Load() da env/.env
├── llm/
│   ├── client.go               # Interfaccia Client + implementazione OpenAI
│   └── client_test.go
├── naruto/
│   ├── dattebayo.go            # Client HTTP per API Dattebayo (solo I/O)
│   ├── router.go               # LLM-based routing: prompt → array di URL da chiamare
│   ├── pipeline.go             # Orchestrazione: router → fetch parallelo → clean → LLM finale
│   ├── cleaner.go              # Pulizia meccanica delle risposte API (no LLM)
│   ├── handler.go              # Handler HTTP e WebSocket (solo layer HTTP)
│   └── *_test.go
├── middleware/
│   └── logging.go              # Middleware di logging HTTP
├── docs/                       # Swagger (generato)
├── .github/workflows/test.yml
└── go.mod
```

### Perché questa struttura

- **`main.go`** fa solo wiring (crea oggetti, li collega, avvia il server). Se domani vuoi
  aggiungere un CLI o un test di integrazione, puoi riusare i package senza passare da `main`.
- **`config/`** separa il "come si configura" dal "cosa fa l'app". Puoi passare da env a YAML
  cambiando un solo file.
- **`llm/`** definisce un'interfaccia — il resto dell'app non sa se sotto c'è OpenAI, Anthropic
  o un mock. Questo è il **Dependency Inversion Principle**: i moduli di alto livello (pipeline)
  non dipendono da moduli di basso livello (HTTP a OpenAI), entrambi dipendono da un'astrazione.
- **`naruto/`** contiene tutta la business logic Naruto. Se domani aggiungi un dominio "One Piece",
  crei un package `onepiece/` con la stessa struttura senza toccare `naruto/`.
- **`middleware/`** è riusabile su qualsiasi progetto.

---

## 3. Step 1 — Configurazione centralizzata

### Problema

Oggi ogni funzione chiama `os.Getenv("OPENAI_API_KEY")` quando serve. Se la key manca, l'utente
lo scopre solo alla prima richiesta.

### Soluzione

Creare `config/config.go`:

```go
package config

import (
    "fmt"
    "os"

    "github.com/joho/godotenv"
)

type Config struct {
    Port           string
    OpenAIKey      string
    OpenAIModel    string
    MockLLM        bool
    GodModeStyle   string
    GodModeSystem  string
}

func Load() (*Config, error) {
    _ = godotenv.Load() // .env opzionale

    key := os.Getenv("OPENAI_API_KEY")
    mock := os.Getenv("MOCK_LLM") == "true"

    if key == "" && !mock {
        return nil, fmt.Errorf("OPENAI_API_KEY richiesta (oppure setta MOCK_LLM=true)")
    }

    model := os.Getenv("OPENAI_MODEL")
    if model == "" {
        model = "gpt-4o-mini"
    }

    return &Config{
        Port:          getEnvDefault("PORT", "3000"),
        OpenAIKey:     key,
        OpenAIModel:   model,
        MockLLM:       mock,
        GodModeStyle:  os.Getenv("GOD_MODE_STYLE"),
        GodModeSystem: os.Getenv("GOD_MODE_SYSTEM"),
    }, nil
}

func getEnvDefault(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

### Perché

- **Fail fast**: se manca la key, il server non parte → scopri il problema in 0 secondi, non dopo
  la prima richiesta utente.
- **Testabilità**: nei test crei un `Config{}` a mano, senza toccare variabili d'ambiente.
- **Single source of truth**: un solo posto dove leggere e validare la configurazione.

### Come usarla in `main.go`

```go
func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }
    // passa cfg ai componenti che ne hanno bisogno
}
```

---

## 4. Step 2 — Interfaccia LLM Client

### Problema

Oggi ci sono due implementazioni della chiamata OpenAI: `llmHandler` (in `serv.go`) e `openAIChat`
(in `openai_helper.go`). Entrambe hanno l'URL hardcoded. Non puoi sostituire il provider.

### Soluzione

Creare `llm/client.go`:

```go
package llm

// Client è l'astrazione per qualsiasi LLM provider.
type Client interface {
    Chat(systemPrompt, userPrompt string) (string, error)
}
```

Poi l'implementazione OpenAI:

```go
type OpenAIClient struct {
    apiKey  string
    model   string
    baseURL string // default "https://api.openai.com/v1"
}

func NewOpenAIClient(apiKey, model string) *OpenAIClient {
    return &OpenAIClient{
        apiKey:  apiKey,
        model:   model,
        baseURL: "https://api.openai.com/v1",
    }
}

func (c *OpenAIClient) Chat(systemPrompt, userPrompt string) (string, error) {
    // stessa logica di openAIChat, ma usa c.apiKey, c.model, c.baseURL
    // ...
}
```

### Perché un'interfaccia

1. **Testabilità**: nei test passi un `mockLLMClient` che ritorna risposte fisse → test deterministici,
   veloci, senza rete.
2. **Sostituibilità**: vuoi provare Anthropic? Implementi `AnthropicClient` che soddisfa `Client`.
   Il resto dell'app non cambia.
3. **`baseURL` configurabile**: nei test puoi puntare a un `httptest.Server` locale senza hackare
   `http.DefaultTransport`.

### Mock per i test

```go
type MockClient struct {
    Response string
    Err      error
}

func (m *MockClient) Chat(_, _ string) (string, error) {
    return m.Response, m.Err
}
```

---

## 5. Step 3 — Separazione handler / logica / I/O

### Problema

`dattebayo.go` contiene: tipi JSON, client HTTP, routing, draft builder, system prompt, handler HTTP.
Modificare una cosa rischia di romperne un'altra.

### Soluzione: dividere per responsabilità

#### `naruto/dattebayo.go` — solo client HTTP

```go
package naruto

// DattebayoClient gestisce le chiamate HTTP all'API Dattebayo.
type DattebayoClient struct {
    baseURL string
}

func NewDattebayoClient(baseURL string) *DattebayoClient {
    return &DattebayoClient{baseURL: baseURL}
}

// Fetch esegue una GET e ritorna il body raw.
func (d *DattebayoClient) Fetch(endpoint string, params map[string]string) ([]byte, error) {
    // costruisci URL, fai GET, ritorna body
}
```

#### `naruto/handler.go` — solo layer HTTP/WS

```go
// Handler contiene le dipendenze e registra le route.
type Handler struct {
    pipeline *Pipeline
}

func (h *Handler) NarutoChatHTTP(w http.ResponseWriter, r *http.Request) {
    // decodifica request JSON
    // chiama h.pipeline.Run(message)
    // codifica response JSON
}

func (h *Handler) ChatWebSocket(w http.ResponseWriter, r *http.Request) {
    // gestione WS, per ogni messaggio chiama h.pipeline.Run(message)
}
```

#### `naruto/pipeline.go` — orchestrazione

```go
type Pipeline struct {
    llm       llm.Client
    dattebayo *DattebayoClient
}

func (p *Pipeline) Run(userMessage string) (string, error) {
    // 1. Router LLM → array di URL
    // 2. Fetch parallelo
    // 3. Pulizia meccanica
    // 4. LLM finale → risposta
}
```

### Perché questa separazione

Ogni file ha **una sola ragione per cambiare**:
- L'API Dattebayo cambia endpoint? → modifica `dattebayo.go`, i test di `pipeline.go` non si rompono.
- Vuoi aggiungere rate limiting HTTP? → modifica `handler.go`, la logica non cambia.
- Vuoi cambiare la strategia di routing? → modifica `router.go`, handler e client restano identici.

---

## 6. Step 4 — Routing intelligente via LLM

Questo è il cambiamento più importante. Oggi il routing è meccanico (`strings.Contains("clan")`).
Il nuovo approccio usa un LLM per **capire** cosa vuole l'utente e **produrre** le query API.

### Come funziona

Il "router" è una chiamata LLM con un system prompt che contiene la documentazione dell'API Dattebayo.
Il prompt chiede al modello di produrre un JSON strutturato con gli URL da chiamare.

### System prompt per il router

Creare `naruto/router.go`:

```go
const routerSystemPrompt = `Sei un router API. Il tuo compito è tradurre la domanda dell'utente
in una lista di chiamate all'API Dattebayo.

## API Dattebayo — Endpoint disponibili

Base URL: https://dattebayo-api.onrender.com

| Endpoint             | Descrizione                          | Parametri query        |
|----------------------|--------------------------------------|------------------------|
| GET /characters      | Cerca personaggi per nome            | name, page, limit      |
| GET /characters/{id} | Singolo personaggio per ID           | —                      |
| GET /clans           | Cerca clan per nome                  | name, page, limit      |
| GET /clans/{id}      | Singolo clan per ID                  | —                      |
| GET /villages        | Cerca villaggi per nome              | name, page, limit      |
| GET /villages/{id}   | Singolo villaggio per ID             | —                      |
| GET /akatsuki        | Membri Akatsuki (schema personaggio) | name, page, limit      |
| GET /tailed-beasts   | Cercoteri / bestie con coda          | name, page, limit      |
| GET /kara            | Membri organizzazione Kara           | name, page, limit      |
| GET /kekkei-genkai   | Abilità ereditarie (Sharingan, etc.) | name, page, limit      |
| GET /teams           | Team ninja                           | name, page, limit      |

## Schemi di risposta

Le risposte delle list endpoint sono paginate:
{"<resource>": [...], "currentPage": 1, "pageSize": 20, "total": N}

I personaggi (da /characters, /akatsuki, /tailed-beasts, /kara) hanno campi ricchi:
name, images, debut, family, jutsu, natureType, personal (birthdate, sex, age, height,
weight, bloodType, kekkeiGenkai, classification, affiliation, team, clan), rank, tools,
voiceActors.

Clan, villaggi, team, kekkei-genkai hanno schema semplice: {id, name, characters: [int]}.

## Regole

1. Rispondi SOLO con un array JSON valido, nessun altro testo.
2. Ogni elemento ha: "url" (string, URL completo) e "usage" (string, perché serve questa chiamata).
3. Usa sempre limit=5 a meno che la domanda non chieda esplicitamente di più.
4. Se la domanda menziona un personaggio, cerca in /characters.
5. Se la domanda menziona un clan, cerca in /clans. Se serve anche i personaggi del clan,
   aggiungi una seconda chiamata a /characters con il nome del clan.
6. Se la domanda riguarda Akatsuki, usa /akatsuki.
7. Se la domanda riguarda bestie con coda / jinchuuriki, usa /tailed-beasts.
8. Se la domanda riguarda kekkei genkai / sharingan / byakugan, usa /kekkei-genkai.
9. Se la domanda è generica o ambigua, usa /characters come default.
10. Puoi produrre più chiamate se servono dati da endpoint diversi.

## Esempi

Domanda: "parlami di Naruto"
Risposta:
[{"url": "https://dattebayo-api.onrender.com/characters?name=Naruto&limit=5", "usage": "Cerco il personaggio Naruto per avere dati biografici completi"}]

Domanda: "quali sono le tecniche del clan Uchiha?"
Risposta:
[
  {"url": "https://dattebayo-api.onrender.com/clans?name=Uchiha&limit=1", "usage": "Ottengo info sul clan Uchiha e gli ID dei personaggi"},
  {"url": "https://dattebayo-api.onrender.com/characters?name=Uchiha&limit=5", "usage": "Cerco i personaggi Uchiha per vedere le loro tecniche/jutsu"}
]

Domanda: "chi sono i membri dell'Akatsuki?"
Risposta:
[{"url": "https://dattebayo-api.onrender.com/akatsuki?limit=10", "usage": "Elenco dei membri Akatsuki con i loro dati"}]

Domanda: "cos'è lo Sharingan?"
Risposta:
[
  {"url": "https://dattebayo-api.onrender.com/kekkei-genkai?name=Sharingan&limit=1", "usage": "Info sullo Sharingan come kekkei genkai"},
  {"url": "https://dattebayo-api.onrender.com/characters?name=Uchiha&limit=5", "usage": "Personaggi Uchiha che possiedono lo Sharingan"}
]
`
```

### Implementazione del router

```go
// RouteRequest rappresenta una singola chiamata API da fare.
type RouteRequest struct {
    URL   string `json:"url"`
    Usage string `json:"usage"`
}

// Route chiede al LLM di tradurre la domanda utente in chiamate API.
func (p *Pipeline) Route(userMessage string) ([]RouteRequest, error) {
    raw, err := p.llm.Chat(routerSystemPrompt, userMessage)
    if err != nil {
        return nil, fmt.Errorf("router llm call failed: %w", err)
    }

    var routes []RouteRequest
    if err := json.Unmarshal([]byte(raw), &routes); err != nil {
        return nil, fmt.Errorf("router returned invalid JSON: %w", err)
    }
    return routes, nil
}
```

### Perché usare un LLM per il routing

| Aspetto | Vecchio (string matching) | Nuovo (LLM router) |
|---------|---------------------------|---------------------|
| "parlami del figlio di Minato" | Cerca "parlami del figlio di Minato" come nome | Capisce → cerca "Naruto" in /characters |
| "membri dell'Akatsuki" | Non gestito (va in /characters) | Usa /akatsuki |
| "cos'è il Byakugan" | Non gestito | Usa /kekkei-genkai + /characters |
| Nuova lingua (inglese, spagnolo) | Bisogna aggiungere nuovi prefissi | Funziona out of the box |
| Nuovo endpoint API | Bisogna aggiungere codice | Basta aggiornare il system prompt |

Il costo è una chiamata LLM in più (~200-500ms con gpt-4o-mini), ma il guadagno in flessibilità
è enorme. Il routing diventa **configurazione** (il prompt) invece che **codice**.

### Validazione dell'output del router

Il LLM potrebbe produrre JSON malformato o URL non validi. Aggiungere validazione:

```go
func validateRoutes(routes []RouteRequest) []RouteRequest {
    valid := make([]RouteRequest, 0, len(routes))
    for _, r := range routes {
        u, err := url.Parse(r.URL)
        if err != nil || !strings.HasPrefix(u.Host, "dattebayo-api") {
            slog.Warn("router: skipping invalid URL", "url", r.URL)
            continue
        }
        valid = append(valid, r)
    }
    return valid
}
```

---

## 7. Step 5 — Chiamate parallele e Retrieved Knowledge

### Concetto

Una volta che il router produce N URL, li chiamiamo **tutti in parallelo** con goroutine
e raccogliamo i risultati in un array strutturato chiamato **Retrieved Knowledge (RK)**.

### Struttura dati

```go
// RetrievedKnowledge è il risultato di una singola chiamata API.
type RetrievedKnowledge struct {
    URL      string `json:"url_chiamato"`
    Response string `json:"response"`
    Usage    string `json:"usage"`
}
```

### Implementazione con goroutine + errgroup

```go
import "golang.org/x/sync/errgroup"

func (p *Pipeline) FetchAll(routes []RouteRequest) []RetrievedKnowledge {
    results := make([]RetrievedKnowledge, len(routes))
    g, _ := errgroup.WithContext(context.Background())

    for i, route := range routes {
        g.Go(func() error {
            body, err := p.dattebayo.FetchRaw(route.URL)
            if err != nil {
                slog.Error("fetch failed", "url", route.URL, "error", err)
                results[i] = RetrievedKnowledge{
                    URL:   route.URL,
                    Response: fmt.Sprintf("errore: %v", err),
                    Usage: route.Usage,
                }
                return nil // non blocchiamo le altre chiamate
            }
            results[i] = RetrievedKnowledge{
                URL:      route.URL,
                Response: string(body),
                Usage:    route.Usage,
            }
            return nil
        })
    }

    g.Wait()
    return results
}
```

### Perché parallelizzare

Se il router produce 3 URL e ogni chiamata a Dattebayo prende ~300ms:
- **Sequenziale**: 900ms
- **Parallelo**: ~300ms (il tempo della chiamata più lenta)

Con `errgroup` gestisci anche il caso in cui una chiamata fallisce senza bloccare le altre.
Il `return nil` nell'errore è intenzionale: una chiamata fallita non deve impedire alle altre
di completarsi — il risultato parziale è comunque utile per il LLM finale.

---

## 8. Step 6 — Pulizia meccanica delle risposte API

### Il problema

Le risposte dell'API Dattebayo sono JSON ricchi. Un singolo personaggio può avere 50+ campi.
Passare tutto il JSON grezzo al LLM finale:
1. Spreca token (e soldi)
2. Diluisce l'informazione utile nel rumore
3. Può superare il context window su risposte multiple

### Studio delle risposte API

Le risposte Dattebayo hanno **tre forme**:

#### Forma 1: Personaggio completo (`/characters`, `/akatsuki`, `/tailed-beasts`, `/kara`)

Campi sempre presenti e utili:
- `name`, `id`
- `personal.sex`, `personal.birthdate`, `personal.clan`, `personal.affiliation`
- `rank.ninjaRank`

Campi ricchi ma spesso non richiesti:
- `jutsu` → array di 50-200 stringhe (enorme, serve solo se l'utente chiede tecniche)
- `tools` → lista armi
- `voiceActors` → raramente utile
- `images` → URL, utile solo per mostrare un'immagine

Campi con struttura variabile:
- `personal.age`, `personal.height`, `personal.weight` → oggetti con chiavi dinamiche per arco narrativo
- `family` → oggetto con chiavi dinamiche per tipo di relazione

#### Forma 2: Riferimento semplice (`/clans`, `/villages`, `/teams`, `/kekkei-genkai`)

```json
{"id": 1, "name": "Uchiha", "characters": [1, 2, 3, ...]}
```

Qui `characters` è un array di ID numerici — non contiene dati utili da solo,
serve per fare un secondo fetch.

#### Forma 3: Wrapper paginazione (tutte le list)

```json
{"characters": [...], "currentPage": 1, "pageSize": 20, "total": 1431}
```

I campi `currentPage`, `pageSize`, `total` sono metadata di paginazione, non servono al LLM finale.

### Strategia di pulizia

Creare `naruto/cleaner.go` con pulizia **meccanica** (no LLM, puro codice):

```go
// CleanResponse prende il JSON grezzo e rimuove campi non necessari.
// Questa è pulizia deterministica, non interpretazione.
func CleanResponse(raw []byte, userQuery string) ([]byte, error) {
    var data map[string]any
    if err := json.Unmarshal(raw, &data); err != nil {
        return raw, nil // se non è JSON valido, ritorna com'è
    }

    // 1. Rimuovi metadata di paginazione (non servono al LLM)
    delete(data, "currentPage")
    delete(data, "pageSize")
    // "total" può essere utile ("ci sono 29 personaggi nel clan")

    // 2. Se contiene un array di personaggi, pulisci ognuno
    for key, val := range data {
        if arr, ok := val.([]any); ok {
            data[key] = cleanCharacterArray(arr, userQuery)
        }
    }

    return json.Marshal(data)
}

func cleanCharacterArray(arr []any, userQuery string) []any {
    wantsJutsu := includeJutsuInQuery(userQuery)
    for i, item := range arr {
        if char, ok := item.(map[string]any); ok {
            // Rimuovi campi pesanti e raramente utili
            delete(char, "voiceActors")

            // Tronca jutsu se non richiesto
            if !wantsJutsu {
                delete(char, "jutsu")
            } else if jutsu, ok := char["jutsu"].([]any); ok && len(jutsu) > 20 {
                char["jutsu"] = jutsu[:20] // tronca a 20
            }

            // Tronca tools
            if tools, ok := char["tools"].([]any); ok && len(tools) > 10 {
                char["tools"] = tools[:10]
            }

            arr[i] = char
        }
    }
    return arr
}
```

### Perché pulizia meccanica e non un altro LLM

| Approccio | Pro | Contro |
|-----------|-----|--------|
| Nessuna pulizia | Semplice | Spreco token, risposte lente |
| Pulizia meccanica (codice) | Veloce (< 1ms), deterministico, gratuito | Non capisce il contesto |
| Pulizia via LLM | Capisce cosa è rilevante | +500ms, +costo, non deterministico |

La pulizia meccanica è il **miglior compromesso**: rimuove il rumore ovvio (voiceActors, paginazione,
jutsu troncati) senza aggiungere latenza o costi. Il LLM finale è comunque capace di ignorare
campi irrilevanti che restano — la pulizia meccanica gli facilita il lavoro, non lo sostituisce.

### Quando la pulizia meccanica NON basta

Se in futuro servisse selezionare dinamicamente quali campi includere in base alla domanda
(es. "confronta l'altezza di Naruto e Sasuke" → tieni solo `personal.height`), a quel punto
ha senso aggiungere un LLM di pulizia. Ma è un'ottimizzazione da fare **solo quando il costo
token diventa un problema misurabile**, non preventivamente.

---

## 9. Step 7 — LLM finale per la risposta utente

### Pipeline completa

A questo punto il flusso è:

```
Utente: "quali tecniche conosce Itachi?"
        │
        ▼
   [LLM Router]
        │
        ▼
   RouteRequests:
   [
     {"url": ".../characters?name=Itachi&limit=3", "usage": "Dati su Itachi e le sue tecniche"},
     {"url": ".../akatsuki?name=Itachi&limit=1",   "usage": "Dati su Itachi come membro Akatsuki"}
   ]
        │
        ▼
   [Fetch Parallelo] (goroutine)
        │
        ▼
   Retrieved Knowledge (RK):
   [
     {"url_chiamato": ".../characters?name=Itachi&limit=3", "response": "{...}", "usage": "..."},
     {"url_chiamato": ".../akatsuki?name=Itachi&limit=1",   "response": "{...}", "usage": "..."}
   ]
        │
        ▼
   [Pulizia Meccanica] (rimuovi voiceActors, tronca jutsu a 20, rimuovi paginazione)
        │
        ▼
   RK pulita
        │
        ▼
   [LLM Finale] (system prompt + domanda utente + RK come contesto)
        │
        ▼
   Risposta all'utente
```

### System prompt per il LLM finale

```
Sei un assistente esperto dell'universo Naruto (anime, manga, lore).
Rispondi SEMPRE in italiano, in modo chiaro e diretto.

REGOLE:
- Rispondi alla domanda dell'utente nelle prime 1-2 frasi.
- Usa SOLO i dati presenti nella sezione "Conoscenza Recuperata" qui sotto.
- NON inventare informazioni non presenti nei dati.
- Se i dati non contengono la risposta, dillo onestamente.
- Non mostrare JSON, ID numerici o URL nella risposta.
- Se ci sono più fonti con dati sovrapposti, uniscili senza ripetere.
```

### Costruzione del prompt utente

```go
func buildFinalPrompt(userMessage string, rk []RetrievedKnowledge) string {
    var b strings.Builder
    b.WriteString("Domanda dell'utente: " + userMessage + "\n\n")
    b.WriteString("## Conoscenza Recuperata\n\n")
    for _, item := range rk {
        b.WriteString("### " + item.Usage + "\n")
        b.WriteString("Fonte: " + item.URL + "\n")
        b.WriteString(item.Response + "\n\n")
    }
    return b.String()
}
```

### Orchestrazione in `pipeline.go`

```go
func (p *Pipeline) Run(userMessage string) (string, error) {
    // 1. Router: domanda → URL
    routes, err := p.Route(userMessage)
    if err != nil {
        return "", fmt.Errorf("routing failed: %w", err)
    }
    routes = validateRoutes(routes)
    if len(routes) == 0 {
        return "", fmt.Errorf("il router non ha prodotto chiamate API")
    }

    // 2. Fetch parallelo
    rk := p.FetchAll(routes)

    // 3. Pulizia meccanica
    for i := range rk {
        cleaned, err := CleanResponse([]byte(rk[i].Response), userMessage)
        if err == nil {
            rk[i].Response = string(cleaned)
        }
    }

    // 4. LLM finale
    finalPrompt := buildFinalPrompt(userMessage, rk)
    return p.llm.Chat(finalSystemPrompt, finalPrompt)
}
```

---

## 10. Riferimento: API Dattebayo completa

Questa sezione serve come documentazione di riferimento.
È lo stesso contenuto usato nel system prompt del router (Step 4).

### Endpoint

| Risorsa | List | Singolo | Totale items | Schema risposta |
|---------|------|---------|-------------|-----------------|
| Characters | `GET /characters` | `GET /characters/{id}` | 1431 | Personaggio completo |
| Clans | `GET /clans` | `GET /clans/{id}` | 58 | `{id, name, characters[]}` |
| Villages | `GET /villages` | `GET /villages/{id}` | 39 | `{id, name, characters[]}` |
| Akatsuki | `GET /akatsuki` | `GET /akatsuki/{id}` | 44 | Personaggio completo |
| Tailed Beasts | `GET /tailed-beasts` | `GET /tailed-beasts/{id}` | 10 | Personaggio completo |
| Kara | `GET /kara` | `GET /kara/{id}` | 32 | Personaggio completo |
| Kekkei Genkai | `GET /kekkei-genkai` | `GET /kekkei-genkai/{id}` | 39 | `{id, name, characters[]}` |
| Teams | `GET /teams` | `GET /teams/{id}` | 191 | `{id, name, characters[]}` |

### Parametri query (tutte le list)

| Parametro | Tipo | Default | Descrizione |
|-----------|------|---------|-------------|
| `name` | string | — | Filtro per nome (substring, case-insensitive) |
| `page` | int | 1 | Pagina |
| `limit` | int | 20 | Elementi per pagina |

### Wrapper paginazione

```json
{
  "<risorsa>": [ ... ],
  "currentPage": 1,
  "pageSize": 20,
  "total": 1431
}
```

### Schema personaggio (campi principali)

```
name: string
images: string[]
personal:
  birthdate: string
  sex: string
  age: {"Part I": "12-13", "Part II": "15-17", ...}
  height: {"Part I": "145.3 cm", ...}
  weight: {"Part I": "40.1 kg", ...}
  bloodType: string
  clan: string
  affiliation: string[]
  classification: string | string[]
  kekkeiGenkai: string | string[]
  team: string[]
debut:
  manga: string
  anime: string
family: {father: string, mother: string, ...}  // chiavi dinamiche
jutsu: string[]          // può avere 50-200 elementi
natureType: string[]
rank:
  ninjaRank: {"Part I": "Genin", "Part II": "Kage", ...}
  ninjaRegistration: string
tools: string[]
voiceActors:
  japanese: string | string[]
  english: string | string[]
```

**Nota**: molti campi sono opzionali. Personaggi minori possono avere solo `name` e `id`.

---

## 11. Checklist finale

Ordine consigliato di implementazione (ogni step è indipendente dal successivo per i test):

- [ ] **Step 1**: `config/config.go` — struct Config + Load + validazione all'avvio
- [ ] **Step 2**: `llm/client.go` — interfaccia Client + implementazione OpenAI + mock
- [ ] **Step 3**: Spostare codice in package `naruto/` e `middleware/` — separare handler, pipeline, client Dattebayo
- [ ] **Step 4**: `naruto/router.go` — routing via LLM con documentazione API nel prompt
- [ ] **Step 5**: `naruto/pipeline.go` — fetch parallelo con errgroup, struttura RetrievedKnowledge
- [ ] **Step 6**: `naruto/cleaner.go` — pulizia meccanica risposte (rimuovi paginazione, tronca jutsu, rimuovi voiceActors)
- [ ] **Step 7**: Collegare tutto nel pipeline Run() — router → fetch → clean → LLM finale
- [ ] **Step 8**: Aggiornare i test per la nuova struttura a package
- [ ] **Step 9**: Aggiornare `main.go` per fare wiring di tutte le dipendenze
- [ ] **Step 10**: Verificare che la GitHub Actions workflow funzioni con la nuova struttura

### Nota su Go modules

Quando crei i sub-package (`config/`, `llm/`, `naruto/`, `middleware/`), gli import saranno:

```go
import (
    "server/config"
    "server/llm"
    "server/naruto"
    "server/middleware"
)
```

Questo perché il module name in `go.mod` è `server`. Non serve modificare `go.mod`.
