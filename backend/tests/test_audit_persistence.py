from collections.abc import Generator

import app.models
import pytest
from fastapi import HTTPException
from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker
from sqlalchemy.pool import StaticPool

from app.core.database import Base
from app.models.audit_run import AuditRun
from app.models.finding import Finding
from app.models.user import User
from app.schemas.project import ProjectCreate
from app.services.audit_service import AuditService
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


def test_audit_run_persists_audit_and_findings(db_session: Session) -> None:
    user = create_user(db_session)
    project_service = ProjectService(db_session)
    created_project = project_service.create_project(
        ProjectCreate(name="Main Project", description="Seed"),
        user.id,
    )
    service = AuditService(db_session)

    result = service.run_project_audit(project_id=created_project.id, user_id=user.id)

    audit_run = db_session.query(AuditRun).filter(AuditRun.id == result["audit_id"]).one()
    findings = (
        db_session.query(Finding)
        .filter(Finding.audit_run_id == audit_run.id)
        .order_by(Finding.id)
        .all()
    )

    assert result["audit_id"] > 0
    assert audit_run.id == result["audit_id"]
    assert audit_run.project_id == created_project.id
    assert audit_run.status == "completed"
    assert audit_run.score == result["score"]
    assert len(findings) == len(result["findings"]) == 3
    assert all(finding.id > 0 for finding in findings)
    assert all(finding.audit_run_id == result["audit_id"] for finding in findings)


def test_audit_history_returns_saved_audits_for_project_owner(
    db_session: Session,
) -> None:
    user = create_user(db_session)
    audit_service = AuditService(db_session)
    project_service = ProjectService(db_session)
    created_project = project_service.create_project(
        ProjectCreate(name="History Project", description="Seed"),
        user.id,
    )

    result = audit_service.run_project_audit(
        project_id=created_project.id,
        user_id=user.id,
    )
    history = audit_service.get_audit_history(
        project_id=created_project.id,
        user_id=user.id,
    )

    assert history == [
        {
            "audit_id": result["audit_id"],
            "project_id": created_project.id,
            "status": "completed",
            "score": result["score"],
        }
    ]


def test_audit_service_blocks_foreign_project_access(db_session: Session) -> None:
    owner = create_user(db_session, "owner@example.com")
    other = create_user(db_session, "other@example.com")
    project_service = ProjectService(db_session)
    created_project = project_service.create_project(
        ProjectCreate(name="Private Audit Project", description="Seed"),
        owner.id,
    )
    audit_service = AuditService(db_session)

    with pytest.raises(HTTPException) as run_exc:
        audit_service.run_project_audit(
            project_id=created_project.id,
            user_id=other.id,
        )

    with pytest.raises(HTTPException) as history_exc:
        audit_service.get_audit_history(
            project_id=created_project.id,
            user_id=other.id,
        )

    assert run_exc.value.status_code == 404
    assert run_exc.value.detail == "Project not found"
    assert history_exc.value.status_code == 404
    assert history_exc.value.detail == "Project not found"


def create_user(db_session: Session, email: str = "owner@example.com") -> User:
    user = User(email=email, hashed_password="hashed")
    db_session.add(user)
    db_session.commit()
    db_session.refresh(user)
    return user
