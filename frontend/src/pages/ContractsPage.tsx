import { type FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  apiClient,
  type ContractChatCitation,
  type ContractChatMessage,
  type ContractSearchChatResultItem,
  type ContractResponse,
  type DocumentResponse,
  type DocumentStatus,
} from "../api/client";
import { formatEuropeanDateTime } from "../app/datetime";

type Filters = {
  status: "all" | DocumentStatus;
  sourceType: "all" | "repository" | "upload" | "api";
  query: string;
  tagsInput: string;
};

type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  citations?: ContractChatCitation[];
  results?: ContractSearchChatResultItem[];
};

function RobotIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <rect x="5" y="7" width="14" height="11" rx="3" />
      <path d="M12 3v4" />
      <path d="M8 18v2" />
      <path d="M16 18v2" />
      <path d="M3 11h2" />
      <path d="M19 11h2" />
      <circle cx="9.5" cy="12" r="1.1" />
      <circle cx="14.5" cy="12" r="1.1" />
      <path d="M9 15h6" />
    </svg>
  );
}

function buildChatPayload(messages: ChatMessage[]): ContractChatMessage[] {
  return messages.map((message) => ({
    role: message.role,
    content: message.content,
  }));
}

export function ContractsPage() {
  const navigate = useNavigate();
  const [filters, setFilters] = useState<Filters>({
    status: "all",
    sourceType: "all",
    query: "",
    tagsInput: "",
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [contracts, setContracts] = useState<ContractResponse[]>([]);
  const [documents, setDocuments] = useState<DocumentResponse[]>([]);
  const [selectedContractIds, setSelectedContractIds] = useState<string[]>([]);
  const [deletingContractId, setDeletingContractId] = useState<string | null>(
    null,
  );
  const [chatOpen, setChatOpen] = useState(false);
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [chatInput, setChatInput] = useState("");
  const [chatLoading, setChatLoading] = useState(false);
  const [chatError, setChatError] = useState<string | null>(null);
  const [assistantFilteredContractIds, setAssistantFilteredContractIds] =
    useState<string[]>([]);
  const [expandedResultMessageIds, setExpandedResultMessageIds] = useState<
    string[]
  >([]);
  const chatBodyRef = useRef<HTMLDivElement | null>(null);
  const selectedTags = useMemo(
    () =>
      Array.from(
        new Set(
          filters.tagsInput
            .split(",")
            .map((tag) => tag.trim())
            .filter((tag) => tag.length > 0),
        ),
      ),
    [filters.tagsInput],
  );

  const loadDocuments = async () => {
    setLoading(true);
    setError(null);
    try {
      const contractsResponse = await apiClient.listContracts({
        limit: 200,
        offset: 0,
      });
      const response = await apiClient.listDocuments({
        status: filters.status === "all" ? undefined : filters.status,
        source_type:
          filters.sourceType === "all" ? undefined : filters.sourceType,
        limit: 200,
        offset: 0,
      });
      setContracts(contractsResponse.items);
      setDocuments(response.items);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load documents.";
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadDocuments();
  }, [filters.status, filters.sourceType, selectedTags]);

  useEffect(() => {
    if (!chatBodyRef.current) {
      return;
    }
    chatBodyRef.current.scrollTop = chatBodyRef.current.scrollHeight;
  }, [chatLoading, chatMessages]);

  const filteredContracts = useMemo(() => {
    const query = filters.query.trim().toLowerCase();
    const selectedTagSet = new Set(selectedTags.map((tag) => tag.toLowerCase()));
    const matchingContracts = new Set(
      documents
        .map((document) => document.contract_id)
        .filter((id): id is string => Boolean(id)),
    );

    return contracts.filter((contract) => {
      if (
        assistantFilteredContractIds.length > 0 &&
        !assistantFilteredContractIds.includes(contract.id)
      ) {
        return false;
      }

      if (
        (filters.status !== "all" || filters.sourceType !== "all") &&
        !matchingContracts.has(contract.id)
      ) {
        return false;
      }

      if (selectedTagSet.size > 0) {
        const hasMatchingTag = (contract.tags ?? []).some((tag) =>
          selectedTagSet.has(tag.toLowerCase()),
        );
        if (!hasMatchingTag) {
          return false;
        }
      }

      if (query.length === 0) {
        return true;
      }

      return (
        contract.name.toLowerCase().includes(query) ||
        contract.id.toLowerCase().includes(query) ||
        (contract.source_ref ?? "").toLowerCase().includes(query) ||
        (contract.tags ?? []).some((tag) => tag.toLowerCase().includes(query))
      );
    });
  }, [
    contracts,
    documents,
    filters.query,
    filters.sourceType,
    filters.status,
    selectedTags,
    assistantFilteredContractIds,
  ]);

  const representativeDocumentByContract = useMemo(() => {
    const map = new Map<string, DocumentResponse>();
    for (const document of documents) {
      if (!document.contract_id) {
        continue;
      }
      if (!map.has(document.contract_id)) {
        map.set(document.contract_id, document);
      }
    }
    return map;
  }, [documents]);

  const visibleContractIds = useMemo(
    () => new Set(filteredContracts.map((contract) => contract.id)),
    [filteredContracts],
  );
  const selectedVisibleCount = useMemo(
    () =>
      filteredContracts.filter((contract) =>
        selectedContractIds.includes(contract.id),
      ).length,
    [filteredContracts, selectedContractIds],
  );
  const allVisibleSelected =
    filteredContracts.length > 0 &&
    selectedVisibleCount === filteredContracts.length;
  const selectedDocumentIds = useMemo(
    () =>
      documents
        .filter(
          (document) =>
            document.contract_id &&
            selectedContractIds.includes(document.contract_id),
        )
        .map((document) => document.id),
    [documents, selectedContractIds],
  );
  const selectableContractCount = useMemo(
    () =>
      contracts.filter((contract) =>
        representativeDocumentByContract.has(contract.id),
      ).length,
    [contracts, representativeDocumentByContract],
  );
  const unfilteredView =
    filters.status === "all" &&
    filters.sourceType === "all" &&
    selectedTags.length === 0 &&
    filters.query.trim().length === 0;
  const allContractsSelected =
    unfilteredView &&
    selectableContractCount > 0 &&
    selectedContractIds.length === selectableContractCount;
  const chatScopeLabel =
    selectedDocumentIds.length > 0
      ? `${selectedContractIds.length} selected contract${
          selectedContractIds.length === 1 ? "" : "s"
        }`
      : "all indexed contracts";
  const assistantFilterCount = assistantFilteredContractIds.length;

  useEffect(() => {
    setSelectedContractIds((prev) =>
      prev.filter((id) => visibleContractIds.has(id)),
    );
  }, [visibleContractIds]);

  const compareSelected = () => {
    if (selectedContractIds.length !== 2) {
      setError("Select exactly two contracts to compare.");
      return;
    }

    const [leftContractId, rightContractId] = selectedContractIds;
    const leftDocument = representativeDocumentByContract.get(leftContractId);
    const rightDocument = representativeDocumentByContract.get(rightContractId);
    if (!leftDocument || !rightDocument) {
      setError(
        "Cannot compare selected contracts because one of them has no comparable file.",
      );
      return;
    }

    setError(null);
    const params = new URLSearchParams({
      left: leftDocument.id,
      right: rightDocument.id,
    });
    navigate(`/contracts/compare?${params.toString()}`);
  };

  const toggleCompareSelection = (contractId: string) => {
    setSelectedContractIds((prev) => {
      if (prev.includes(contractId)) {
        return prev.filter((id) => id !== contractId);
      }
      return [...prev, contractId];
    });
  };

  const toggleSelectAllVisible = () => {
    setSelectedContractIds((prev) => {
      if (allVisibleSelected) {
        return prev.filter((id) => !visibleContractIds.has(id));
      }

      const next = new Set(prev);
      for (const contract of filteredContracts) {
        if (representativeDocumentByContract.has(contract.id)) {
          next.add(contract.id);
        }
      }
      return Array.from(next);
    });
  };

  const startGuidelineForSelection = () => {
    const params = new URLSearchParams();

    if (allContractsSelected) {
      params.set("scope", "all");
    } else {
      params.set("scope", "selected");
      for (const documentId of selectedDocumentIds) {
        params.append("documentId", documentId);
      }
    }

    navigate(`/guidelines/run?${params.toString()}`);
  };

  const handleDelete = async (contract: ContractResponse) => {
    const confirmed = window.confirm(
      `Delete "${contract.name}" permanently?\n\nThis will hard-delete all files in the contract and related data.`,
    );
    if (!confirmed) {
      return;
    }

    setError(null);
    setDeletingContractId(contract.id);
    try {
      await apiClient.deleteContract(contract.id);
      setContracts((prev) => prev.filter((item) => item.id !== contract.id));
      setDocuments((prev) =>
        prev.filter((item) => item.contract_id !== contract.id),
      );
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to delete document.";
      setError(message);
    } finally {
      setDeletingContractId(null);
    }
  };

  const submitSearchChat = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (chatLoading) {
      return;
    }

    const question = chatInput.trim();
    if (!question) {
      return;
    }

    const nextMessages: ChatMessage[] = [
      ...chatMessages,
      {
        id: `user-${Date.now()}`,
        role: "user",
        content: question,
      },
    ];

    setChatMessages(nextMessages);
    setChatInput("");
    setChatLoading(true);
    setChatError(null);

    try {
      const response = await apiClient.chatWithContractSearch({
        messages: buildChatPayload(nextMessages),
        document_ids:
          selectedDocumentIds.length > 0 ? selectedDocumentIds : undefined,
        limit: 3,
      });
      setChatMessages([
        ...nextMessages,
        {
          id: `assistant-${Date.now()}`,
          role: "assistant",
          content: response.answer,
          citations: response.citations,
          results: response.results,
        },
      ]);
    } catch (err) {
      setChatError(
        err instanceof Error
          ? err.message
          : "Failed to ask the contracts assistant.",
      );
    } finally {
      setChatLoading(false);
    }
  };

  const toggleResultBlock = (messageId: string) => {
    setExpandedResultMessageIds((current) =>
      current.includes(messageId)
        ? current.filter((id) => id !== messageId)
        : [...current, messageId],
    );
  };

  const applyAssistantResultsToList = (results: ContractSearchChatResultItem[]) => {
    const nextIds = Array.from(
      new Set(
        results
          .map((result) => result.contract_id)
          .filter((contractId): contractId is string => Boolean(contractId)),
      ),
    );
    setAssistantFilteredContractIds(nextIds);
  };

  return (
    <section className="page">
      <header className="page-header">
        <h2>Contracts</h2>
        <div className="page-actions">
          {selectedContractIds.length > 0 ? (
            <button
              type="button"
              onClick={startGuidelineForSelection}
              disabled={selectedDocumentIds.length === 0}
            >
              Check Guidelines
            </button>
          ) : null}
          <button
            type="button"
            className="secondary"
            onClick={compareSelected}
            disabled={selectedContractIds.length !== 2}
          >
            Compare Selected
          </button>
          <Link to="/contracts/import" className="button-link secondary">
            Batch Import
          </Link>
          <Link to="/contracts/new" className="button-link">
            New Contract
          </Link>
        </div>
      </header>

      <section className="contracts-list">
        <div className="filter-row">
          <label>
            Status
            <select
              value={filters.status}
              onChange={(event) =>
                setFilters((prev) => ({
                  ...prev,
                  status: event.target.value as Filters["status"],
                }))
              }
            >
              <option value="all">all</option>
              <option value="ingested">ingested</option>
              <option value="processing">processing</option>
              <option value="indexed">indexed</option>
              <option value="failed">failed</option>
            </select>
          </label>
          <label>
            Source
            <select
              value={filters.sourceType}
              onChange={(event) =>
                setFilters((prev) => ({
                  ...prev,
                  sourceType: event.target.value as Filters["sourceType"],
                }))
              }
            >
              <option value="all">all</option>
              <option value="upload">upload</option>
              <option value="repository">repository</option>
              <option value="api">api</option>
            </select>
          </label>
          <label>
            Search
            <input
              value={filters.query}
              onChange={(event) =>
                setFilters((prev) => ({ ...prev, query: event.target.value }))
              }
              placeholder="filename or id"
            />
          </label>
          <label>
            Tags
            <input
              value={filters.tagsInput}
              onChange={(event) =>
                setFilters((prev) => ({
                  ...prev,
                  tagsInput: event.target.value,
                }))
              }
              placeholder="filter by tags (comma-separated)"
            />
          </label>
        </div>

        {assistantFilterCount > 0 ? (
          <p className="muted">
            Assistant filter active: {assistantFilterCount} contract
            {assistantFilterCount === 1 ? "" : "s"} shown from the latest
            applied result set.{" "}
            <button
              type="button"
              className="secondary"
              onClick={() => setAssistantFilteredContractIds([])}
            >
              Clear Assistant Filter
            </button>
          </p>
        ) : null}

        {error ? <p className="error-text">{error}</p> : null}
        {loading ? <p className="muted">Loading documents...</p> : null}
        {!loading && filteredContracts.length === 0 ? (
          <p className="muted">No contracts found.</p>
        ) : null}

        {filteredContracts.length > 0 ? (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th aria-label="Selection">
                    <input
                      type="checkbox"
                      aria-label={
                        allVisibleSelected
                          ? "Deselect visible contracts"
                          : "Select visible contracts"
                      }
                      checked={allVisibleSelected}
                      onChange={toggleSelectAllVisible}
                    />
                  </th>
                  <th>Name</th>
                  <th>Files</th>
                  <th>Tags</th>
                  <th>Created</th>
                  <th aria-label="Delete actions"></th>
                </tr>
              </thead>
              <tbody>
                {filteredContracts.map((contract) => (
                  <tr key={contract.id}>
                    <td>
                      <input
                        type="checkbox"
                        aria-label={`Select ${contract.name}`}
                        checked={selectedContractIds.includes(contract.id)}
                        onChange={() => toggleCompareSelection(contract.id)}
                        disabled={!representativeDocumentByContract.has(contract.id)}
                      />
                    </td>
                    <td>
                      <strong>
                        <Link
                          to={`/contracts/${encodeURIComponent(contract.id)}/edit`}
                        >
                          {contract.name}
                        </Link>
                      </strong>
                    </td>
                    <td>{contract.file_count}</td>
                    <td>
                      {(contract.tags ?? []).length > 0 ? (
                        <div className="tag-list">
                          {(contract.tags ?? []).map((tag) => (
                            <span
                              key={`${contract.id}-${tag}`}
                              className="chip chip-tag"
                            >
                              {tag}
                            </span>
                          ))}
                        </div>
                      ) : (
                        <span className="muted">-</span>
                      )}
                    </td>
                    <td>
                      <span className="contract-created-at">
                        {formatEuropeanDateTime(contract.created_at)}
                      </span>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="danger contract-delete-button"
                        disabled={deletingContractId !== null}
                        onClick={() => void handleDelete(contract)}
                      >
                        {deletingContractId === contract.id
                          ? "Deleting..."
                          : "Delete"}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>

      <div className="contract-chat-dock">
        {chatOpen ? (
          <section
            className="contract-chat-panel"
            aria-label="Contracts assistant"
          >
            <header className="contract-chat-header">
              <div className="contract-chat-title">
                <span className="contract-chat-title-icon" aria-hidden="true">
                  <RobotIcon />
                </span>
                <div>
                  <h3>Contracts Assistant</h3>
                  <p className="muted">Search scope: {chatScopeLabel}</p>
                </div>
              </div>
              <div className="contract-chat-header-actions">
                <button
                  type="button"
                  className="secondary contract-chat-close-button"
                  onClick={() => setChatOpen(false)}
                  aria-label="Close contracts assistant"
                >
                  Close
                </button>
              </div>
            </header>
            <div className="contract-chat-body" ref={chatBodyRef}>
              {chatMessages.length === 0 ? (
                <div className="contract-chat-message contract-chat-message-assistant">
                  Ask about obligations, payment language, renewal terms, or patterns across contracts. I will search, pick the best leads, and open the strongest matching contracts before answering.
                </div>
              ) : null}
              {chatMessages.map((message) => (
                <div
                  key={message.id}
                  className={`contract-chat-message ${
                    message.role === "user"
                      ? "contract-chat-message-user"
                      : "contract-chat-message-assistant"
                  }`}
                >
                  <p>{message.content}</p>
                  {message.role === "assistant" &&
                  message.citations &&
                  message.citations.length > 0 ? (
                    <div className="contract-chat-citations">
                      {message.citations.map((citation, index) =>
                        citation.contract_id ? (
                          <Link
                            key={`${message.id}-${citation.document_id}-${index}`}
                            className={`button-link secondary contract-chat-citation-pill contract-chat-citation-pill-${index % 4}`}
                            to={`/contracts/${encodeURIComponent(citation.contract_id)}/edit`}
                            title={citation.snippet_text}
                          >
                            {citation.filename || "Open contract"}:{" "}
                            {citation.reason || "View support"}
                          </Link>
                        ) : (
                          <span
                            key={`${message.id}-${citation.document_id}-${index}`}
                            className={`contract-chat-citation-pill contract-chat-citation-pill-${index % 4}`}
                            title={citation.snippet_text}
                          >
                            {citation.filename || "Search result"}:{" "}
                            {citation.reason || "Used in answer"}
                          </span>
                        ),
                      )}
                    </div>
                  ) : null}
                  {message.role === "assistant" &&
                  message.results &&
                  message.results.length > 0 ? (
                    <div className="contract-chat-actions">
                      <p className="muted">
                        I found {message.results.length} result
                        {message.results.length === 1 ? "" : "s"}.
                      </p>
                      <div className="page-actions">
                        <button
                          type="button"
                          className="secondary"
                          onClick={() => toggleResultBlock(message.id)}
                        >
                          {expandedResultMessageIds.includes(message.id)
                            ? "Hide Results"
                            : "Show Results"}
                        </button>
                        <button
                          type="button"
                          className="secondary"
                          onClick={() =>
                            applyAssistantResultsToList(message.results ?? [])
                          }
                        >
                          Filter List By IDs
                        </button>
                      </div>
                      {expandedResultMessageIds.includes(message.id) ? (
                        <div className="contract-chat-citations">
                          {message.results.map((result, index) => (
                            <div
                              key={`${message.id}-${result.document_id}-${index}`}
                              className={`contract-chat-citation-pill contract-chat-citation-pill-${index % 4}`}
                            >
                              <strong>
                                {result.contract_name ||
                                  result.filename ||
                                  result.contract_id ||
                                  result.document_id}
                              </strong>
                              <span>
                                {" "}
                                · Score {result.score.toFixed(3)}
                              </span>
                              {result.contract_id ? (
                                <>
                                  {" "}
                                  ·{" "}
                                  <Link
                                    className="button-link secondary"
                                    to={`/contracts/${encodeURIComponent(result.contract_id)}/edit`}
                                  >
                                    Open
                                  </Link>
                                </>
                              ) : null}
                              {result.snippet_text ? (
                                <p>{result.snippet_text}</p>
                              ) : null}
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  ) : null}
                </div>
              ))}
              {chatLoading ? (
                <div className="contract-chat-message contract-chat-message-assistant">
                  Searching contracts and drafting an answer...
                </div>
              ) : null}
            </div>
            <form className="contract-chat-form" onSubmit={submitSearchChat}>
              <label className="sr-only" htmlFor="contracts-chat-input">
                Ask a question about the contracts list
              </label>
              <div className="contract-chat-compose">
                <textarea
                  id="contracts-chat-input"
                  value={chatInput}
                  onChange={(event) => setChatInput(event.target.value)}
                  placeholder="Which contracts mention late fees, auto-renewal, or termination for convenience?"
                  rows={3}
                />
                <button
                  type="submit"
                  disabled={chatLoading || chatInput.trim().length === 0}
                >
                  {chatLoading ? "Asking..." : "Ask"}
                </button>
              </div>
              <div className="contract-chat-actions">
                {chatError ? <p className="error-text">{chatError}</p> : null}
                {!chatError ? (
                  <p className="muted">
                    {selectedDocumentIds.length > 0
                      ? "Only the selected contracts are searched."
                      : "No contracts selected, so the assistant searches across all indexed contracts."}
                  </p>
                ) : null}
              </div>
            </form>
          </section>
        ) : null}
        {!chatOpen ? (
          <button
            type="button"
            className="contract-chat-toggle"
            aria-label="Open contracts assistant"
            onClick={() => setChatOpen(true)}
          >
            <span className="contract-chat-toggle-icon">
              <RobotIcon />
            </span>
            <span>Ask AI</span>
          </button>
        ) : null}
      </div>
    </section>
  );
}
