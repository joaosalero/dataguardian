from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from app.core.database import get_db
from app.models.user import User
from app.security.auth import get_current_user
from app.services.audit_service import AuditService
from app.services.project_service import ProjectService


router = APIRouter(prefix="/projects", tags=["audits"])


async def get_audit_service(db: Session = Depends(get_db)) -> AuditService:
    return AuditService(db)


async def get_project_service(db: Session = Depends(get_db)) -> ProjectService:
    return ProjectService(db)


@router.post("/{project_id}/audit/run")
async def run_audit(
    project_id: int,
    current_user: User = Depends(get_current_user),
    project_service: ProjectService = Depends(get_project_service),
    audit_service: AuditService = Depends(get_audit_service),
) -> dict[str, int | list[dict[str, str]]]:
    project_service.get_project(project_id, current_user.id)
    return audit_service.run_project_audit(project_id)


@router.get("/{project_id}/audit/history")
async def get_audit_history(
    project_id: int,
    current_user: User = Depends(get_current_user),
    project_service: ProjectService = Depends(get_project_service),
    audit_service: AuditService = Depends(get_audit_service),
) -> list[dict[str, int | str]]:
    project_service.get_project(project_id, current_user.id)
    return audit_service.get_audit_history(project_id)
