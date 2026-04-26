package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prow-sh/prow/internal/store"
)

// Version is the prowd API version string (override via -ldflags for releases).
var Version = "0.0.0-dev"

// Commit is optional build metadata (override via -ldflags for releases).
var Commit = "dev"

// ServerConfig is minimal Phase 0A HTTP configuration.
type ServerConfig struct {
	BindAddr   string
	LabToken   string
	SQLiteDSN string
}

// Run starts the HTTP server until ctx is cancelled or the server stops.
func Run(ctx context.Context, cfg ServerConfig) error {
	if cfg.BindAddr == "" {
		return errors.New("api: empty bind address")
	}
	if cfg.LabToken == "" {
		return errors.New("api: empty lab token")
	}
	if cfg.SQLiteDSN == "" {
		return errors.New("api: empty sqlite dsn")
	}

	st, err := store.OpenSQLite(cfg.SQLiteDSN)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer st.Close()

	deps := Deps{Store: st}
	base := Handler(ctx, deps)
	h := RequireBearer(cfg.LabToken)(base)

	srv := &http.Server{
		Addr:              cfg.BindAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
