from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv


ROOT_DIR = Path(__file__).resolve().parents[3]
ENV_FILE = ROOT_DIR / ".env"

load_dotenv(
    dotenv_path=ENV_FILE,
    override=os.getenv("ENVIRONMENT", "development") != "production",
)

DEFAULT_APP_NAME = "DataGuardian"
DEFAULT_ENVIRONMENT = "development"
DEFAULT_DEBUG = True
DEFAULT_API_PREFIX = "/api"
DEFAULT_SECRET_KEY = "supersecretkey"
DEFAULT_ALGORITHM = "HS256"
DEFAULT_ACCESS_TOKEN_EXPIRE_MINUTES = 30
DEFAULT_DATABASE_URL = "postgresql://dataguardian:dataguardian@localhost:5434/dataguardian"


def _get_bool_env(name: str, default: bool) -> bool:
    raw_value = os.getenv(name)
    if raw_value is None:
        return default

    return raw_value.strip().lower() in {"1", "true", "yes", "on"}


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

    @classmethod
    def from_env(cls) -> Settings:
        return cls(
            app_name=os.getenv("APP_NAME", DEFAULT_APP_NAME),
            environment=os.getenv("ENVIRONMENT", DEFAULT_ENVIRONMENT),
            debug=_get_bool_env("DEBUG", DEFAULT_DEBUG),
            api_prefix=os.getenv("API_PREFIX", DEFAULT_API_PREFIX),
            secret_key=os.getenv("SECRET_KEY", DEFAULT_SECRET_KEY),
            algorithm=os.getenv("ALGORITHM", DEFAULT_ALGORITHM),
            access_token_expire_minutes=int(
                os.getenv(
                    "ACCESS_TOKEN_EXPIRE_MINUTES",
                    str(DEFAULT_ACCESS_TOKEN_EXPIRE_MINUTES),
                )
            ),
            database_url=os.getenv("DATABASE_URL", DEFAULT_DATABASE_URL),
        )


settings = Settings.from_env()
