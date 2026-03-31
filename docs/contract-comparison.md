# Contract Comparison

## User flow

```mermaid
flowchart LR
    A["Contracts page"] --> B["Select 2 contracts"]
    B --> C["Open compare view"]
    C --> D["Load left + right document text"]
    D --> E["Semantic Sections"]
    D --> F["Line Diff"]
    E --> G["Reviewer checks clause-level changes"]
    F --> G
```

### Current scope
- Compare 2 document files
- If a contract has multiple files, the UI picks one representative file
- Two modes: `Semantic Sections` and `Line Diff`

## Technical flow

```mermaid
sequenceDiagram
    participant U as User
    participant FE as Frontend
    participant GO as Go API

    U->>FE: Compare two contracts
    FE->>GO: GET document meta (left)
    FE->>GO: GET document text (left)
    FE->>GO: GET document meta (right)
    FE->>GO: GET document text (right)
    GO-->>FE: Extracted text for both docs
    FE->>FE: Build semantic diff locally
    FE->>FE: Build line diff locally
    FE-->>U: Show side-by-side comparison
```

### Main files
- `frontend/src/pages/ContractsPage.tsx`
- `frontend/src/pages/CompareContractsPage.tsx`
- `frontend/src/pages/contractCompare.ts`
- `go-api/internal/http/handlers/documents.go`

## How we decide which contract parts to fetch

### Current rule
We do not fetch clause sections from the backend yet.

```mermaid
flowchart TD
    A["Full extracted text"] --> B["Split into paragraphs"]
    B --> C["Pick category"]
    C --> D["Score each paragraph by keywords"]
    D --> E["Sort by score"]
    E --> F["Take top 4 paragraphs"]
    F --> G["Compare left vs right text"]
```

### Categories
```mermaid
mindmap
  root((Semantic Sections))
    Parties
    Scope
    Payment
    "Term & Termination"
    Obligations
    "Liability & Indemnity"
    "Confidentiality & Data"
    "Intellectual Property"
    "Disputes & Governing Law"
    "Breach & Remedies"
```

### Matching rule
- Split text on blank lines
- Ignore very short paragraphs
- Score paragraphs by category keywords
- Multi-word keywords get slightly more weight
- Join the top 4 matches per category

## Similarity rule

```mermaid
flowchart LR
    A["Left category text"] --> C["Tokenize"]
    B["Right category text"] --> C
    C --> D["Jaccard similarity"]
    D --> E{"Result"}
    E -->|"both empty or >= 0.65"| F["similar"]
    E -->|"left missing"| G["missing_left"]
    E -->|"right missing"| H["missing_right"]
    E -->|"otherwise"| I["changed"]
```

## Limitations
- Keyword matching can miss unusual wording
- Wrong paragraphs can be selected when terms overlap
- No heading or clause-number awareness yet
- No chunk-level or vector-based alignment yet

## Next likely step

```mermaid
flowchart LR
    A["Current: frontend keyword matching"] --> B["Backend section matcher"]
    B --> C["Chunk-level retrieval"]
    C --> D["Heading / clause alignment"]
    D --> E["Evidence links to pages and chunks"]
```
