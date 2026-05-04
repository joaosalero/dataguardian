RISK_POINTS_BY_SEVERITY = {
    "LOW": 2,
    "MEDIUM": 5,
    "HIGH": 10,
    "CRITICAL": 20,
}


def calculate_score(findings: list[dict[str, str]]) -> int:
    """Convert findings into a bounded 0-100 score; more severe findings cost more."""
    total_risk_points = sum(
        RISK_POINTS_BY_SEVERITY.get(finding["severity"], 0)
        for finding in findings
    )
    return max(0, 100 - total_risk_points)
