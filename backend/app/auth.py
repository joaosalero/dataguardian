from datetime import datetime, timedelta, timezone
import logging
from typing import Any
import warnings

from fastapi import APIRouter, Depends, HTTPException, status
from fastapi.security import OAuth2PasswordBearer
from jose import JWTError, jwt

warnings.filterwarnings(
    "ignore",
    message="'crypt' is deprecated and slated for removal in Python 3.13",
    category=DeprecationWarning,
)

from passlib.context import CryptContext
from pydantic import BaseModel
from sqlalchemy.orm import Session

from app.core.config import settings
from app.core.database import get_db
from app.models.user import User


logger = logging.getLogger(__name__)
router = APIRouter(prefix="/auth", tags=["auth"])
oauth2_scheme = OAuth2PasswordBearer(tokenUrl="/auth/login")
pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")


class LoginRequest(BaseModel):
    username: str
    password: str


class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "bearer"


class UserResponse(BaseModel):
    id: int
    email: str
    created_at: datetime

    model_config = {"from_attributes": True}


def hash_password(password: str) -> str:
    """Hash a plaintext password for storage; callers must never log the result."""
    return pwd_context.hash(password)


def verify_password(plain: str, hashed: str) -> bool:
    """Verify a password without exposing whether the user lookup or hash check failed."""
    return pwd_context.verify(plain, hashed)


def create_access_token(data: dict[str, Any]) -> str:
    """Create a short-lived signed JWT containing only non-sensitive claims."""
    expires_at = datetime.now(timezone.utc) + timedelta(
        minutes=settings.access_token_expire_minutes
    )
    payload = {**data, "exp": expires_at}
    return jwt.encode(payload, settings.secret_key, algorithm=settings.algorithm)


async def get_current_user(
    token: str = Depends(oauth2_scheme),
    db: Session = Depends(get_db),
) -> User:
    """Decode the bearer token and load the user for protected routes."""
    try:
        payload = jwt.decode(
            token,
            settings.secret_key,
            algorithms=[settings.algorithm],
        )
        user_id = int(payload["user_id"])
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
    response_model=TokenResponse,
    summary="Authenticate user",
    description=(
        "Validates credentials and returns a bearer token. Failure responses are "
        "generic to avoid account enumeration."
    ),
    responses={
        200: {"description": "Authentication succeeded."},
        401: {"description": "Credentials are invalid."},
    },
)
async def login(payload: LoginRequest, db: Session = Depends(get_db)) -> TokenResponse:
    username = payload.username.strip().lower()
    user = db.query(User).filter(User.email == username).one_or_none()
    if user is None or not verify_password(payload.password, user.hashed_password):
        logger.warning("User login failed")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Invalid credentials",
        )

    token = create_access_token({"user_id": user.id})
    logger.info("User login succeeded: user_id=%s", user.id)
    return TokenResponse(access_token=token)


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
