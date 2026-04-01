# Mappa rapida backend Naruto

Questa nota serve per trovare velocemente dove intervenire nel codice.

## Routing e fallback provider
- File: `naruto_provider.go`
- Cosa contiene:
  - interfaccia `NarutoProvider`
  - registry `ProviderRegistry`
  - logica "first provider con risultati"
- Quando toccarlo:
  - vuoi cambiare priorita provider
  - vuoi cambiare strategia fallback
  - vuoi aggiungere metriche/error handling globale

## Pipeline chat e output API HTTP
- File: `dattebayo.go`
- Cosa contiene:
  - `runNarutoChatPipelineWithSource(...)`
  - `runNarutoChatPipeline(...)` (retrocompat)
  - `narutoChatHandler` endpoint `/naruto/chat`
  - `NarutoChatResponse` con `provider_used`
- Quando toccarlo:
  - vuoi cambiare prompt/format risposta
  - vuoi cambiare struttura JSON HTTP
  - vuoi cambiare estrazione intent/search term

## Output realtime WebSocket
- File: `websocket_chat.go`
- Cosa contiene:
  - handler `/ws/chat`
  - parse payload testo/JSON
  - risposta `wsOutbound` con `provider_used`
- Quando toccarlo:
  - vuoi cambiare protocollo WS
  - vuoi aggiungere nuovi campi in risposta realtime

## Hardening NarutoDB
- File: `provider_narutodb.go`
- Cosa contiene:
  - retry con backoff
  - header custom (`User-Agent`, `Accept`)
  - rilevamento anti-bot HTML (`isHTMLResponse`)
- Quando toccarlo:
  - narutodb cambia comportamento
  - servono piu tentativi o timeout diversi
  - vuoi logging diagnostico dettagliato

## Fallback Jikan
- File: `provider_jikan.go`
- Cosa contiene:
  - provider fallback su `api.jikan.moe`
  - mapping base in `CanonicalCharacter`
- Quando toccarlo:
  - vuoi arricchire campi da Jikan
  - vuoi gestire rate-limit/retry specifici Jikan

## Contratti dati condivisi
- File: `naruto_types.go`
- Cosa contiene:
  - `SearchRequest`
  - `CanonicalCharacter`
  - `CanonicalClan`
- Quando toccarlo:
  - aggiungi nuovi campi condivisi tra provider
  - cambi il contratto usato dalla pipeline

## Test fallback e orchestrazione
- File: `naruto_provider_test.go`
- Cosa copre:
  - ordine fallback provider
  - casi errore multipli
  - primo risultato non vuoto
- Quando toccarlo:
  - cambi regole nel registry/provider order

## Test anti-bot/parsing HTML
- File: `provider_narutodb_test.go`
- Cosa copre:
  - detection HTML via content-type
  - detection HTML via body (`<!doctype html>`, `<html>`)
  - caso JSON valido
- Quando toccarlo:
  - aggiorni `isHTMLResponse`
  - introduci nuove firme anti-bot

## Ordine provider attuale (personaggi)
1. Dattebayo
2. NarutoDB
3. Jikan

Se vuoi cambiarlo: `newNarutoRegistry()` in `dattebayo.go`.
