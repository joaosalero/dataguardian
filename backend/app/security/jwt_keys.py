from __future__ import annotations

from functools import lru_cache

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from app.core.config import settings


@lru_cache(maxsize=1)
def _dev_key_pair() -> tuple[str, str]:
    private_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    ).decode()
    public_pem = private_key.public_key().public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    ).decode()
    return private_pem, public_pem


def get_jwt_private_key() -> str:
    if settings.jwt_private_key:
        return settings.jwt_private_key.replace("\\n", "\n")
    if settings.is_production:
        raise RuntimeError("JWT_PRIVATE_KEY is required in production")
    return _dev_key_pair()[0]


def get_jwt_public_key() -> str:
    if settings.jwt_public_key:
        return settings.jwt_public_key.replace("\\n", "\n")
    if settings.is_production:
        raise RuntimeError("JWT_PUBLIC_KEY is required in production")
    return _dev_key_pair()[1]
