from collections.abc import AsyncGenerator, Generator
from datetime import timedelta
import logging

import app.models
import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from httpx import ASGITransport, AsyncClient
from jose import jwt
from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker
from sqlalchemy.pool import StaticPool

from app.auth import (
    create_access_token,
    hash_password,
    reset_rate_limits,
    verify_password,
)
from app.core.config import settings
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
    reset_rate_limits()
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
    assert login_response.json() == {"message": "Authenticated"}
    token = login_response.cookies.get(settings.auth_cookie_name)
    assert token
    return f"{settings.auth_cookie_name}={token}"


def create_user(email: str) -> User:
    db = TestingSessionLocal()
    try:
        user = User(
            email=email,
            encrypted_email=None,
            hashed_password=hash_password("strong-password"),
        )
        db.add(user)
        db.commit()
        db.refresh(user)
        return user
    finally:
        db.close()


@pytest.mark.asyncio
async def test_login_sets_httponly_session_cookie() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        cookie_header = await login_user(client)

    assert settings.auth_cookie_name in cookie_header


@pytest.mark.asyncio
async def test_login_cookie_does_not_expose_token_in_body() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.post(
            "/auth/login",
            json={"username": "owner@example.com", "password": "strong-password"},
        )

    assert response.status_code == 200
    assert response.json() == {"message": "Authenticated"}
    assert "access_token" not in response.text
    assert "httponly" in response.headers["set-cookie"].lower()


def test_password_hashing_uses_argon2() -> None:
    hashed_password = hash_password("StrongPassword123")

    assert hashed_password.startswith("$argon2")
    assert "StrongPassword123" not in hashed_password
    assert verify_password("StrongPassword123", hashed_password)
    assert not verify_password("wrong-password", hashed_password)


def test_jwt_contains_hardened_claims() -> None:
    token = create_access_token({"sub": 123})
    from app.security.jwt_keys import get_jwt_public_key

    payload = jwt.decode(token, get_jwt_public_key(), algorithms=[settings.algorithm])

    assert payload["sub"] == "123"
    assert "exp" in payload
    assert "iat" in payload
    assert "user_id" not in payload


@pytest.mark.asyncio
async def test_valid_token_allows_current_user() -> None:
    user = create_user("owner@example.com")
    token = create_access_token({"sub": user.id})

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.get(
            "/auth/me",
            headers={"Authorization": f"Bearer {token}"},
        )

    assert response.status_code == 200
    assert response.json()["email"] == "owner@example.com"


@pytest.mark.asyncio
async def test_expired_token_returns_generic_error() -> None:
    user = create_user("owner@example.com")
    token = create_access_token({"sub": user.id}, expires_delta=timedelta(seconds=-1))

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.get(
            "/auth/me",
            headers={"Authorization": f"Bearer {token}"},
        )

    assert response.status_code == 401
    assert response.json() == {"detail": "Invalid authentication credentials"}


@pytest.mark.asyncio
async def test_invalid_signature_returns_generic_error() -> None:
    create_user("owner@example.com")
    private_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    ).decode()
    forged_token = jwt.encode(
        {"sub": "1"},
        private_pem,
        algorithm=settings.algorithm,
    )

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.get(
            "/auth/me",
            headers={"Authorization": f"Bearer {forged_token}"},
        )

    assert response.status_code == 401
    assert response.json() == {"detail": "Invalid authentication credentials"}


@pytest.mark.asyncio
async def test_register_creates_argon2_user_with_encrypted_email() -> None:
    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.post(
            "/auth/register",
            json={"username": "new@example.com", "password": "StrongPass123"},
        )

    assert response.status_code == 201
    assert response.json()["email"] == "new@example.com"

    db = TestingSessionLocal()
    try:
        user = db.query(User).filter(User.email == "new@example.com").one()
        assert user.hashed_password.startswith("$argon2")
        assert user.encrypted_email
        assert user.encrypted_email != user.email
    finally:
        db.close()


@pytest.mark.asyncio
async def test_register_rejects_weak_password_and_duplicate_user() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        weak_response = await client.post(
            "/auth/register",
            json={"username": "weak@example.com", "password": "weak"},
        )
        duplicate_response = await client.post(
            "/auth/register",
            json={"username": "owner@example.com", "password": "StrongPass123"},
        )

    assert weak_response.status_code == 400
    assert duplicate_response.status_code == 409


@pytest.mark.asyncio
async def test_login_rate_limit_triggers_generic_429() -> None:
    original_limit = settings.auth_rate_limit_max_requests
    original_window = settings.auth_rate_limit_window_seconds
    object.__setattr__(settings, "auth_rate_limit_max_requests", 2)
    object.__setattr__(settings, "auth_rate_limit_window_seconds", 60)
    try:
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            first_response = await client.post(
                "/auth/login",
                json={"username": "missing@example.com", "password": "wrong-password"},
            )
            second_response = await client.post(
                "/auth/login",
                json={"username": "missing@example.com", "password": "wrong-password"},
            )
            limited_response = await client.post(
                "/auth/login",
                json={"username": "missing@example.com", "password": "wrong-password"},
            )
    finally:
        object.__setattr__(settings, "auth_rate_limit_max_requests", original_limit)
        object.__setattr__(settings, "auth_rate_limit_window_seconds", original_window)

    assert first_response.status_code == 401
    assert second_response.status_code == 401
    assert limited_response.status_code == 429
    assert limited_response.json() == {"detail": "Too many requests"}


@pytest.mark.asyncio
async def test_register_rate_limit_triggers() -> None:
    original_limit = settings.auth_rate_limit_max_requests
    original_window = settings.auth_rate_limit_window_seconds
    object.__setattr__(settings, "auth_rate_limit_max_requests", 1)
    object.__setattr__(settings, "auth_rate_limit_window_seconds", 60)
    try:
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            first_response = await client.post(
                "/auth/register",
                json={"username": "first@example.com", "password": "StrongPass123"},
            )
            limited_response = await client.post(
                "/auth/register",
                json={"username": "second@example.com", "password": "StrongPass123"},
            )
    finally:
        object.__setattr__(settings, "auth_rate_limit_max_requests", original_limit)
        object.__setattr__(settings, "auth_rate_limit_window_seconds", original_window)

    assert first_response.status_code == 201
    assert limited_response.status_code == 429


@pytest.mark.asyncio
async def test_normal_login_flow_allowed_before_rate_limit() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        response = await client.post(
            "/auth/login",
            json={"username": "owner@example.com", "password": "strong-password"},
        )

    assert response.status_code == 200
    assert response.json() == {"message": "Authenticated"}


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
    assert response.json()["detail"] == "Invalid credentials"


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
    assert "owner@example.com" not in caplog.text


@pytest.mark.asyncio
async def test_login_uses_same_error_for_missing_user_and_bad_password() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        wrong_password_response = await client.post(
            "/auth/login",
            json={"username": "owner@example.com", "password": "wrong-password"},
        )
        missing_user_response = await client.post(
            "/auth/login",
            json={"username": "missing@example.com", "password": "wrong-password"},
        )

    assert wrong_password_response.status_code == 401
    assert missing_user_response.status_code == 401
    assert wrong_password_response.json() == missing_user_response.json()


@pytest.mark.asyncio
async def test_auth_me_returns_current_user() -> None:
    user = create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        await login_user(client)
        response = await client.get(
            "/auth/me",
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
        owner_cookie = await login_user(client, "owner@example.com")
        other_cookie = await login_user(client, "other@example.com")

        create_response = await client.post(
            "/projects",
            headers={"Cookie": owner_cookie},
            json={"name": "Private Project", "description": "Sensitive"},
        )
        assert create_response.status_code == 201
        project_id = create_response.json()["id"]

        owner_list_response = await client.get(
            "/projects",
            headers={"Cookie": owner_cookie},
        )
        other_list_response = await client.get(
            "/projects",
            headers={"Cookie": other_cookie},
        )
        other_get_response = await client.get(
            f"/projects/{project_id}",
            headers={"Cookie": other_cookie},
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
        cookie = await login_user(client, "owner@example.com")
        response = await client.post(
            "/projects",
            headers={"Cookie": cookie},
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
        owner_cookie = await login_user(client, "owner@example.com")
        other_cookie = await login_user(client, "other@example.com")

        owner_project_response = await client.post(
            "/projects",
            headers={"Cookie": owner_cookie},
            json={"name": "Owner Project"},
        )
        other_project_response = await client.post(
            "/projects",
            headers={"Cookie": other_cookie},
            json={"name": "Other Project"},
        )
        owner_list_response = await client.get(
            "/projects",
            headers={"Cookie": owner_cookie},
        )
        other_list_response = await client.get(
            "/projects",
            headers={"Cookie": other_cookie},
        )

    assert owner_project_response.status_code == 201
    assert other_project_response.status_code == 201
    assert [project["name"] for project in owner_list_response.json()] == [
        "Owner Project"
    ]
    assert [project["name"] for project in other_list_response.json()] == [
        "Other Project"
    ]


@pytest.mark.asyncio
async def test_audit_routes_are_limited_to_project_owner() -> None:
    create_user("owner@example.com")
    create_user("other@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        owner_cookie = await login_user(client, "owner@example.com")
        other_cookie = await login_user(client, "other@example.com")

        create_response = await client.post(
            "/projects",
            headers={"Cookie": owner_cookie},
            json={"name": "Audit Project"},
        )
        assert create_response.status_code == 201
        project_id = create_response.json()["id"]

        owner_audit_response = await client.post(
            f"/projects/{project_id}/audit/run",
            headers={"Cookie": owner_cookie},
        )
        owner_history_response = await client.get(
            f"/projects/{project_id}/audit/history",
            headers={"Cookie": owner_cookie},
        )
        other_audit_response = await client.post(
            f"/projects/{project_id}/audit/run",
            headers={"Cookie": other_cookie},
        )
        other_history_response = await client.get(
            f"/projects/{project_id}/audit/history",
            headers={"Cookie": other_cookie},
        )

    assert owner_audit_response.status_code == 200
    assert owner_history_response.status_code == 200
    assert len(owner_history_response.json()) == 1
    assert other_audit_response.status_code == 404
    assert other_audit_response.json() == {"detail": "Project not found"}
    assert other_history_response.status_code == 404
    assert other_history_response.json() == {"detail": "Project not found"}


@pytest.mark.asyncio
async def test_audit_run_and_history_flow() -> None:
    create_user("owner@example.com")

    async with AsyncClient(
        transport=ASGITransport(app=app),
        base_url="http://testserver",
    ) as client:
        cookie = await login_user(client, "owner@example.com")
        create_response = await client.post(
            "/projects",
            headers={"Cookie": cookie},
            json={"name": "Audit Project"},
        )
        project_id = create_response.json()["id"]

        audit_response = await client.post(
            f"/projects/{project_id}/audit/run",
            headers={"Cookie": cookie},
        )
        history_response = await client.get(
            f"/projects/{project_id}/audit/history",
            headers={"Cookie": cookie},
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
        cookie = await login_user(client, "owner@example.com")
        response = await client.post(
            "/projects/999/audit/run",
            headers={"Cookie": cookie},
        )

    assert response.status_code == 404
    assert response.json()["detail"] == "Project not found"
