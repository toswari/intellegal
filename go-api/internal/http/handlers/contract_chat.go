package handlers

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"legal-doc-intel/go-api/internal/ai"
	"legal-doc-intel/go-api/internal/db"
	"legal-doc-intel/go-api/internal/http/middleware"
	"legal-doc-intel/go-api/internal/ids"
	"legal-doc-intel/go-api/internal/models"
)

func (a *API) ChatContract(w http.ResponseWriter, r *http.Request) {
	contractID := pathParam(r, "contract_id")
	if !ids.IsUUID(contractID) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "contract_id must be a valid UUID", false, nil)
		return
	}

	var req contractChatRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	messages := make([]ai.ContractChatMessage, 0, len(req.Messages))
	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		content := strings.TrimSpace(message.Content)
		if role != "user" && role != "assistant" {
			writeError(w, http.StatusBadRequest, "invalid_argument", "messages.role must be one of: user, assistant", false, nil)
			return
		}
		if content == "" {
			writeError(w, http.StatusBadRequest, "invalid_argument", "messages.content is required", false, nil)
			return
		}
		messages = append(messages, ai.ContractChatMessage{
			Role:    role,
			Content: content,
		})
	}
	if len(messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_argument", "messages must contain at least one chat message", false, nil)
		return
	}

	documents, err := a.contractChatDocuments(r.Context(), contractID)
	if err != nil {
		switch err.Error() {
		case "contract not found":
			writeError(w, http.StatusNotFound, "not_found", err.Error(), false, nil)
		case "no contract files":
			writeError(w, http.StatusConflict, "contract_not_ready", "contract has no files yet", false, nil)
		case "no extracted text":
			writeError(w, http.StatusConflict, "contract_not_ready", "no extracted text is available for this contract yet", false, nil)
		default:
			if errors.Is(err, db.ErrNotConfigured) {
				writeError(w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", true, nil)
				return
			}
			writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
		}
		return
	}

	result, err := a.ai.ContractChat(context.Background(), ai.ContractChatRequest{
		JobID:      ids.NewUUID(),
		RequestID:  middleware.GetRequestID(r.Context()),
		ContractID: contractID,
		Messages:   messages,
		Documents:  documents,
	})
	if err != nil {
		a.logger.Error("contract chat failed", "contract_id", contractID, "error", err)
		writeError(w, http.StatusBadGateway, "contract_chat_unavailable", "contract chat is temporarily unavailable", true, nil)
		return
	}

	filenamesByDocumentID := make(map[string]string, len(documents))
	for _, doc := range documents {
		filenamesByDocumentID[doc.DocumentID] = doc.Filename
	}

	citations := make([]contractChatCitationResponse, 0, len(result.Citations))
	for _, citation := range result.Citations {
		snippet := strings.TrimSpace(citation.SnippetText)
		documentID := strings.TrimSpace(citation.DocumentID)
		if documentID == "" || snippet == "" {
			continue
		}
		citations = append(citations, contractChatCitationResponse{
			DocumentID:  documentID,
			Filename:    filenamesByDocumentID[documentID],
			SnippetText: snippet,
			Reason:      strings.TrimSpace(citation.Reason),
		})
	}

	writeJSON(w, http.StatusOK, contractChatResponse{
		Answer:    strings.TrimSpace(result.Answer),
		Citations: citations,
	})
}

func (a *API) contractChatDocuments(ctx context.Context, contractID string) ([]ai.ContractChatDocument, error) {
	if a.contractsModel == nil {
		return nil, db.ErrNotConfigured
	}

	item, ok, err := a.contractsModel.Get(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errString("contract not found")
	}
	if len(item.Files) == 0 {
		return nil, errString("no contract files")
	}

	documents := make([]ai.ContractChatDocument, 0, len(item.Files))
	for _, doc := range item.Files {
		text := strings.TrimSpace(doc.ExtractedText)
		if text == "" {
			continue
		}
		documents = append(documents, ai.ContractChatDocument{
			DocumentID: doc.ID,
			Filename:   doc.Filename,
			Text:       text,
		})
	}
	if len(documents) == 0 {
		return nil, errString("no extracted text")
	}
	return documents, nil
}

type errString string

func (e errString) Error() string {
	return string(e)
}

type searchChatCandidate struct {
	DocumentID  string
	ContractID  string
	Filename    string
	SnippetText string
	Score       float64
}

func (a *API) ChatContractSearch(w http.ResponseWriter, r *http.Request) {
	var req contractSearchChatRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	messages := make([]ai.ContractChatMessage, 0, len(req.Messages))
	latestQuestion := ""
	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		content := strings.TrimSpace(message.Content)
		if role != "user" && role != "assistant" {
			writeError(w, http.StatusBadRequest, "invalid_argument", "messages.role must be one of: user, assistant", false, nil)
			return
		}
		if content == "" {
			writeError(w, http.StatusBadRequest, "invalid_argument", "messages.content is required", false, nil)
			return
		}
		messages = append(messages, ai.ContractChatMessage{
			Role:    role,
			Content: content,
		})
		if role == "user" {
			latestQuestion = content
		}
	}
	if len(messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_argument", "messages must contain at least one chat message", false, nil)
		return
	}
	if latestQuestion == "" {
		writeError(w, http.StatusBadRequest, "invalid_argument", "messages must include at least one user message", false, nil)
		return
	}

	limit := req.Limit
	if limit == 0 {
		limit = 3
	}
	if limit < 1 || limit > 5 {
		writeError(w, http.StatusBadRequest, "invalid_argument", "limit must be between 1 and 5", false, nil)
		return
	}

	var resolvedDocIDs []string
	var err error
	if len(req.DocumentIDs) > 0 {
		resolvedDocIDs, err = a.resolveDocumentIDs(req.DocumentIDs)
		if err != nil {
			handleCreateCheckError(w, err)
			return
		}
	} else {
		if a.documentsModel == nil {
			writeError(w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", true, nil)
			return
		}
		resolvedDocIDs, err = a.documentsModel.ListIDs(r.Context())
		if err != nil {
			if errors.Is(err, db.ErrNotConfigured) {
				writeError(w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", true, nil)
				return
			}
			a.logger.Error("list document ids for search chat failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to load documents", true, nil)
			return
		}
	}

	if len(resolvedDocIDs) == 0 {
		writeJSON(w, http.StatusOK, contractChatResponse{
			Answer:    "No indexed contracts are available for search yet.",
			Citations: []contractChatCitationResponse{},
		})
		return
	}

	documentsByID, err := a.documentsModel.GetByIDs(r.Context(), resolvedDocIDs)
	if err != nil {
		if errors.Is(err, db.ErrNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", true, nil)
			return
		}
		a.logger.Error("load documents for search chat scope failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to load documents", true, nil)
		return
	}

	candidates, err := a.searchChatCandidates(
		r.Context(),
		latestQuestion,
		resolvedDocIDs,
		documentsByID,
		limit,
		middleware.GetRequestID(r.Context()),
	)
	if err != nil {
		a.logger.Error("contract search chat retrieval failed", "error", err)
		writeError(w, http.StatusBadGateway, "search_unavailable", "search is temporarily unavailable", true, nil)
		return
	}
	if len(candidates) == 0 {
		writeJSON(w, http.StatusOK, contractChatResponse{
			Answer:    "I could not find matching contract text to investigate for that question.",
			Citations: []contractChatCitationResponse{},
		})
		return
	}

	searchDocuments, err := a.expandSearchChatCandidates(r.Context(), candidates, documentsByID)
	if err != nil {
		switch err.Error() {
		case "contract not found", "no contract files", "no extracted text":
			writeJSON(w, http.StatusOK, contractChatResponse{
				Answer:    "I found possible matches, but there was not enough extracted contract text to dig deeper.",
				Citations: []contractChatCitationResponse{},
			})
		default:
			if errors.Is(err, db.ErrNotConfigured) {
				writeError(w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", true, nil)
				return
			}
			a.logger.Error("expand documents for search chat failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to load expanded contracts", true, nil)
		}
		return
	}
	if len(searchDocuments) == 0 {
		writeJSON(w, http.StatusOK, contractChatResponse{
			Answer:    "I found possible matches, but there was not enough extracted contract text to inspect further.",
			Citations: []contractChatCitationResponse{},
		})
		return
	}

	chatResult, err := a.ai.ContractChat(context.Background(), ai.ContractChatRequest{
		JobID:      ids.NewUUID(),
		RequestID:  middleware.GetRequestID(r.Context()),
		ContractID: "search-results",
		Messages:   messages,
		Documents:  searchDocuments,
	})
	if err != nil {
		a.logger.Error("contract search chat failed", "error", err)
		writeError(w, http.StatusBadGateway, "contract_chat_unavailable", "contract chat is temporarily unavailable", true, nil)
		return
	}

	filenamesByDocumentID := make(map[string]string, len(documentsByID))
	contractIDsByDocumentID := make(map[string]string, len(documentsByID))
	for documentID, doc := range documentsByID {
		filenamesByDocumentID[documentID] = doc.Filename
		contractIDsByDocumentID[documentID] = doc.ContractID
	}

	citations := make([]contractChatCitationResponse, 0, len(chatResult.Citations))
	for _, citation := range chatResult.Citations {
		snippet := strings.TrimSpace(citation.SnippetText)
		documentID := strings.TrimSpace(citation.DocumentID)
		if documentID == "" || snippet == "" {
			continue
		}
		citations = append(citations, contractChatCitationResponse{
			DocumentID:  documentID,
			ContractID:  contractIDsByDocumentID[documentID],
			Filename:    filenamesByDocumentID[documentID],
			SnippetText: snippet,
			Reason:      strings.TrimSpace(citation.Reason),
		})
	}

	slices.SortStableFunc(citations, func(left, right contractChatCitationResponse) int {
		switch {
		case left.Filename < right.Filename:
			return -1
		case left.Filename > right.Filename:
			return 1
		default:
			return 0
		}
	})

	results := a.buildSearchChatResults(r.Context(), candidates)

	writeJSON(w, http.StatusOK, contractChatResponse{
		Answer:    strings.TrimSpace(chatResult.Answer),
		Citations: citations,
		Results:   results,
	})
}

func (a *API) searchChatCandidates(
	ctx context.Context,
	question string,
	documentIDs []string,
	documentsByID map[string]models.DocumentRow,
	limit int,
	requestID string,
) ([]searchChatCandidate, error) {
	strategies := []struct {
		name  string
		boost float64
	}{
		{name: "strict", boost: 0.08},
		{name: "semantic", boost: 0},
	}
	bestByGroup := make(map[string]searchChatCandidate)
	order := make([]string, 0)

	for _, strategy := range strategies {
		result, err := a.ai.SearchSections(ctx, ai.SearchSectionsRequest{
			JobID:       ids.NewUUID(),
			RequestID:   requestID,
			QueryText:   question,
			DocumentIDs: documentIDs,
			Limit:       min(12, max(6, limit*4)),
			Strategy:    strategy.name,
			ResultMode:  "sections",
		})
		if err != nil {
			return nil, err
		}
		for _, item := range result.Items {
			doc, ok := documentsByID[item.DocumentID]
			if !ok {
				continue
			}
			groupKey := strings.TrimSpace(doc.ContractID)
			if groupKey == "" {
				groupKey = item.DocumentID
			}
			candidate := searchChatCandidate{
				DocumentID:  item.DocumentID,
				ContractID:  doc.ContractID,
				Filename:    doc.Filename,
				SnippetText: item.SnippetText,
				Score:       item.Score + strategy.boost,
			}
			current, exists := bestByGroup[groupKey]
			if !exists {
				bestByGroup[groupKey] = candidate
				order = append(order, groupKey)
				continue
			}
			if candidate.Score > current.Score {
				bestByGroup[groupKey] = candidate
			}
		}
	}

	slices.SortStableFunc(order, func(left, right string) int {
		aCandidate := bestByGroup[left]
		bCandidate := bestByGroup[right]
		switch {
		case aCandidate.Score > bCandidate.Score:
			return -1
		case aCandidate.Score < bCandidate.Score:
			return 1
		case aCandidate.ContractID < bCandidate.ContractID:
			return -1
		case aCandidate.ContractID > bCandidate.ContractID:
			return 1
		case aCandidate.DocumentID < bCandidate.DocumentID:
			return -1
		case aCandidate.DocumentID > bCandidate.DocumentID:
			return 1
		default:
			return 0
		}
	})

	candidates := make([]searchChatCandidate, 0, min(limit, len(order)))
	for _, key := range order {
		candidates = append(candidates, bestByGroup[key])
		if len(candidates) == limit {
			break
		}
	}
	return candidates, nil
}

func (a *API) buildSearchChatResults(
	ctx context.Context,
	candidates []searchChatCandidate,
) []contractSearchChatResultItemResponse {
	results := make([]contractSearchChatResultItemResponse, 0, len(candidates))
	contractNames := make(map[string]string, len(candidates))

	for _, candidate := range candidates {
		contractID := strings.TrimSpace(candidate.ContractID)
		if contractID != "" {
			if _, ok := contractNames[contractID]; !ok && a.contractsModel != nil {
				item, found, err := a.contractsModel.Get(ctx, contractID)
				if err == nil && found {
					contractNames[contractID] = strings.TrimSpace(item.Name)
				}
			}
		}

		results = append(results, contractSearchChatResultItemResponse{
			ContractID:   contractID,
			DocumentID:   candidate.DocumentID,
			ContractName: contractNames[contractID],
			Filename:     strings.TrimSpace(candidate.Filename),
			Score:        candidate.Score,
			SnippetText:  strings.TrimSpace(candidate.SnippetText),
		})
	}

	return results
}

func (a *API) expandSearchChatCandidates(
	ctx context.Context,
	candidates []searchChatCandidate,
	documentsByID map[string]models.DocumentRow,
) ([]ai.ContractChatDocument, error) {
	expanded := make([]ai.ContractChatDocument, 0, len(candidates)*2)
	seenDocumentIDs := make(map[string]struct{})

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ContractID) != "" {
			documents, err := a.contractChatDocuments(ctx, candidate.ContractID)
			if err != nil {
				continue
			}
			for _, document := range documents {
				if _, seen := seenDocumentIDs[document.DocumentID]; seen {
					continue
				}
				seenDocumentIDs[document.DocumentID] = struct{}{}
				expanded = append(expanded, document)
			}
			continue
		}

		doc, ok := documentsByID[candidate.DocumentID]
		if !ok {
			continue
		}
		text := strings.TrimSpace(doc.ExtractedText)
		if text == "" {
			continue
		}
		if _, seen := seenDocumentIDs[doc.ID]; seen {
			continue
		}
		seenDocumentIDs[doc.ID] = struct{}{}
		expanded = append(expanded, ai.ContractChatDocument{
			DocumentID: doc.ID,
			Filename:   doc.Filename,
			Text:       text,
		})
	}

	return expanded, nil
}
