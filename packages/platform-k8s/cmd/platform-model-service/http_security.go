package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
)

func readyzHandler(
	logger *slog.Logger,
	statePool *pgxpool.Pool,
	natsJS jetstream.JetStream,
) http.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if statePool == nil {
			logger.Warn("readiness dependency missing", "dependency", "postgres", "error", "postgres pool is nil")
			writeReadyError(w, http.StatusServiceUnavailable, "postgres", fmt.Errorf("postgres pool is nil"))
			return
		}
		if err := statePool.Ping(ctx); err != nil {
			logger.Warn("readiness dependency check failed", "dependency", "postgres", "error", err)
			writeReadyError(w, http.StatusServiceUnavailable, "postgres", err)
			return
		}
		if natsJS != nil {
			if _, err := natsJS.AccountInfo(ctx); err != nil {
				logger.Warn("readiness dependency check failed", "dependency", "nats", "error", err)
				writeReadyError(w, http.StatusServiceUnavailable, "nats", err)
				return
			}
		}
		writeHTTPJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeReadyError(w http.ResponseWriter, status int, dependency string, err error) {
	writeHTTPJSON(w, status, map[string]string{
		"status":     "not_ready",
		"dependency": strings.TrimSpace(dependency),
		"error":      strings.TrimSpace(err.Error()),
	})
}

func writeHTTPJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
