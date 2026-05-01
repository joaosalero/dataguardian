from app.audit.engine import run_schema_audit


class AuditService:
    def run_project_audit(self) -> dict[str, int | list[dict[str, str]]]:
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

        return run_schema_audit(normalized_schema)
