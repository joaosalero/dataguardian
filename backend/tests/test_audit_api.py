import pytest
from fastapi import HTTPException, status

from app.api.audit_routes import get_audit_history, run_audit


class FakeUser:
    id = 1


class FakeAuditService:
    def __init__(self, exists: bool = True) -> None:
        self.exists = exists

    def run_project_audit(
        self,
        project_id: int,
        user_id: int,
    ) -> dict[str, int | list[dict[str, str]]]:
        if not self.exists:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="Project not found",
            )
        return {
            "audit_id": 7,
            "score": 80,
            "findings": [
                {
                    "title": "Sensitive field detected",
                    "description": (
                        "Sensitive field detected: table 'users' column "
                        "'password' matched 'password'."
                    ),
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
                    "title": "Possible PII field detected",
                    "description": (
                        "Possible PII field detected: table 'users' column "
                        "'email' matched 'email'."
                    ),
                    "severity": "MEDIUM",
                    "recommendation": "Review personal data controls.",
                },
            ],
        }

    def get_audit_history(
        self,
        project_id: int,
        user_id: int,
    ) -> list[dict[str, int | str]]:
        if not self.exists:
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail="Project not found",
            )
        return [
            {
                "audit_id": 7,
                "project_id": project_id,
                "status": "completed",
                "score": 80,
            }
        ]


@pytest.mark.asyncio
async def test_audit_runs_successfully() -> None:
    result = await run_audit(
        project_id=1,
        current_user=FakeUser(),
        audit_service=FakeAuditService(),
    )

    assert result["score"] == 80
    assert result["audit_id"] == 7
    assert isinstance(result["findings"], list)
    assert len(result["findings"]) == 3


@pytest.mark.asyncio
async def test_audit_invalid_project_returns_404() -> None:
    with pytest.raises(HTTPException) as exc_info:
        await run_audit(
            project_id=999,
            current_user=FakeUser(),
            audit_service=FakeAuditService(exists=False),
        )

    assert exc_info.value.status_code == 404
    assert exc_info.value.detail == "Project not found"


@pytest.mark.asyncio
async def test_audit_history_returns_audits() -> None:
    result = await get_audit_history(
        project_id=1,
        current_user=FakeUser(),
        audit_service=FakeAuditService(),
    )

    assert result == [
        {
            "audit_id": 7,
            "project_id": 1,
            "status": "completed",
            "score": 80,
        }
    ]
