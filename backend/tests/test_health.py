import pytest
from httpx import ASGITransport, AsyncClient

from app.main import app


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
