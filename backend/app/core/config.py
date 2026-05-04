from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv


ROOT_DIR = Path(__file__).resolve().parents[3]
ENV_FILE = ROOT_DIR / ".env"
_raw_environment = os.getenv("ENVIRONMENT", "dev").strip().lower()

load_dotenv(
    dotenv_path=ENV_FILE,
    override=_raw_environment not in {"prod", "production"},
)

DEFAULT_APP_NAME = "DataGuardian"
DEFAULT_ENVIRONMENT = "dev"
DEFAULT_DEBUG = True
DEFAULT_API_PREFIX = "/api"
DEFAULT_SECRET_KEY = "development-only-secret-key-change-before-production"
DEFAULT_ALGORITHM = "RS256"
DEFAULT_ACCESS_TOKEN_EXPIRE_MINUTES = 30
DEFAULT_DATABASE_URL = "postgresql://dataguardian:dataguardian@localhost:5434/dataguardian"
DEFAULT_AUTH_COOKIE_NAME = "dataguardian_session"
DEFAULT_COOKIE_SAMESITE = "lax"
DEFAULT_AUTH_RATE_LIMIT_MAX_REQUESTS = 20
DEFAULT_AUTH_RATE_LIMIT_WINDOW_SECONDS = 60
MINIMUM_SECRET_KEY_LENGTH = 32
UNSAFE_SECRET_KEYS = {
    "supersecretkey",
    "change-this-in-production",
    "change-this-in-ci",
    DEFAULT_SECRET_KEY,
}


def _get_bool_env(name: str, default: bool) -> bool:
    raw_value = os.getenv(name)
    if raw_value is None:
        return default

    return raw_value.strip().lower() in {"1", "true", "yes", "on"}


def _normalize_environment(value: str) -> str:
    normalized = value.strip().lower()
    if normalized in {"development", "dev", "local"}:
        return "dev"
    if normalized in {"production", "prod"}:
        return "prod"
    if normalized == "test":
        return "test"
    return normalized


@dataclass(frozen=True, slots=True)
class Settings:
    app_name: str = DEFAULT_APP_NAME
    environment: str = DEFAULT_ENVIRONMENT
    debug: bool = DEFAULT_DEBUG
    api_prefix: str = DEFAULT_API_PREFIX
    secret_key: str = DEFAULT_SECRET_KEY
    algorithm: str = DEFAULT_ALGORITHM
    access_token_expire_minutes: int = DEFAULT_ACCESS_TOKEN_EXPIRE_MINUTES
    database_url: str = DEFAULT_DATABASE_URL
    auth_cookie_name: str = DEFAULT_AUTH_COOKIE_NAME
    cookie_samesite: str = DEFAULT_COOKIE_SAMESITE
    jwt_private_key: str | None = None
    jwt_public_key: str | None = None
    encryption_key: str | None = None
    admin_email: str | None = None
    admin_password: str | None = None
    auth_rate_limit_max_requests: int = DEFAULT_AUTH_RATE_LIMIT_MAX_REQUESTS
    auth_rate_limit_window_seconds: int = DEFAULT_AUTH_RATE_LIMIT_WINDOW_SECONDS

    @property
    def is_production(self) -> bool:
        return self.environment == "prod"

    @property
    def is_dev_or_test(self) -> bool:
        return self.environment in {"dev", "test"}

    @property
    def cookie_secure(self) -> bool:
        return self.is_production

    @classmethod
    def from_env(cls) -> Settings:
        environment = _normalize_environment(os.getenv("ENVIRONMENT", DEFAULT_ENVIRONMENT))
        secret_key = os.getenv("SECRET_KEY", DEFAULT_SECRET_KEY)
        jwt_private_key = os.getenv("JWT_PRIVATE_KEY")
        jwt_public_key = os.getenv("JWT_PUBLIC_KEY")
        encryption_key = os.getenv("ENCRYPTION_KEY")
        if environment == "prod" and (
            secret_key in UNSAFE_SECRET_KEYS
            or len(secret_key) < MINIMUM_SECRET_KEY_LENGTH
        ):
            raise ValueError("A strong SECRET_KEY is required in production")
        if environment == "prod" and (not jwt_private_key or not jwt_public_key):
            raise ValueError("JWT_PRIVATE_KEY and JWT_PUBLIC_KEY are required in production")
        if environment == "prod" and not encryption_key:
            raise ValueError("ENCRYPTION_KEY is required in production")

        return cls(
            app_name=os.getenv("APP_NAME", DEFAULT_APP_NAME),
            environment=environment,
            debug=_get_bool_env("DEBUG", DEFAULT_DEBUG),
            api_prefix=os.getenv("API_PREFIX", DEFAULT_API_PREFIX),
            secret_key=secret_key,
            algorithm=DEFAULT_ALGORITHM,
            access_token_expire_minutes=int(
                os.getenv(
                    "ACCESS_TOKEN_EXPIRE_MINUTES",
                    str(DEFAULT_ACCESS_TOKEN_EXPIRE_MINUTES),
                )
            ),
            database_url=os.getenv("DATABASE_URL", DEFAULT_DATABASE_URL),
            auth_cookie_name=os.getenv("AUTH_COOKIE_NAME", DEFAULT_AUTH_COOKIE_NAME),
            cookie_samesite=os.getenv("COOKIE_SAMESITE", DEFAULT_COOKIE_SAMESITE),
            jwt_private_key=jwt_private_key,
            jwt_public_key=jwt_public_key,
            encryption_key=encryption_key,
            admin_email=os.getenv("ADMIN_EMAIL"),
            admin_password=os.getenv("ADMIN_PASSWORD"),
            auth_rate_limit_max_requests=int(
                os.getenv(
                    "AUTH_RATE_LIMIT_MAX_REQUESTS",
                    str(DEFAULT_AUTH_RATE_LIMIT_MAX_REQUESTS),
                )
            ),
            auth_rate_limit_window_seconds=int(
                os.getenv(
                    "AUTH_RATE_LIMIT_WINDOW_SECONDS",
                    str(DEFAULT_AUTH_RATE_LIMIT_WINDOW_SECONDS),
                )
            ),
        )


settings = Settings.from_env()
