// devserver runs the API with the in-memory store: no database needed.
// Data disappears when you stop it. Use cmd/server for the real PostgreSQL version.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"crm/internal/httpapi"
	"crm/internal/store"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	handler := httpapi.New(store.NewMemory(), "dev-only-secret-change-me", "http://localhost:5173")

	srv := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("devserver (in-memory data) listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}
