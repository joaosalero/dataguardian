COLUMN_RULES = [
    {
        "name": "detect_sensitive_field",
        "patterns": ["password", "senha", "token", "secret", "api_key"],
        "title": "Sensitive field detected",
        "severity": "HIGH",
        "description": "Sensitive field detected",
        "recommendation": "Review storage and protection controls for sensitive secrets.",
    },
    {
        "name": "detect_pii_field",
        "patterns": ["email", "phone", "telefone", "cpf", "document", "documento"],
        "title": "Possible PII field detected",
        "severity": "MEDIUM",
        "description": "Possible PII field detected",
        "recommendation": (
            "Review masking, encryption, and access controls for personal data."
        ),
    },
]


def _normalize_name(value: str) -> str:
    return value.strip().lower()


def _contains_keyword(column_name: str, keywords: list[str]) -> str | None:
    normalized_column_name = _normalize_name(column_name)
    for keyword in keywords:
        if keyword in normalized_column_name:
            return keyword
    return None


def apply_column_rules(table_name: str, columns: list[str]) -> list[dict[str, str]]:
    findings: list[dict[str, str]] = []
    for column_name in columns:
        for rule in COLUMN_RULES:
            matched_keyword = _contains_keyword(column_name, rule["patterns"])
            if matched_keyword is None:
                continue

            findings.append(
                {
                    "title": rule["title"],
                    "description": (
                        f"{rule['description']}: table '{table_name}' column "
                        f"'{column_name}' matched '{matched_keyword}'."
                    ),
                    "severity": rule["severity"],
                    "recommendation": rule["recommendation"],
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
