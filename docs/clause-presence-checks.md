# Clause-Presence Checks

## User flow

```mermaid
flowchart LR
    A["Guidelines page"] --> B["Choose saved rule"]
    B --> C["Pick scope: all or selected"]
    C --> D["Start run"]
    D --> E["Check enters queued/running state"]
    E --> F["Results page loads run"]
    F --> G["Reviewer sees match / review / missing"]
    G --> H["Reviewer opens evidence snippets"]
```

### Current scope
- Run against all documents or a selected set
- Return one result per document
- Show summary, confidence, and evidence snippets

## Frontend rule mapping

- `Lexical clause check`: frontend rule type `clause_presence`; runs the backend clause-presence analysis described in this doc
- `Gemini contract review`: frontend rule type `gemini_review`; uses the separate LLM review path and is not covered by this doc
- `Strict keyword check`: frontend rule type `keyword_match`; runs locally in the frontend and is not covered by this doc

## Technical flow

```mermaid
sequenceDiagram
    participant U as User
    participant FE as Frontend
    participant GO as Go API
    participant AI as Python AI API
    participant Q as Indexed Chunks

    U->>FE: Start clause-presence check
    FE->>GO: POST /api/v1/guidelines/clause-presence
    GO-->>FE: check_id + queued
    GO->>GO: Mark running
    GO->>AI: POST /internal/v1/analyze/clause
    AI->>Q: Load indexed chunks per document
    AI->>AI: Score best matching chunk
    AI-->>GO: outcome + confidence + evidence
    GO->>GO: Mark completed
    FE->>GO: GET /api/v1/guidelines/:id
    FE->>GO: GET /api/v1/guidelines/:id/results
    GO-->>FE: status + results
```

### Main files
- `frontend/src/pages/GuidelineRunPage.tsx`
- `frontend/src/pages/GuidelinesPage.tsx`
- `frontend/src/api/client.ts`
- `go-api/internal/http/handlers/checks.go`
- `py-ai-api/py_ai_api/analysis.py`
- `py-ai-api/py_ai_api/main.py`

## How matching works

```mermaid
flowchart TD
    A["Required clause text"] --> B["Tokenize clause text"]
    C["Indexed chunks for one document"] --> D["For each chunk"]
    B --> D
    D --> E["Token overlap score"]
    A --> F["Exact phrase containment check"]
    D --> G["Best score = max(overlap, phrase match)"]
    F --> G
    G --> H["Pick best chunk"]
```

### Data source
- Uses indexed chunks, not whole-document text
- Loads up to 64 chunks per document from Qdrant-backed retrieval

## Decision logic

```mermaid
flowchart LR
    A["Best score"] --> B{"Threshold"}
    B -->|"score >= 0.7"| C["match"]
    B -->|"0.35 <= score < 0.7"| D["review"]
    B -->|"score < 0.35"| E["missing"]
```

### Current outputs
- `match`: strong evidence that the clause is present
- `review`: some overlap, but not enough for automatic confidence
- `missing`: no convincing evidence found

## Evidence model

```mermaid
flowchart LR
    A["Best chunk"] --> B["snippet_text"]
    A --> C["page_number"]
    A --> D["chunk_id"]
    A --> E["score"]
    B --> F["Shown in results UI"]
    C --> F
    D --> F
    E --> F
```

### What the reviewer sees
- Outcome
- Confidence %
- Short summary
- Evidence snippet with page reference

## Status flow

```mermaid
stateDiagram-v2
    [*] --> queued
    queued --> running
    running --> completed
    running --> failed
```

## Important nuance

```mermaid
flowchart LR
    A["Lexical clause check"] --> B["Remote backend clause-presence analysis"]
    C["Gemini contract review"] --> D["Separate LLM review path"]
    E["Strict keyword rule"] --> F["Separate local frontend path"]
```

- This doc describes the backend clause-presence feature behind the `Lexical clause check` rule type
- It is different from both the `Gemini contract review` path and the local `Strict keyword check` path in the UI

## Limitations
- Lexical matching can miss paraphrased clauses
- Best-chunk selection may miss distributed language across multiple chunks
- `context_hint` exists in the request shape but is not used in current analysis
- Results depend on ingestion quality and chunking quality
