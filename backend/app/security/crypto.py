from __future__ import annotations

import logging

from cryptography.fernet import Fernet

from app.core.config import settings


logger = logging.getLogger(__name__)
_runtime_fernet_key = Fernet.generate_key()


def _get_fernet() -> Fernet:
    key = settings.encryption_key.encode() if settings.encryption_key else _runtime_fernet_key
    return Fernet(key)


def encrypt_text(value: str) -> str:
    return _get_fernet().encrypt(value.encode()).decode()


def decrypt_text(value: str) -> str:
    return _get_fernet().decrypt(value.encode()).decode()
