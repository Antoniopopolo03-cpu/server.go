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
func main() {
	http.HandleFunc("/", rootHandler)
	http.ListenAndServe(":3000", nil)
}
