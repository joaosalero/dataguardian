from datetime import datetime, timedelta, timezone
import logging
from typing import Any

from argon2 import PasswordHasher
from argon2.exceptions import VerifyMismatchError, VerificationError
from fastapi import APIRouter, Cookie, Depends, HTTPException, Response, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from jose import JWTError, jwt
from pydantic import BaseModel, Field
from sqlalchemy.orm import Session

from app.core.config import settings
from app.core.database import get_db
from app.models.user import User
from app.security.crypto import encrypt_text


logger = logging.getLogger(__name__)
router = APIRouter(prefix="/auth", tags=["auth"])
bearer_scheme = HTTPBearer(auto_error=False)
password_hasher = PasswordHasher()


class LoginRequest(BaseModel):
    username: str
    password: str


class RegisterRequest(BaseModel):
    username: str = Field(min_length=1, max_length=255)
    password: str = Field(min_length=1, max_length=255)


class AuthResponse(BaseModel):
    message: str


class UserResponse(BaseModel):
    id: int
    email: str
    created_at: datetime

    model_config = {"from_attributes": True}


def hash_password(password: str) -> str:
    """Hash a plaintext password for storage; callers must never log the result."""
    return password_hasher.hash(password)


def verify_password(plain: str, hashed: str) -> bool:
    """Verify a password without exposing whether the user lookup or hash check failed."""
    try:
        return password_hasher.verify(hashed, plain)
    except (VerifyMismatchError, VerificationError):
        return False


def create_access_token(data: dict[str, Any]) -> str:
    """Create a short-lived signed JWT containing only non-sensitive claims."""
    issued_at = datetime.now(timezone.utc)
    expires_at = issued_at + timedelta(minutes=settings.access_token_expire_minutes)
    subject = str(data["sub"])
    payload = {"sub": subject, "iat": issued_at, "exp": expires_at}
    return jwt.encode(payload, settings.secret_key, algorithm=settings.algorithm)


def validate_password_strength(password: str) -> None:
    if len(password) < 8:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Password must be at least 8 characters long",
        )
    if password.lower() == password or password.upper() == password:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Password must include mixed case characters",
        )
    if not any(character.isdigit() for character in password):
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail="Password must include a number",
        )


def set_auth_cookie(response: Response, token: str) -> None:
    response.set_cookie(
        key=settings.auth_cookie_name,
        value=token,
        httponly=True,
        secure=settings.cookie_secure,
        samesite=settings.cookie_samesite,
        max_age=settings.access_token_expire_minutes * 60,
        path="/",
    )


def clear_auth_cookie(response: Response) -> None:
    response.delete_cookie(
        key=settings.auth_cookie_name,
        httponly=True,
        secure=settings.cookie_secure,
        samesite=settings.cookie_samesite,
        path="/",
    )


def create_user_record(
    db: Session,
    username: str,
    password: str,
    *,
    is_admin: bool = False,
    must_change_password: bool = False,
) -> User:
    normalized_username = username.strip().lower()
    user = User(
        email=normalized_username,
        encrypted_email=encrypt_text(normalized_username),
        hashed_password=hash_password(password),
        is_admin=is_admin,
        must_change_password=must_change_password,
    )
    db.add(user)
    return user


async def get_current_user(
    cookie_token: str | None = Cookie(default=None, alias=settings.auth_cookie_name),
    credentials: HTTPAuthorizationCredentials | None = Depends(bearer_scheme),
    db: Session = Depends(get_db),
) -> User:
    """Decode the session token and load the user for protected routes."""
    token = cookie_token or (credentials.credentials if credentials else None)
    if not token:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid authentication credentials",
        )

    try:
        payload = jwt.decode(
            token,
            settings.secret_key,
            algorithms=[settings.algorithm],
        )
        user_id = int(payload["sub"])
    except (JWTError, KeyError, ValueError) as exc:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid authentication credentials",
        ) from exc

    user = db.get(User, user_id)
    if user is None:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid authentication credentials",
        )

    return user


@router.post(
    "/login",
    response_model=AuthResponse,
    summary="Authenticate user",
    description=(
        "Validates credentials and sets a secure HttpOnly session cookie. "
        "Failure responses are generic to avoid account enumeration."
    ),
    responses={
        200: {"description": "Authentication succeeded."},
        401: {"description": "Credentials are invalid."},
    },
)
async def login(
    payload: LoginRequest,
    response: Response,
    db: Session = Depends(get_db),
) -> AuthResponse:
    username = payload.username.strip().lower()
    user = db.query(User).filter(User.email == username).one_or_none()
    if user is None or not verify_password(payload.password, user.hashed_password):
        logger.warning("User login failed")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid credentials",
        )

    token = create_access_token({"sub": user.id})
    set_auth_cookie(response, token)
    logger.info("User login succeeded: user_id=%s", user.id)
    return AuthResponse(message="Authenticated")


@router.post(
    "/register",
    response_model=UserResponse,
    status_code=status.HTTP_201_CREATED,
    summary="Register user",
    description="Creates a user with Argon2 password hashing and duplicate protection.",
    responses={
        201: {"description": "User registered."},
        400: {"description": "Registration input is invalid."},
        409: {"description": "User already exists."},
    },
)
async def register(payload: RegisterRequest, db: Session = Depends(get_db)) -> User:
    username = payload.username.strip().lower()
    validate_password_strength(payload.password)
    existing_user = db.query(User).filter(User.email == username).one_or_none()
    if existing_user is not None:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail="User already exists",
        )

    user = create_user_record(db, username, payload.password)
    db.commit()
    db.refresh(user)
    logger.info("User registered: user_id=%s", user.id)
    return user


@router.post(
    "/logout",
    response_model=AuthResponse,
    summary="Clear session",
    description="Clears the HttpOnly session cookie.",
)
async def logout(response: Response) -> AuthResponse:
    clear_auth_cookie(response)
    return AuthResponse(message="Logged out")


@router.get(
    "/me",
    response_model=UserResponse,
    summary="Get current user",
    description="Returns the authenticated user's safe profile fields.",
    responses={
        200: {"description": "Current user profile."},
        401: {"description": "Missing or invalid bearer token."},
    },
)
async def read_current_user(
    current_user: User = Depends(get_current_user),
) -> User:
    return current_user
