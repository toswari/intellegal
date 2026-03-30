from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field

from .indexing import HashEmbeddingGenerator
from .qdrant import QdrantService


class SearchSectionsResultItem(BaseModel):
    document_id: str
    page_number: int
    chunk_id: str | None = None
    score: float
    snippet_text: str


class SearchSectionsResult(BaseModel):
    items: list[SearchSectionsResultItem] = Field(default_factory=list)
    diagnostics: dict[str, Any] = Field(default_factory=dict)


class SearchPipeline:
    def __init__(self, *, qdrant: QdrantService, vector_size: int) -> None:
        self._qdrant = qdrant
        self._embeddings = HashEmbeddingGenerator(vector_size=vector_size)

    def search_sections(
        self,
        *,
        query_text: str,
        document_ids: list[str] | None,
        limit: int = 10,
    ) -> SearchSectionsResult:
        query = query_text.strip()
        if not query:
            return SearchSectionsResult(
                items=[],
                diagnostics={"fallback": "empty_query"},
            )

        vector = self._embeddings.embed(query)
        chunks = self._qdrant.search_chunks(
            query_vector=vector,
            document_ids=document_ids,
            limit=max(1, limit),
        )
        items = [
            SearchSectionsResultItem(
                document_id=chunk["document_id"],
                page_number=int(chunk.get("page_number") or 1),
                chunk_id=chunk.get("chunk_id"),
                score=round(float(chunk.get("score") or 0.0), 6),
                snippet_text=str(chunk.get("text") or ""),
            )
            for chunk in chunks
        ]
        return SearchSectionsResult(
            items=items,
            diagnostics={
                "query_length": len(query),
                "result_count": len(items),
                "strategy": "qdrant_vector",
            },
        )
