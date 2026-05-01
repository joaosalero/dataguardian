SUSPICIOUS_COLUMN_NAMES = {
    "password",
    "senha",
    "token",
    "secret",
    "api_key",
}

PII_COLUMN_NAMES = {
    "email",
    "phone",
    "telefone",
    "cpf",
    "document",
    "documento",
}


def _normalize_name(value: str) -> str:
    return value.strip().lower()


def _contains_keyword(column_name: str, keywords: set[str]) -> str | None:
    normalized_column_name = _normalize_name(column_name)
    for keyword in keywords:
        if keyword in normalized_column_name:
            return keyword
    return None


def check_suspicious_columns(table_name: str, columns: list[str]) -> list[dict[str, str]]:
    findings: list[dict[str, str]] = []
    for column_name in columns:
        matched_keyword = _contains_keyword(column_name, SUSPICIOUS_COLUMN_NAMES)
        if matched_keyword is None:
            continue

        findings.append(
            {
                "title": "Suspicious column detected",
                "description": (
                    f"Table '{table_name}' contains suspicious column "
                    f"'{column_name}' related to '{matched_keyword}'."
                ),
                "severity": "HIGH",
                "recommendation": (
                    "Review storage and protection controls for sensitive secrets."
                ),
            }
        )
    return findings


def check_missing_primary_key(
    table_name: str,
    primary_key: str | list[str] | None,
) -> list[dict[str, str]]:
    if primary_key is None:
        return [
            {
                "title": "Table without primary key",
                "description": f"Table '{table_name}' does not define a primary key.",
                "severity": "MEDIUM",
                "recommendation": "Add a primary key to ensure integrity and traceability.",
            }
        ]

    if isinstance(primary_key, list) and not primary_key:
        return [
            {
                "title": "Table without primary key",
                "description": f"Table '{table_name}' does not define a primary key.",
                "severity": "MEDIUM",
                "recommendation": "Add a primary key to ensure integrity and traceability.",
            }
        ]

    return []


def check_pii_columns(table_name: str, columns: list[str]) -> list[dict[str, str]]:
    findings: list[dict[str, str]] = []
    for column_name in columns:
        matched_keyword = _contains_keyword(column_name, PII_COLUMN_NAMES)
        if matched_keyword is None:
            continue

        findings.append(
            {
                "title": "Possible PII column detected",
                "description": (
                    f"Table '{table_name}' contains possible PII column "
                    f"'{column_name}' related to '{matched_keyword}'."
                ),
                "severity": "MEDIUM",
                "recommendation": (
                    "Review masking, encryption, and access controls for personal data."
                ),
            }
        )
    return findings
