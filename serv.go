package main

import (
	"fmt"
	"net/http"
)

// handler per 2.1 root endpoint
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	fmt.Fprintf(w, "status healthy")
}

// handler per 2.2 saluta il server
func salutaHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/saluta" {
		http.NotFound(w, r)
		return
	}
	nome := r.URL.Query().Get("nome")
	if nome == "" {
		fmt.Fprintf(w, "ciao")
	} else {
		fmt.Fprintf(w, "ciao %s", nome)
	}
}

func main() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/saluta", salutaHandler)
	http.ListenAndServe(":3000", nil)

}
