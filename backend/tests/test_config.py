from urllib.parse import urlparse

from app.core.config import DEFAULT_DATABASE_URL, settings


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
