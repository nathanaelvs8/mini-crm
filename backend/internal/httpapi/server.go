// Package httpapi turns HTTP requests into calls on the store.
package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crm/internal/auth"
	"crm/internal/store"
)

type Server struct {
	store         store.Store
	secret        []byte
	allowedOrigin string
	tokenTTL      time.Duration
}

// New builds the router. allowedOrigin is the frontend's origin for CORS
// (leave empty when the frontend is served from the same origin or via the Vite proxy).
func New(st store.Store, secret string, allowedOrigin string) http.Handler {
	s := &Server{store: st, secret: []byte(secret), allowedOrigin: allowedOrigin, tokenTTL: 24 * time.Hour}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("GET /api/me", s.authed(s.me))

	mux.HandleFunc("GET /api/stats", s.authed(s.stats))
	mux.HandleFunc("GET /api/customers", s.authed(s.listCustomers))
	mux.HandleFunc("POST /api/customers", s.authed(s.createCustomer))
	mux.HandleFunc("GET /api/customers/{id}", s.authed(s.getCustomer))
	mux.HandleFunc("PUT /api/customers/{id}", s.authed(s.updateCustomer))
	mux.HandleFunc("DELETE /api/customers/{id}", s.authed(s.deleteCustomer))
	mux.HandleFunc("PATCH /api/customers/{id}/stage", s.authed(s.setStage))

	mux.HandleFunc("GET /api/customers/{id}/interactions", s.authed(s.listInteractions))
	mux.HandleFunc("POST /api/customers/{id}/interactions", s.authed(s.addInteraction))
	mux.HandleFunc("DELETE /api/interactions/{id}", s.authed(s.deleteInteraction))

	return s.logging(s.cors(mux))
}

// ---- middleware ----

type userHandler func(w http.ResponseWriter, r *http.Request, userID int64)

// authed wraps a handler so it only runs for a valid "Authorization: Bearer <token>".
func (s *Server) authed(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing token")
			return
		}
		userID, err := auth.ParseToken(s.secret, token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		next(w, r, userID)
	}
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.allowedOrigin != "" {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", s.allowedOrigin)
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// statusRecorder remembers the status code so the logger can print it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

// ---- small helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// readJSON decodes the body into v (max 1 MB). It writes the 400 itself and returns false on failure.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

// serverError logs the real error but only tells the client something generic.
func serverError(w http.ResponseWriter, ctx context.Context, err error) {
	if ctx.Err() != nil { // client went away, nothing to answer
		return
	}
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}
