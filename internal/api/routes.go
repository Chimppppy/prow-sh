package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/prow-sh/prow/internal/store"
)

// Deps wires HTTP handlers to backing services.
type Deps struct {
	Store *store.SQLiteStore
}

func jsonWrite(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	jsonWrite(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleVersion(w http.ResponseWriter, _ *http.Request) {
	jsonWrite(w, http.StatusOK, map[string]string{
		"version": Version,
		"commit":  Commit,
	})
}

func handleEvents(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if deps.Store == nil {
			http.Error(w, "store unavailable", http.StatusInternalServerError)
			return
		}
		events, err := deps.Store.ListEvents(ctx)
		if err != nil {
			http.Error(w, "failed to list events", http.StatusInternalServerError)
			return
		}
		jsonWrite(w, http.StatusOK, events)
	}
}

// Handler returns the full API mux with routes registered (before auth wrap).
func Handler(ctx context.Context, deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-ctx.Done():
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		default:
		}
		handleHealth(w, r)
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-ctx.Done():
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		default:
		}
		handleVersion(w, r)
	})
	mux.Handle("GET /v1/events", handleEvents(deps))
	return mux
}
