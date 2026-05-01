from app.core.config import DEFAULT_DATABASE_URL, settings


def test_database_url_defaults_to_docker_compose_port() -> None:
    assert "localhost:5434" in DEFAULT_DATABASE_URL
    assert "localhost:5434" in settings.database_url
