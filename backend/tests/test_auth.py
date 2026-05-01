from collections.abc import AsyncGenerator, Generator

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
