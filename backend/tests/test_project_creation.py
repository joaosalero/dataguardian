from collections.abc import Generator

import app.models
import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker
from sqlalchemy.pool import StaticPool

from app.core.database import Base
from app.models.user import User
from app.schemas.project import ProjectCreate
from app.services.project_service import ProjectService


engine = create_engine(
    "sqlite://",
    connect_args={"check_same_thread": False},
    poolclass=StaticPool,
)
TestingSessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)


@pytest.fixture(autouse=True)
def setup_database() -> Generator[None, None, None]:
    Base.metadata.create_all(bind=engine)
    yield
    Base.metadata.drop_all(bind=engine)


@pytest.fixture
def db_session() -> Generator[Session, None, None]:
    db = TestingSessionLocal()
    try:
        yield db
    finally:
        db.close()


def test_project_creation_and_fetch(db_session: Session) -> None:
    user = User(email="owner@example.com", hashed_password="hashed")
    db_session.add(user)
    db_session.commit()
    db_session.refresh(user)
    service = ProjectService(db_session)

    created_project = service.create_project(
        ProjectCreate(
            name="Security Audit",
            description="Core platform review",
        ),
        user.id,
    )

    fetched_project = service.get_project(created_project.id, user.id)

    assert created_project.id > 0
    assert fetched_project.id == created_project.id
    assert fetched_project.name == "Security Audit"
    assert fetched_project.user_id == user.id
