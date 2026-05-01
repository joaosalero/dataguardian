import pytest
from fastapi import HTTPException, status

from app.api.audit_routes import get_audit_history, run_audit


class FakeUser:
    id = 1


class FakeProjectService:
    def __init__(self, exists: bool) -> None:
        self.exists = exists

    def get_project(self, project_id: int, user_id: int) -> dict[str, int]:
        if not self.exists:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="Project not found",
            )
        return {"id": project_id}


class FakeAuditService:
    def run_project_audit(self, project_id: int) -> dict[str, int | list[dict[str, str]]]:
        return {
            "audit_id": 7,
            "score": 88,
            "findings": [
                {
                    "title": "Suspicious column detected",
                    "description": "Table 'users' contains suspicious column 'password'.",
                    "severity": "HIGH",
                    "recommendation": "Review storage and protection controls.",
                },
                {
                    "title": "Table without primary key",
                    "description": "Table 'logs' does not define a primary key.",
                    "severity": "MEDIUM",
                    "recommendation": "Add a primary key.",
                },
                {
                    "title": "Possible PII column detected",
                    "description": "Table 'users' contains possible PII column 'email'.",
                    "severity": "MEDIUM",
                    "recommendation": "Review personal data controls.",
                },
            ],
        }

    def get_audit_history(self, project_id: int) -> list[dict[str, int | str]]:
        return [
            {
                "audit_id": 7,
                "project_id": project_id,
                "status": "completed",
                "score": 88,
            }
        ]


@pytest.mark.asyncio
async def test_audit_runs_successfully() -> None:
    result = await run_audit(
        project_id=1,
        current_user=FakeUser(),
        project_service=FakeProjectService(exists=True),
        audit_service=FakeAuditService(),
    )

    assert result["score"] == 88
    assert result["audit_id"] == 7
    assert isinstance(result["findings"], list)
    assert len(result["findings"]) == 3


@pytest.mark.asyncio
async def test_audit_invalid_project_returns_404() -> None:
    with pytest.raises(HTTPException) as exc_info:
        await run_audit(
            project_id=999,
            current_user=FakeUser(),
            project_service=FakeProjectService(exists=False),
            audit_service=FakeAuditService(),
        )

    assert exc_info.value.status_code == 404
    assert exc_info.value.detail == "Project not found"


@pytest.mark.asyncio
async def test_audit_history_returns_audits() -> None:
    result = await get_audit_history(
        project_id=1,
        current_user=FakeUser(),
        project_service=FakeProjectService(exists=True),
        audit_service=FakeAuditService(),
    )

    assert result == [
        {
            "audit_id": 7,
            "project_id": 1,
            "status": "completed",
            "score": 88,
        }
    ]
