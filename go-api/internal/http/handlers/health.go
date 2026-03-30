package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"legal-doc-intel/go-api/internal/health"
)

type readinessResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Error     string `json:"error,omitempty"`
}

type ReadinessProbe func(ctx context.Context) error

func Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, health.OK())
}

func Readiness(probe ReadinessProbe) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if probe != nil {
			if err := probe(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, readinessResponse{
					Status:    "not_ready",
					Timestamp: time.Now().UTC().Format(time.RFC3339),
					Error:     err.Error(),
				})
				return
			}
		}

		writeJSON(w, http.StatusOK, readinessResponse{
			Status:    "ready",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
