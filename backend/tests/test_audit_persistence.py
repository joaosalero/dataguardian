from collections.abc import Generator

import app.models
import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import Session, sessionmaker
from sqlalchemy.pool import StaticPool

from app.api.audit_routes import get_audit_history
from app.core.database import Base
from app.models.audit_run import AuditRun
from app.models.finding import Finding
from app.models.project import Project
from app.models.user import User
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
        user = User(email="owner@dataguardian.dev", password_hash="hashed")
        db.add(user)
        db.flush()
        db.add(Project(name="Main Project", description="Seed", user_id=user.id))
        db.commit()
        yield db
    finally:
        db.close()


def test_audit_run_persists_audit_and_findings(db_session: Session) -> None:
    service = AuditService(db_session)

    result = service.run_project_audit(project_id=1)

    audit_run = db_session.query(AuditRun).filter(AuditRun.id == result["audit_id"]).one()
    findings = (
        db_session.query(Finding)
        .filter(Finding.audit_run_id == audit_run.id)
        .order_by(Finding.id)
        .all()
    )

    assert result["audit_id"] > 0
    assert audit_run.project_id == 1
    assert audit_run.status == "completed"
    assert audit_run.score == result["score"]
    assert len(findings) == len(result["findings"]) == 3


@pytest.mark.asyncio
async def test_audit_history_endpoint_returns_saved_audits(db_session: Session) -> None:
    audit_service = AuditService(db_session)
    project_service = ProjectService(db_session)

    result = audit_service.run_project_audit(project_id=1)
    history = await get_audit_history(
        project_id=1,
        project_service=project_service,
        audit_service=audit_service,
    )

    assert history == [
        {
            "audit_id": result["audit_id"],
            "project_id": 1,
            "status": "completed",
            "score": result["score"],
        }
    ]
