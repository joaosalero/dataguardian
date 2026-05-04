import logging

import pytest
from httpx import ASGITransport, AsyncClient

from app.main import app
from app.core.config import settings


@app.get("/test-unhandled-error")
async def unhandled_error_route() -> None:
    raise RuntimeError("database password leaked")


@pytest.mark.asyncio
async def test_health_check_returns_ok() -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.get("/health")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


@pytest.mark.asyncio
async def test_health_check_includes_security_headers() -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.get("/health")

    assert response.headers["x-content-type-options"] == "nosniff"
    assert response.headers["x-frame-options"] == "DENY"
    assert response.headers["referrer-policy"] == "no-referrer"


@pytest.mark.asyncio
async def test_production_rejects_plain_http() -> None:
    original_environment = settings.environment
    object.__setattr__(settings, "environment", "prod")
    try:
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            response = await client.get("/health")
    finally:
        object.__setattr__(settings, "environment", original_environment)

    assert response.status_code == 403
    assert response.json() == {"detail": "HTTPS is required"}


@pytest.mark.asyncio
async def test_cors_allows_frontend_login_requests() -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.options(
            "/auth/login",
            headers={
                "Origin": "http://localhost:3000",
                "Access-Control-Request-Method": "POST",
                "Access-Control-Request-Headers": "content-type",
            },
        )

    assert response.status_code == 200
    assert response.headers["access-control-allow-origin"] == "http://localhost:3000"
    assert response.headers["access-control-allow-credentials"] == "true"


@pytest.mark.asyncio
async def test_unhandled_errors_return_safe_response() -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app, raise_app_exceptions=False),
        base_url="http://testserver",
    ) as client:
        response = await client.get("/test-unhandled-error")

    assert response.status_code == 500
    assert response.json() == {"detail": "Internal server error"}
    assert "database password leaked" not in response.text


@pytest.mark.asyncio
async def test_unhandled_error_logs_do_not_include_exception_message(
    caplog: pytest.LogCaptureFixture,
) -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app, raise_app_exceptions=False),
        base_url="http://testserver",
    ) as client:
        with caplog.at_level(logging.ERROR):
            response = await client.get("/test-unhandled-error")

    assert response.status_code == 500
    assert "Unhandled exception" in caplog.text
    assert "RuntimeError" in caplog.text
    assert "database password leaked" not in caplog.text


@pytest.mark.asyncio
async def test_openapi_documents_core_routes() -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.get("/openapi.json")

    schema = response.json()
    assert response.status_code == 200
    assert schema["paths"]["/health"]["get"]["summary"] == "Check API health"
    assert schema["paths"]["/auth/login"]["post"]["summary"] == "Authenticate user"
    assert schema["paths"]["/projects"]["get"]["summary"] == "List projects"
    assert (
        schema["paths"]["/projects/{project_id}/audit/run"]["post"]["summary"]
        == "Run project audit"
    )
