// @title           API Server Go
// @version         1.0
// @description     Documentazione API con Swagger
// @host            localhost:3000
// @BasePath        /
package main

import (
	"fmt"
	"net/http"
	"os"

	_ "server/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

// handler per 2.1 root endpoint
// @Summary      Health check
// @Description  Risponde "status healthy" sulla root
// @Tags         system
// @Produce      plain
// @Success      200  {string}  string  "status healthy"
// @Router       / [get]
func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "status healthy")
}

// handler per 2.2 saluta il server
// @Summary      Saluta il server
// @Description  Risponde "ciao" o "ciao {nome}"
// @Tags         system
// @Produce      plain
// @Success      200  {string}  string  "ciao"  "ciao {nome}"
// @Router       /saluta [get]
func salutaHandler(w http.ResponseWriter, r *http.Request) {
	nome := r.URL.Query().Get("nome")
	if nome == "" {
		fmt.Fprintf(w, "ciao")
	} else {
		fmt.Fprintf(w, "ciao %s", nome)
	}
}
// BestemmiaHandler handler per una 2.3 bestemmia endpoint
// @Summary      Bestemmia il server
// @Description  Risponde "Porco Dio" o "ciao {nome} che voi Porco Dio"
// @Tags         system
// @Produce      plain
// @Success      200  {string}  string  "Porco Dio"  "ciao {nome} che voi Porco Dio"
// @Router       /saluta/con-bestemmia [get]
func BestemmiaHandler(w http.ResponseWriter, r *http.Request) {
	nome := r.URL.Query().Get("nome")
	if nome == "" {
		fmt.Fprintf(w, "Porco Dio")
	} else {
		fmt.Fprintf(w, "ciao %s che voi Porco Dio", nome)
	}
}
func main() {
	// Swagger UI: http://localhost:3000/swagger/index.html
	http.HandleFunc("/swagger/*", httpSwagger.WrapHandler)

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/saluta/con-bestemmia", BestemmiaHandler)
	http.HandleFunc("/saluta", salutaHandler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	http.ListenAndServe(":"+port, nil)
}
