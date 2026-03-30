from py_ai_api.config import Settings


def test_database_url_adds_sslmode_disable() -> None:
    settings = Settings(database_url="postgresql://app:app@localhost:5432/legal_doc_intel")
    assert settings.database_url.endswith("sslmode=disable")
