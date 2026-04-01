# Playbook operativo backend Naruto

Guida pratica per intervenire velocemente sul server.

## 1) Aggiungere un nuovo provider (es. altra API)
1. Crea `provider_<nome>.go`.
2. Implementa interfaccia `NarutoProvider`:
   - `Name() string`
   - `SearchCharacters(ctx, req)`
   - `SearchClans(ctx, req)`
3. Mappa i dati API sui tipi canonici in `naruto_types.go`.
4. Inserisci il provider in `newNarutoRegistry()` dentro `dattebayo.go`.
5. Aggiungi test in `naruto_provider_test.go` per verificare ordine fallback.

## 2) Cambiare priorita fallback provider
1. Apri `dattebayo.go`.
2. Cerca `newNarutoRegistry()`.
3. Cambia l'ordine dei provider (dall'alto al basso = priorita).
4. Esegui `go test ./...` per confermare che non rompi il fallback.

## 3) Debug risposta sbagliata su /naruto/chat
1. Controlla `provider_used` nella risposta JSON.
2. Se il provider e sbagliato, verifica ordine in `newNarutoRegistry()`.
3. Se il provider e giusto ma i dati sono poveri:
   - controlla mapping nel file provider relativo (`provider_dattebayo.go`, `provider_narutodb.go`, `provider_jikan.go`)
4. Se il testo finale e strano:
   - controlla `system` e `userContent` in `runNarutoChatPipelineWithSource` (`dattebayo.go`).

## 4) Debug WebSocket (/ws/chat)
1. Apri `websocket_chat.go`.
2. Verifica payload in ingresso (`parseWebSocketUserText`).
3. Verifica output `wsOutbound`:
   - `reply`
   - `provider_used`
   - `god_mode`
4. In modalita `?god=1`, il provider non si applica (risposta OpenAI diretta).

## 5) Debug NarutoDB anti-bot
1. Apri `provider_narutodb.go`.
2. Controlla:
   - `maxRetries`
   - `retryBackoff`
   - `User-Agent`
   - funzione `isHTMLResponse`
3. Se cambia il comportamento del sito, aggiorna `isHTMLResponse`.
4. Conferma con test `provider_narutodb_test.go`.

## 6) Quando toccare i tipi canonici
Apri `naruto_types.go` solo se:
- serve un nuovo campo condiviso tra provider
- vuoi uniformare meglio i campi prima del prompt finale

Dopo modifica tipi:
1. aggiorna mapping in ogni provider
2. esegui `go test ./...`

## 7) Checklist rapida prima di push
1. `go test ./...`
2. `go build ./...`
3. verifica endpoint:
   - `POST /naruto/chat`
   - `WS /ws/chat`
4. controlla che `provider_used` sia valorizzato fuori da god mode
