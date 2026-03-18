package main

import (
	"fmt"
	"net/http"
)

// handler per 2.1 root endpoint
func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "status healthy")
}

// handler per 2.2 saluta il server
func salutaHandler(w http.ResponseWriter, r *http.Request) {
	nome := r.URL.Query().Get("nome")
	if nome == "" {
		fmt.Fprintf(w, "ciao")
	} else {
		fmt.Fprintf(w, "ciao %s", nome)
	}
}

func main() {
	http.HandleFunc("/{$}", rootHandler) // suggerimento: invece di "/" (cattura tutto) usi il "/{$}" che cattura solo lo slash
	http.HandleFunc("/saluta", salutaHandler)
	http.ListenAndServe(":3000", nil)
}
