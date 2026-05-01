RISK_POINTS_BY_SEVERITY = {
    "LOW": 2,
    "MEDIUM": 5,
    "HIGH": 10,
    "CRITICAL": 20,
}


def calculate_score(findings: list[dict[str, str]]) -> int:
    total_risk_points = sum(
        RISK_POINTS_BY_SEVERITY.get(finding["severity"], 0)
        for finding in findings
    )
    return max(0, 100 - total_risk_points)
