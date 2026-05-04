from urllib.parse import urlparse

import pytest

from app.core.config import DEFAULT_DATABASE_URL, Settings, settings


def _assert_valid_database_url(database_url: str) -> None:
    parsed_url = urlparse(database_url)

    assert parsed_url.scheme in {"postgresql", "postgresql+psycopg2"}
    assert parsed_url.hostname
    assert parsed_url.username
    assert parsed_url.password
    assert parsed_url.port is not None
    assert parsed_url.path.strip("/")


def test_database_urls_are_valid_postgres_urls() -> None:
    _assert_valid_database_url(DEFAULT_DATABASE_URL)
    _assert_valid_database_url(settings.database_url)


def test_production_requires_strong_secret_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ENVIRONMENT", "production")
    monkeypatch.setenv("SECRET_KEY", "change-this-in-production")

    with pytest.raises(ValueError, match="strong SECRET_KEY"):
        Settings.from_env()


def test_development_allows_local_default_secret(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("ENVIRONMENT", raising=False)
    monkeypatch.delenv("SECRET_KEY", raising=False)

    local_settings = Settings.from_env()

    assert local_settings.environment == "development"
    assert len(local_settings.secret_key) >= 32
