from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv


ROOT_DIR = Path(__file__).resolve().parents[3]
ENV_FILE = ROOT_DIR / ".env"

load_dotenv(dotenv_path=ENV_FILE)


def _get_bool_env(name: str, default: bool) -> bool:
    raw_value = os.getenv(name)
    if raw_value is None:
        return default

    return raw_value.strip().lower() in {"1", "true", "yes", "on"}


@dataclass(frozen=True, slots=True)
class Settings:
    app_name: str = "DataGuardian"
    environment: str = "development"
    debug: bool = True
    api_prefix: str = "/api"
    secret_key: str = "supersecretkey"
    database_url: str = "postgresql://user:password@localhost:5432/dataguardian"

    @classmethod
    def from_env(cls) -> Settings:
        return cls(
            app_name=os.getenv("APP_NAME", cls.app_name),
            environment=os.getenv("ENVIRONMENT", cls.environment),
            debug=_get_bool_env("DEBUG", cls.debug),
            api_prefix=os.getenv("API_PREFIX", cls.api_prefix),
            secret_key=os.getenv("SECRET_KEY", cls.secret_key),
            database_url=os.getenv("DATABASE_URL", cls.database_url),
        )


settings = Settings.from_env()
