import logging

from sqlalchemy.exc import SQLAlchemyError
from sqlalchemy.orm import Session

from app.audit.engine import run_schema_audit
from app.models.audit_run import AuditRun
from app.models.finding import Finding


class AuditService:
    def __init__(self, db: Session) -> None:
        self.db = db

    def run_project_audit(self, project_id: int) -> dict[str, int | list[dict[str, str]]]:
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
        try:
            audit_runs = (
                self.db.query(AuditRun)
                .filter(AuditRun.project_id == project_id)
                .order_by(AuditRun.id.desc())
                .all()
            )
        except SQLAlchemyError:
            logger.warning("Database unavailable, returning empty audit history")
            return []

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
        try:
            audit_run = AuditRun(
                project_id=project_id,
                status="completed",
                score=int(result["score"]),
            )
            self.db.add(audit_run)
            self.db.flush()

            for finding in result["findings"]:
                self.db.add(
                    Finding(
                        audit_run_id=audit_run.id,
                        title=finding["title"],
                        description=finding["description"],
                        severity=finding["severity"],
                        recommendation=finding["recommendation"],
                    )
                )

            self.db.commit()
            self.db.refresh(audit_run)
            return audit_run.id
        except SQLAlchemyError:
            self.db.rollback()
            logger.warning("Database unavailable, returning non-persisted audit result")
            return 0


logger = logging.getLogger(__name__)
