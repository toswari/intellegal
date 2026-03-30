package router

import (
	"context"
	"log/slog"
	"net/http"

	"legal-doc-intel/go-api/internal/http/handlers"
	"legal-doc-intel/go-api/internal/http/middleware"
)

func New(
	logger *slog.Logger,
	api *handlers.API,
	readinessProbe func(context.Context) error,
	corsAllowedOrigins []string,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", handlers.Health)
	mux.HandleFunc("GET /api/v1/readiness", handlers.Readiness(readinessProbe))

	mux.HandleFunc("POST /api/v1/documents", api.CreateDocument)
	mux.HandleFunc("GET /api/v1/documents", api.ListDocuments)
	mux.HandleFunc("GET /api/v1/documents/{document_id}", api.GetDocument)
	mux.HandleFunc("GET /api/v1/documents/{document_id}/text", api.GetDocumentText)
	mux.HandleFunc("DELETE /api/v1/documents/{document_id}", api.DeleteDocument)
	mux.HandleFunc("POST /api/v1/contracts/search", api.SearchContracts)
	mux.HandleFunc("POST /api/v1/contracts", api.CreateContract)
	mux.HandleFunc("GET /api/v1/contracts", api.ListContracts)
	mux.HandleFunc("GET /api/v1/contracts/{contract_id}", api.GetContract)
	mux.HandleFunc("PATCH /api/v1/contracts/{contract_id}", api.UpdateContract)
	mux.HandleFunc("DELETE /api/v1/contracts/{contract_id}", api.DeleteContract)
	mux.HandleFunc("POST /api/v1/contracts/{contract_id}/files", api.AddContractFile)
	mux.HandleFunc("PATCH /api/v1/contracts/{contract_id}/files/order", api.ReorderContractFiles)

	mux.HandleFunc("POST /api/v1/checks/clause-presence", api.CreateClauseCheck)
	mux.HandleFunc("POST /api/v1/checks/company-name", api.CreateCompanyNameCheck)
	mux.HandleFunc("GET /api/v1/checks/{check_id}", api.GetCheck)
	mux.HandleFunc("GET /api/v1/checks/{check_id}/results", api.GetCheckResults)

	var handler http.Handler = mux
	handler = middleware.CORS(handler, corsAllowedOrigins)
	handler = middleware.RequestID(handler)
	handler = middleware.AccessLog(logger, handler)

	return handler
}
