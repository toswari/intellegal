import { type FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { apiClient } from "../api/client";
import { addAuditEvent } from "../app/localState";

async function toBase64(file: File): Promise<string> {
  const buffer = await file.arrayBuffer();
  let binary = "";
  const bytes = new Uint8Array(buffer);

  for (const value of bytes) {
    binary += String.fromCharCode(value);
  }

  return btoa(binary);
}

export function NewContractPage() {
  const navigate = useNavigate();
  const [contractName, setContractName] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [sourceRef, setSourceRef] = useState("");
  const [tagsInput, setTagsInput] = useState("");
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const sourceType = "upload" as const;

  const uploadDocument = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!contractName.trim()) {
      setUploadError("Contract name is required.");
      return;
    }
    if (files.length === 0) {
      setUploadError("Select one or more files first.");
      return;
    }
    for (const file of files) {
      if (file.type !== "application/pdf" && file.type !== "image/jpeg" && file.type !== "image/png") {
        setUploadError("Only PDF, JPEG, and PNG files are supported.");
        return;
      }
    }

    setUploading(true);
    setUploadError(null);

    try {
      const tags = Array.from(
        new Set(
          tagsInput
            .split(",")
            .map((tag) => tag.trim())
            .filter((tag) => tag.length > 0)
        )
      );
      const contract = await apiClient.createContract(
        {
          name: contractName.trim(),
          source_type: sourceType,
          source_ref: sourceRef.trim() || undefined,
          tags: tags.length > 0 ? tags : undefined
        },
        { idempotencyKey: globalThis.crypto?.randomUUID?.() ?? `contract-${Date.now()}` }
      );

      for (const file of files) {
        const contentBase64 = await toBase64(file);
        await apiClient.addContractFile(
          contract.id,
          {
            filename: file.name,
            mime_type: file.type as "application/pdf" | "image/jpeg" | "image/png",
            source_type: sourceType,
            source_ref: sourceRef.trim() || undefined,
            tags: tags.length > 0 ? tags : undefined,
            content_base64: contentBase64
          },
          { idempotencyKey: globalThis.crypto?.randomUUID?.() ?? `upload-${Date.now()}-${file.name}` }
        );
      }

      addAuditEvent({
        type: "contract.created",
        message: `Created contract ${contract.name}`,
        metadata: { contract_id: contract.id, file_count: String(files.length) }
      });

      navigate(`/contracts/${encodeURIComponent(contract.id)}/edit`);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Upload failed.";
      setUploadError(message);
    } finally {
      setUploading(false);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <h2>New Contract</h2>
        <Link to="/contracts" className="button-link secondary">
          Back to Contracts
        </Link>
      </header>

      <form className="panel" onSubmit={uploadDocument}>
        <h3>Upload Contract</h3>
        <div className="form-grid form-grid-single-column">
          <label>
            Contract Name
            <input
              value={contractName}
              onChange={(event) => setContractName(event.target.value)}
              placeholder="Master Services Agreement 2026"
              required
            />
          </label>
          <label>
            Files
            <input
              type="file"
              accept="application/pdf,image/jpeg,image/png"
              multiple
              onChange={(event) => setFiles(Array.from(event.target.files ?? []))}
              required
            />
          </label>
          <label>
            Source Ref
            <input value={sourceRef} onChange={(event) => setSourceRef(event.target.value)} placeholder="Optional" />
          </label>
          <label>
            Tags
            <input
              value={tagsInput}
              onChange={(event) => setTagsInput(event.target.value)}
              placeholder="MSA, Vendor, 2026"
            />
          </label>
        </div>
        {files.length > 0 ? (
          <p className="muted">Upload order: {files.map((file) => file.name).join(" -> ")}</p>
        ) : null}
        {uploadError ? <p className="error-text">{uploadError}</p> : null}
        <div className="form-actions-end">
          <button type="submit" disabled={uploading}>
            {uploading ? "Uploading..." : "Create Contract"}
          </button>
        </div>
      </form>
    </section>
  );
}
