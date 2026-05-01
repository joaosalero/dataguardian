from collections.abc import AsyncGenerator, Generator
import logging

import app.models
import pytest
from httpx import ASGITransport, AsyncClient
from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker
from sqlalchemy.pool import StaticPool

from app.auth import hash_password
from app.core.database import Base, get_db
from app.main import app
from app.models.user import User


engine = create_engine(
    "sqlite://",
    connect_args={"check_same_thread": False},
    poolclass=StaticPool,
)
TestingSessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)


@pytest.fixture(autouse=True)
def setup_database() -> Generator[None, None, None]:
    Base.metadata.create_all(bind=engine)
    app.dependency_overrides[get_db] = override_get_db
    yield
    app.dependency_overrides.clear()
    Base.metadata.drop_all(bind=engine)


async def override_get_db() -> AsyncGenerator[Session, None]:
    db = TestingSessionLocal()
    try:
        yield db
    finally:
        db.close()


async def login_user(
    client: AsyncClient,
    email: str = "owner@example.com",
) -> str:
    login_response = await client.post(
        "/auth/login",
        json={"username": email, "password": "strong-password"},
    )
    assert login_response.status_code == 200
    token = login_response.json()["access_token"]
    assert token
    return token


def create_user(email: str) -> User:
    db = TestingSessionLocal()
    try:
        user = User(email=email, hashed_password=hash_password("strong-password"))
        db.add(user)
        db.commit()
        db.refresh(user)
        return user
    finally:
        db.close()


@pytest.mark.asyncio
async def test_login_returns_jwt() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        token = await login_user(client)

    assert isinstance(token, str)


@pytest.mark.asyncio
async def test_invalid_login_returns_401() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.post(
            "/auth/login",
            json={"username": "owner@example.com", "password": "wrong-password"},
        )

    assert response.status_code == 401
    assert response.json()["detail"] == "Invalid username or password"


@pytest.mark.asyncio
async def test_login_logs_success_and_failure(caplog: pytest.LogCaptureFixture) -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        with caplog.at_level(logging.INFO):
            success_response = await client.post(
                "/auth/login",
                json={"username": "owner@example.com", "password": "strong-password"},
            )
            failure_response = await client.post(
                "/auth/login",
                json={"username": "owner@example.com", "password": "wrong-password"},
            )

    assert success_response.status_code == 200
    assert failure_response.status_code == 401
    assert "User login succeeded" in caplog.text
    assert "User login failed" in caplog.text


@pytest.mark.asyncio
async def test_auth_me_returns_current_user() -> None:
    user = create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        token = await login_user(client)
        response = await client.get(
            "/auth/me",
            headers={"Authorization": f"Bearer {token}"},
        )

    assert response.status_code == 200
    assert response.json()["id"] == user.id
    assert response.json()["email"] == user.email


@pytest.mark.asyncio
async def test_auth_me_requires_token() -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.get("/auth/me")

    assert response.status_code == 401


@pytest.mark.asyncio
async def test_protected_projects_require_authentication() -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.get("/projects")

    assert response.status_code == 401


@pytest.mark.asyncio
async def test_project_routes_are_limited_to_current_user() -> None:
    create_user("owner@example.com")
    create_user("other@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        owner_token = await login_user(client, "owner@example.com")
        other_token = await login_user(client, "other@example.com")

        create_response = await client.post(
            "/projects",
            headers={"Authorization": f"Bearer {owner_token}"},
            json={"name": "Private Project", "description": "Sensitive"},
        )
        assert create_response.status_code == 201
        project_id = create_response.json()["id"]

        owner_list_response = await client.get(
            "/projects",
            headers={"Authorization": f"Bearer {owner_token}"},
        )
        other_list_response = await client.get(
            "/projects",
            headers={"Authorization": f"Bearer {other_token}"},
        )
        other_get_response = await client.get(
            f"/projects/{project_id}",
            headers={"Authorization": f"Bearer {other_token}"},
        )

    assert len(owner_list_response.json()) == 1
    assert other_list_response.json() == []
    assert other_get_response.status_code == 404


@pytest.mark.asyncio
async def test_project_create_rejects_empty_name() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        token = await login_user(client, "owner@example.com")
        response = await client.post(
            "/projects",
            headers={"Authorization": f"Bearer {token}"},
            json={"name": "   ", "description": "No name"},
        )

    assert response.status_code == 400
    assert response.json()["detail"] == "Project name is required"


@pytest.mark.asyncio
async def test_project_list_returns_only_current_user_projects() -> None:
    create_user("owner@example.com")
    create_user("other@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        owner_token = await login_user(client, "owner@example.com")
        other_token = await login_user(client, "other@example.com")

        owner_project_response = await client.post(
            "/projects",
            headers={"Authorization": f"Bearer {owner_token}"},
            json={"name": "Owner Project"},
        )
        other_project_response = await client.post(
            "/projects",
            headers={"Authorization": f"Bearer {other_token}"},
            json={"name": "Other Project"},
        )
        owner_list_response = await client.get(
            "/projects",
            headers={"Authorization": f"Bearer {owner_token}"},
        )

    assert owner_project_response.status_code == 201
    assert other_project_response.status_code == 201
    assert [project["name"] for project in owner_list_response.json()] == [
        "Owner Project"
    ]


@pytest.mark.asyncio
async def test_audit_routes_are_limited_to_project_owner() -> None:
    create_user("owner@example.com")
    create_user("other@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        owner_token = await login_user(client, "owner@example.com")
        other_token = await login_user(client, "other@example.com")

        create_response = await client.post(
            "/projects",
            headers={"Authorization": f"Bearer {owner_token}"},
            json={"name": "Audit Project"},
        )
        assert create_response.status_code == 201
        project_id = create_response.json()["id"]

        owner_audit_response = await client.post(
            f"/projects/{project_id}/audit/run",
            headers={"Authorization": f"Bearer {owner_token}"},
        )
        other_audit_response = await client.post(
            f"/projects/{project_id}/audit/run",
            headers={"Authorization": f"Bearer {other_token}"},
        )
        other_history_response = await client.get(
            f"/projects/{project_id}/audit/history",
            headers={"Authorization": f"Bearer {other_token}"},
        )

    assert owner_audit_response.status_code == 200
    assert other_audit_response.status_code == 404
    assert other_history_response.status_code == 404


@pytest.mark.asyncio
async def test_audit_run_and_history_flow() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        token = await login_user(client, "owner@example.com")
        create_response = await client.post(
            "/projects",
            headers={"Authorization": f"Bearer {token}"},
            json={"name": "Audit Project"},
        )
        project_id = create_response.json()["id"]

        audit_response = await client.post(
            f"/projects/{project_id}/audit/run",
            headers={"Authorization": f"Bearer {token}"},
        )
        history_response = await client.get(
            f"/projects/{project_id}/audit/history",
            headers={"Authorization": f"Bearer {token}"},
        )

    assert audit_response.status_code == 200
    assert audit_response.json()["audit_id"] > 0
    assert audit_response.json()["score"] == 80
    assert history_response.status_code == 200
    assert history_response.json() == [
        {
            "audit_id": audit_response.json()["audit_id"],
            "project_id": project_id,
            "status": "completed",
            "score": 80,
        }
    ]


@pytest.mark.asyncio
async def test_audit_invalid_project_returns_404() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        token = await login_user(client, "owner@example.com")
        response = await client.post(
            "/projects/999/audit/run",
            headers={"Authorization": f"Bearer {token}"},
        )

    assert response.status_code == 404
    assert response.json()["detail"] == "Project not found"
