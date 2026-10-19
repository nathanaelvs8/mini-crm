// server is the production entry point: the API backed by PostgreSQL.
//
// Environment variables:
//
//	DATABASE_URL    required, e.g. postgres://crm:crm@localhost:5432/crm?sslmode=disable
//	JWT_SECRET      required, at least 16 characters, keep it secret
//	PORT            optional, default 8080
//	ALLOWED_ORIGIN  optional, the frontend's origin for CORS, e.g. https://my-crm.vercel.app
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crm/internal/httpapi"
	"crm/internal/store/pg"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dbURL := os.Getenv("DATABASE_URL")
	secret := os.Getenv("JWT_SECRET")
	if dbURL == "" {
		return errors.New("DATABASE_URL is not set")
	}
	if len(secret) < 16 {
		return errors.New("JWT_SECRET must be set and at least 16 characters long")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ctx is cancelled when the process gets Ctrl+C or a stop signal from the host.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := pg.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.New(st, secret, os.Getenv("ALLOWED_ORIGIN")),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx) // let running requests finish
	}()

	log.Printf("API listening on :%s", port)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
