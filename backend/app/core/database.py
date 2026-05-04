from collections.abc import AsyncGenerator
import logging

from sqlalchemy import create_engine
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
    logger.info("Database metadata initialized")


async def get_db() -> AsyncGenerator[Session, None]:
    """Provide one SQLAlchemy session per request and always close it."""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
