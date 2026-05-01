from app.audit.engine import run_schema_audit


def test_detects_suspicious_password_column() -> None:
    schema = {
        "users": {
            "columns": ["id", "password_hash"],
            "primary_key": "id",
        }
    }

    result = run_schema_audit(schema)

    assert any(
        finding["title"] == "Suspicious column detected"
        and "password_hash" in finding["description"]
        and finding["severity"] == "HIGH"
        for finding in result["findings"]
    )


def test_detects_table_without_primary_key() -> None:
    schema = {
        "audit_logs": {
            "columns": ["event", "created_at"],
            "primary_key": None,
        }
    }

    result = run_schema_audit(schema)

    assert any(
        finding["title"] == "Table without primary key"
        and "audit_logs" in finding["description"]
        and finding["severity"] == "MEDIUM"
        for finding in result["findings"]
    )


def test_detects_pii_column() -> None:
    schema = {
        "customers": {
            "columns": ["id", "email_address"],
            "primary_key": "id",
        }
    }

    result = run_schema_audit(schema)

    assert any(
        finding["title"] == "Possible PII column detected"
        and "email_address" in finding["description"]
        and finding["severity"] == "MEDIUM"
        for finding in result["findings"]
    )


def test_calculates_score_correctly() -> None:
    schema = {
        "users": {
            "columns": ["id", "password", "email"],
            "primary_key": None,
        }
    }

    result = run_schema_audit(schema)

    assert result["score"] == 88
    assert len(result["findings"]) == 3


def test_clean_schema_returns_full_score() -> None:
    schema = {
        "projects": {
            "columns": ["id", "name", "created_at"],
            "primary_key": "id",
        }
    }

    result = run_schema_audit(schema)

    assert result["score"] == 100
    assert result["findings"] == []
