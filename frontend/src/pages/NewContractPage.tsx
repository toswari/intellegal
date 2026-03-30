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
  const [file, setFile] = useState<File | null>(null);
  const [sourceType, setSourceType] = useState<"repository" | "upload" | "api">("upload");
  const [sourceRef, setSourceRef] = useState("");
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  const uploadDocument = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!file) {
      setUploadError("Select a PDF or JPEG file first.");
      return;
    }

    if (file.type !== "application/pdf" && file.type !== "image/jpeg") {
      setUploadError("Only application/pdf and image/jpeg are supported.");
      return;
    }

    setUploading(true);
    setUploadError(null);

    try {
      const contentBase64 = await toBase64(file);
      const response = await apiClient.createDocument(
        {
          filename: file.name,
          mime_type: file.type as "application/pdf" | "image/jpeg",
          source_type: sourceType,
          source_ref: sourceRef.trim() || undefined,
          content_base64: contentBase64
        },
        { idempotencyKey: globalThis.crypto?.randomUUID?.() ?? `upload-${Date.now()}` }
      );

      addAuditEvent({
        type: "document.uploaded",
        message: `Uploaded ${response.filename}`,
        metadata: { document_id: response.id, mime_type: response.mime_type }
      });

      navigate("/contracts");
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
        <div className="form-grid">
          <label>
            File
            <input
              type="file"
              accept="application/pdf,image/jpeg"
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              required
            />
          </label>
          <label>
            Source Type
            <select value={sourceType} onChange={(event) => setSourceType(event.target.value as typeof sourceType)}>
              <option value="upload">upload</option>
              <option value="repository">repository</option>
              <option value="api">api</option>
            </select>
          </label>
          <label>
            Source Ref
            <input value={sourceRef} onChange={(event) => setSourceRef(event.target.value)} placeholder="Optional" />
          </label>
        </div>
        {uploadError ? <p className="error-text">{uploadError}</p> : null}
        <button type="submit" disabled={uploading}>{uploading ? "Uploading..." : "Upload"}</button>
      </form>
    </section>
  );
}
