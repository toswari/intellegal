from __future__ import annotations

from py_ai_api.search import SearchPipeline


class _FakeQdrant:
    def __init__(self) -> None:
        self.called = False

    def search_chunks(self, **_: object):
        self.called = True
        return [
            {
                "document_id": "doc-1",
                "page_number": 2,
                "chunk_id": "11",
                "score": 0.67,
                "text": "payment terms are due in 30 days",
            }
        ]


class _FakeChunkStore:
    def __init__(self) -> None:
        self.calls: list[dict[str, object]] = []

    def search_sections_strict(self, *, query_text: str, document_ids: list[str] | None, limit: int):
        self.calls.append({"query_text": query_text, "document_ids": document_ids, "limit": limit})
        return [
            {
                "document_id": "doc-2",
                "page_number": 4,
                "chunk_id": "3",
                "score": 0.82,
                "text": "strict postgres match",
            }
        ]


def test_search_sections_semantic_strategy_uses_qdrant() -> None:
    qdrant = _FakeQdrant()
    pipeline = SearchPipeline(qdrant=qdrant, vector_size=8)

    result = pipeline.search_sections(query_text="payment terms", document_ids=["doc-1"], limit=5)

    assert result.diagnostics["strategy"] == "qdrant_vector"
    assert len(result.items) == 1
    assert result.items[0].document_id == "doc-1"
    assert qdrant.called is True


def test_search_sections_strict_strategy_uses_chunk_store() -> None:
    qdrant = _FakeQdrant()
    chunk_store = _FakeChunkStore()
    pipeline = SearchPipeline(qdrant=qdrant, vector_size=8, chunk_store=chunk_store)

    result = pipeline.search_sections(
        query_text="payment terms",
        document_ids=["doc-2"],
        limit=7,
        strategy="strict",
    )

    assert result.diagnostics["strategy"] == "postgres_strict_text"
    assert len(result.items) == 1
    assert result.items[0].document_id == "doc-2"
    assert chunk_store.calls == [{"query_text": "payment terms", "document_ids": ["doc-2"], "limit": 7}]
    assert qdrant.called is False
