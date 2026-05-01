import logging

from fastapi import HTTPException, status
from sqlalchemy.exc import SQLAlchemyError
from sqlalchemy.orm import Session

from app.audit.engine import run_schema_audit
from app.models.audit_run import AuditRun
from app.models.finding import Finding
from app.repositories.project_repository import ProjectRepository


logger = logging.getLogger(__name__)


class AuditService:
    def __init__(self, db: Session) -> None:
        self.db = db
        self.project_repository = ProjectRepository(db)

    def run_project_audit(self, project_id: int) -> dict[str, int | list[dict[str, str]]]:
        project = self.project_repository.get_by_id(project_id)
        if project is None:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="Project not found",
            )

        logger.info("Audit started for project %s", project.id)
        simulated_schema = {
            "users": {
                "columns": ["id", "email", "password"],
                "has_pk": True,
            },
            "logs": {
                "columns": ["event", "timestamp"],
                "has_pk": False,
            },
        }

        normalized_schema = {
            table_name: {
                "columns": table_definition["columns"],
                "primary_key": "id" if table_definition["has_pk"] else None,
            }
            for table_name, table_definition in simulated_schema.items()
        }

        result = run_schema_audit(normalized_schema)
        audit_id = self._persist_audit_result(project_id, result)

        return {
            "audit_id": audit_id,
            "score": result["score"],
            "findings": result["findings"],
        }

    def get_audit_history(self, project_id: int) -> list[dict[str, int | str]]:
        audit_runs = (
            self.db.query(AuditRun)
            .filter(AuditRun.project_id == project_id)
            .order_by(AuditRun.id.desc())
            .all()
        )

        return [
            {
                "audit_id": audit_run.id,
                "project_id": audit_run.project_id,
                "status": audit_run.status,
                "score": audit_run.score,
            }
            for audit_run in audit_runs
        ]

    def _persist_audit_result(
        self,
        project_id: int,
        result: dict[str, int | list[dict[str, str]]],
    ) -> int:
        audit_run = AuditRun(
            project_id=project_id,
            status="completed",
            score=int(result["score"]),
        )

        try:
            self.db.add(audit_run)
            self.db.commit()
            self.db.refresh(audit_run)
            logger.info("AuditRun persisted with ID: %s", audit_run.id)

            findings = [
                Finding(
                    audit_run_id=audit_run.id,
                    title=finding["title"],
                    description=finding["description"],
                    severity=finding["severity"],
                    recommendation=finding["recommendation"],
                )
                for finding in result["findings"]
            ]

            self.db.add_all(findings)
            self.db.commit()
            return audit_run.id
        except SQLAlchemyError:
            self.db.rollback()
            raise
