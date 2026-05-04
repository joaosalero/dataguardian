from collections.abc import AsyncGenerator
import logging
import secrets
import sys

from sqlalchemy import create_engine, inspect, text
from sqlalchemy.orm import Session, declarative_base, sessionmaker

from app.core.config import settings


logger = logging.getLogger(__name__)

engine = create_engine(settings.database_url, pool_pre_ping=True)
SessionLocal = sessionmaker(
    autocommit=False,
    autoflush=False,
    bind=engine,
)
Base = declarative_base()


def init_db() -> None:
    """Create known application tables without logging connection details."""
    import app.models  # Ensures model metadata is registered before table creation.

    Base.metadata.create_all(bind=engine)
    ensure_user_security_columns()
    bootstrap_required_users()
    logger.info("Database metadata initialized")


def ensure_user_security_columns() -> None:
    inspector = inspect(engine)
    if "users" not in inspector.get_table_names():
        return

    existing_columns = {column["name"] for column in inspector.get_columns("users")}
    column_definitions = {
        "encrypted_email": "TEXT",
        "is_admin": "BOOLEAN NOT NULL DEFAULT FALSE",
        "must_change_password": "BOOLEAN NOT NULL DEFAULT FALSE",
    }
    with engine.begin() as connection:
        for column_name, column_definition in column_definitions.items():
            if column_name not in existing_columns:
                connection.execute(
                    text(f"ALTER TABLE users ADD COLUMN {column_name} {column_definition}")
                )


def bootstrap_required_users() -> None:
    from app.auth import create_user_record
    from app.models.user import User

    db = SessionLocal()
    try:
        has_users = db.query(User.id).first() is not None
        if not has_users:
            if settings.is_production:
                if settings.admin_email and settings.admin_password:
                    create_user_record(
                        db,
                        settings.admin_email,
                        settings.admin_password,
                        is_admin=True,
                        must_change_password=True,
                    )
                    logger.info("Production admin user bootstrapped")
            else:
                admin_password = secrets.token_urlsafe(18)
                create_user_record(
                    db,
                    "admin",
                    admin_password,
                    is_admin=True,
                    must_change_password=True,
                )
                if sys.stdout.isatty():
                    print("[BOOTSTRAP] Admin user created: username=admin")
                    print(f"[BOOTSTRAP] Temporary admin password: {admin_password}")
                else:
                    logger.warning(
                        "Local admin user bootstrapped; run scripts/create_user.py "
                        "to create named users without logging credentials"
                    )

        if settings.is_dev_or_test:
            test_user = db.query(User).filter(User.email == "test").one_or_none()
            if test_user is None:
                create_user_record(db, "test", "test123")
                logger.info("Local test user ensured")
        db.commit()
    finally:
        db.close()


async def get_db() -> AsyncGenerator[Session, None]:
    """Provide one SQLAlchemy session per request and always close it."""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
