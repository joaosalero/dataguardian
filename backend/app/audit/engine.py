from app.audit.rules import apply_column_rules, check_missing_primary_key
from app.audit.scoring import calculate_score


def run_schema_audit(schema: dict) -> dict[str, int | list[dict[str, str]]]:
    findings: list[dict[str, str]] = []

    for table_name, table_definition in schema.items():
        columns = table_definition.get("columns", [])
        primary_key = table_definition.get("primary_key")

        findings.extend(apply_column_rules(table_name, columns))
        findings.extend(check_missing_primary_key(table_name, primary_key))

    return {
        "score": calculate_score(findings),
        "findings": findings,
    }
