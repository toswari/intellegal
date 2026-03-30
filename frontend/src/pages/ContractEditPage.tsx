import { type DragEvent, type FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { apiClient, type ContractResponse, type DocumentResponse, type DocumentTextResponse } from "../api/client";
import { formatEuropeanDateTime } from "../app/datetime";

async function toBase64(file: File): Promise<string> {
  const buffer = await file.arrayBuffer();
  let binary = "";
  const bytes = new Uint8Array(buffer);
  for (const value of bytes) {
    binary += String.fromCharCode(value);
  }
  return btoa(binary);
}

export function ContractEditPage() {
  const { contractId } = useParams<{ contractId: string }>();
  const [contract, setContract] = useState<ContractResponse | null>(null);
  const [files, setFiles] = useState<DocumentResponse[]>([]);
  const [draggingId, setDraggingId] = useState<string | null>(null);
  const [savingOrder, setSavingOrder] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [newFiles, setNewFiles] = useState<File[]>([]);
  const [isUploadDragOver, setIsUploadDragOver] = useState(false);
  const [contractNameInput, setContractNameInput] = useState("");
  const [contractTagsInput, setContractTagsInput] = useState("");
  const [savingDetails, setSavingDetails] = useState(false);
  const [textLoading, setTextLoading] = useState(false);
  const [documentTexts, setDocumentTexts] = useState<Record<string, DocumentTextResponse>>({});
  const [textError, setTextError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const appendFiles = (incomingFiles: File[]) => {
    if (incomingFiles.length === 0) return;
    setNewFiles((prev) => [...prev, ...incomingFiles]);
    setError(null);
  };

  const loadContract = async () => {
    if (!contractId) return;
    setError(null);
    try {
      const response = await apiClient.getContract(contractId);
      setContract(response);
      setFiles(response.files ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load contract.");
    }
  };

  useEffect(() => {
    void loadContract();
  }, [contractId]);

  useEffect(() => {
    if (!contract) return;
    setContractNameInput(contract.name);
    setContractTagsInput((contract.tags ?? []).join(", "));
  }, [contract]);

  useEffect(() => {
    let cancelled = false;

    const loadDocumentTexts = async () => {
      if (files.length === 0) {
        setDocumentTexts({});
        setTextError(null);
        return;
      }

      setTextLoading(true);
      setTextError(null);
      try {
        const loaded = await Promise.all(
          files.map(async (file) => {
            const text = await apiClient.getDocumentText(file.id);
            return [file.id, text] as const;
          })
        );
        if (cancelled) return;
        setDocumentTexts(Object.fromEntries(loaded));
      } catch (err) {
        if (cancelled) return;
        setTextError(err instanceof Error ? err.message : "Failed to load contract text.");
      } finally {
        if (!cancelled) {
          setTextLoading(false);
        }
      }
    };

    void loadDocumentTexts();
    return () => {
      cancelled = true;
    };
  }, [files]);

  const onDragStart = (id: string) => {
    setDraggingId(id);
  };

  const moveFileBefore = (targetId: string) => {
    if (!draggingId || draggingId === targetId) return;
    setFiles((prev) => {
      const sourceIndex = prev.findIndex((item) => item.id === draggingId);
      const targetIndex = prev.findIndex((item) => item.id === targetId);
      if (sourceIndex < 0 || targetIndex < 0) return prev;
      const next = [...prev];
      const [moved] = next.splice(sourceIndex, 1);
      next.splice(targetIndex, 0, moved);
      return next;
    });
  };

  const hasUnsavedOrder = useMemo(() => {
    const original = (contract?.files ?? []).map((item) => item.id).join(",");
    const current = files.map((item) => item.id).join(",");
    return original !== current;
  }, [contract?.files, files]);

  const hasUnsavedDetails = useMemo(() => {
    if (!contract) return false;
    const normalizedName = contractNameInput.trim();
    const normalizedTags = contractTagsInput
      .split(",")
      .map((tag) => tag.trim())
      .filter((tag) => tag.length > 0)
      .join("|")
      .toLowerCase();
    const currentTags = (contract.tags ?? []).join("|").toLowerCase();
    return normalizedName !== contract.name || normalizedTags !== currentTags;
  }, [contract, contractNameInput, contractTagsInput]);

  const saveContractDetails = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!contractId || !contract) return;

    setSavingDetails(true);
    setError(null);
    setMessage(null);
    try {
      const updated = await apiClient.updateContract(contractId, {
        name: contractNameInput.trim(),
        tags: contractTagsInput
          .split(",")
          .map((tag) => tag.trim())
          .filter((tag) => tag.length > 0)
      });
      setContract(updated);
      setFiles(updated.files ?? []);
      setMessage("Contract details saved.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save contract details.");
    } finally {
      setSavingDetails(false);
    }
  };

  const saveOrder = async () => {
    if (!contractId) return;
    setSavingOrder(true);
    setError(null);
    setMessage(null);
    try {
      const updated = await apiClient.reorderContractFiles(
        contractId,
        files.map((item) => item.id)
      );
      setContract(updated);
      setFiles(updated.files ?? []);
      setMessage("File order saved.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save order.");
    } finally {
      setSavingOrder(false);
    }
  };

  const uploadMoreFiles = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!contractId || newFiles.length === 0) return;

    for (const file of newFiles) {
      if (file.type !== "application/pdf" && file.type !== "image/jpeg" && file.type !== "image/png") {
        setError("Only PDF, JPEG, and PNG files are supported.");
        return;
      }
    }

    setUploading(true);
    setError(null);
    setMessage(null);
    try {
      for (const file of newFiles) {
        const contentBase64 = await toBase64(file);
        await apiClient.addContractFile(contractId, {
          filename: file.name,
          mime_type: file.type as "application/pdf" | "image/jpeg" | "image/png",
          content_base64: contentBase64
        });
      }
      setNewFiles([]);
      await loadContract();
      setMessage("Files uploaded.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Upload failed.");
    } finally {
      setUploading(false);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <h2>Contract Files</h2>
        <div className="page-actions">
          <Link to="/contracts" className="button-link secondary">
            Back to Contracts
          </Link>
        </div>
      </header>

      {contract ? (
        <section className="panel">
          <h3>Contract Details</h3>
          <form className="form-grid" onSubmit={saveContractDetails}>
            <label>
              Contract Name
              <input value={contractNameInput} onChange={(event) => setContractNameInput(event.target.value)} />
            </label>
            <label>
              Tags
              <input
                value={contractTagsInput}
                onChange={(event) => setContractTagsInput(event.target.value)}
                placeholder="comma-separated"
              />
            </label>
            <div className="page-actions">
              <button type="submit" disabled={savingDetails || !hasUnsavedDetails || contractNameInput.trim().length === 0}>
                {savingDetails ? "Saving..." : "Save Details"}
              </button>
            </div>
          </form>
          <p className="muted">
            Contract ID: <code>{contract.id}</code> | Files: {files.length} | Updated: {formatEuropeanDateTime(contract.updated_at)}
          </p>
        </section>
      ) : null}

      <section className="contract-detail-grid">
        <div className="contract-detail-column">
          <section className="panel">
            <h3>Files</h3>
            <p className="muted">Drag and drop files to reorder pages/attachments inside this contract.</p>
            {files.length === 0 ? <p className="muted">No files uploaded yet.</p> : null}
            <div className="contract-file-list">
              {files.map((file, index) => (
                <div
                  key={file.id}
                  className={`contract-file-item${draggingId === file.id ? " dragging" : ""}`}
                  draggable
                  onDragStart={() => onDragStart(file.id)}
                  onDragOver={(event: DragEvent<HTMLDivElement>) => {
                    event.preventDefault();
                    moveFileBefore(file.id);
                  }}
                  onDrop={(event) => {
                    event.preventDefault();
                    setDraggingId(null);
                  }}
                  onDragEnd={() => setDraggingId(null)}
                >
                  <span className="drag-handle">::</span>
                  <span className="order-index">{index + 1}.</span>
                  <span className="file-name">{file.filename}</span>
                  <span className="chip chip-neutral">{file.mime_type}</span>
                </div>
              ))}
            </div>

            <form
              className={`file-upload-dropzone${isUploadDragOver ? " is-drag-over" : ""}`}
              onSubmit={uploadMoreFiles}
              onDragOver={(event: DragEvent<HTMLFormElement>) => {
                event.preventDefault();
                setIsUploadDragOver(true);
              }}
              onDragLeave={() => setIsUploadDragOver(false)}
              onDrop={(event: DragEvent<HTMLFormElement>) => {
                event.preventDefault();
                setIsUploadDragOver(false);
                appendFiles(Array.from(event.dataTransfer.files ?? []));
              }}
            >
              <h3>Add More Files</h3>
              <p className="muted">Drag and drop files here to add them to the bottom, or choose files manually.</p>
              <label>
                Files
                <input
                  type="file"
                  accept="application/pdf,image/jpeg,image/png"
                  multiple
                  onChange={(event) => appendFiles(Array.from(event.target.files ?? []))}
                />
              </label>
              {newFiles.length > 0 ? <p className="muted">Queued: {newFiles.length} file(s)</p> : null}
              <button type="submit" disabled={uploading || newFiles.length === 0}>
                {uploading ? "Uploading..." : "Upload Files"}
              </button>
            </form>

            <div className="page-actions">
              <button type="button" className="secondary" onClick={saveOrder} disabled={!hasUnsavedOrder || savingOrder}>
                {savingOrder ? "Saving..." : "Save Order"}
              </button>
            </div>
          </section>

          {message ? <p className="success-text">{message}</p> : null}
          {error ? <p className="error-text">{error}</p> : null}
        </div>

        <section className="panel">
          <h3>Contract Text</h3>
          <p className="muted">Combined extracted text from all files, shown in reading order.</p>
          {textLoading ? <p className="muted">Loading contract text...</p> : null}
          {textError ? <p className="error-text">{textError}</p> : null}
          {!textLoading && !textError ? (
            <article className="word-document-view">
              {files.length === 0 ? <p className="muted">No files yet, so there is no text to show.</p> : null}
              {files.map((file, index) => {
                const entry = documentTexts[file.id];
                return (
                  <section key={file.id} className="word-document-section">
                    <h4>
                      {index + 1}. {file.filename}
                    </h4>
                    {entry?.has_text ? (
                      <div className="word-document-text">{entry.text}</div>
                    ) : (
                      <p className="muted">No extracted text available for this file yet.</p>
                    )}
                  </section>
                );
              })}
            </article>
          ) : null}
        </section>
      </section>
    </section>
  );
}
