from py_ai_api.config import Settings
from py_ai_api.qdrant import QdrantService, build_collection_schema


class _FakeQdrantClient:
    def __init__(self) -> None:
        self.calls: list[tuple[str, dict[str, object]]] = []
        self.exists = False

    def get_collections(self) -> None:
        self.calls.append(("get_collections", {}))

    def collection_exists(self, collection_name: str) -> bool:
        self.calls.append(("collection_exists", {"collection_name": collection_name}))
        return self.exists

    def create_collection(self, **kwargs: object) -> None:
        self.calls.append(("create_collection", kwargs))
        self.exists = True

    def create_payload_index(self, **kwargs: object) -> None:
        self.calls.append(("create_payload_index", kwargs))

    def count(self, **kwargs: object):
        self.calls.append(("count", kwargs))

        class _Response:
            count = 0

        return _Response()

    def delete(self, **kwargs: object) -> None:
        self.calls.append(("delete", kwargs))

    def upsert(self, **kwargs: object) -> None:
        self.calls.append(("upsert", kwargs))


def test_build_collection_schema_defaults() -> None:
    schema = build_collection_schema(Settings())

    assert schema.name == "document_chunks"
    assert schema.vector_size == 1536
    assert "document_id" in schema.payload_fields
    assert "page_number" in schema.payload_fields


def test_startup_check_creates_collection_and_indexes() -> None:
    client = _FakeQdrantClient()
    service = QdrantService(Settings(), client=client)

    service.startup_check()

    call_names = [name for name, _ in client.calls]
    assert "get_collections" in call_names
    assert "create_collection" in call_names
    assert call_names.count("create_payload_index") >= 1


def test_chunk_ops_call_expected_qdrant_methods() -> None:
    client = _FakeQdrantClient()
    service = QdrantService(Settings(), client=client)

    count = service.count_chunks(document_id="doc-1", checksum="sha")
    service.delete_document_chunks(document_id="doc-1")
    service.upsert_chunks(
        [
            {
                "id": "doc-1:sha:1",
                "vector": [0.1, 0.2],
                "payload": {"document_id": "doc-1", "checksum": "sha"},
            }
        ]
    )

    assert count == 0
    call_names = [name for name, _ in client.calls]
    assert "count" in call_names
    assert "delete" in call_names
    assert "upsert" in call_names
