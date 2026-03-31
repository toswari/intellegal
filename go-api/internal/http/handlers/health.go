package handlers

import (
	"context"
	"net/http"
	"time"
)

type DependencyProbe struct {
	Name  string
	Probe func(ctx context.Context) error
}

type dependencyStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type livenessResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type readinessResponse struct {
	Status       string                      `json:"status"`
	Timestamp    string                      `json:"timestamp"`
	Dependencies map[string]dependencyStatus `json:"dependencies,omitempty"`
}

func NewDependencyProbe(name string, probe func(ctx context.Context) error) DependencyProbe {
	return DependencyProbe{Name: name, Probe: probe}
}

func Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, livenessResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func Readiness(probes ...DependencyProbe) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dependencies := make(map[string]dependencyStatus)
		overallStatus := http.StatusOK
		readiness := "ready"

		for _, probe := range probes {
			if probe.Probe == nil {
				continue
			}

			if err := probe.Probe(r.Context()); err != nil {
				overallStatus = http.StatusServiceUnavailable
				readiness = "not_ready"
				dependencies[probe.Name] = dependencyStatus{
					Status: "down",
					Error:  err.Error(),
				}
				continue
			}

			dependencies[probe.Name] = dependencyStatus{Status: "up"}
		}

		writeJSON(w, overallStatus, readinessResponse{
			Status:       readiness,
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
			Dependencies: dependencies,
		})
	}
}
