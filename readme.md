# note

ottimo lavoro, alcune osservazioni:

1. non serve aggiungere alla dir del progetto la estensione ".go": non e' un errore, ma solo i files in go necessitano dell'estensione;
2. ricordati di pushare il go.mod (contiene tutti i packages che vengono usati nel progetto, che servono agli altri a scaricarli in locale (in questo caso sono state usate solo i package standard (inclusi in go) ma in generale non e' detto))
   (per non pushare dei files/dir che hai in locale puoi usare il .gitignore)
3. codice va bene;

miglioramento:
guarda il cambiamento che ho fatto, che evita di dover fare il check della coincidenza della rotta ad ogni endpoint:

e.g.---

prima:
```go
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprintf(w, "status healthy")
}
```


dopo:
```go
func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "status healthy")
}
```
 
4.todo: guarda come integrare swagger per documentare le api (chiedi a cursor): viene creato un endpoint speciale per vederre la documentazione
poi lo apri nel browser;


