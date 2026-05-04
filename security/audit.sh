#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR" || exit 1

DEPENDENCY_STATUS="PASS"
SECRETS_STATUS="PASS"
BACKEND_STATUS="PASS"
FRONTEND_STATUS="PASS"

contains_sensitive_output() {
  local value="$1"

  if printf '%s' "$value" | grep -Eiq 'Traceback|stack trace|postgresql://|DATABASE_URL|SECRET_KEY|access_token|token=|password='; then
    return 0
  fi

  return 1
}

check_dependencies() {
  ./security/run_security_checks.sh >/tmp/dataguardian-security-checks.out 2>/tmp/dataguardian-security-checks.err

  if ! grep -q '\[PASS\] Node dependency scan' /tmp/dataguardian-security-checks.out; then
    DEPENDENCY_STATUS="FAIL"
  fi

  if ! grep -q '\[PASS\] Secret scan' /tmp/dataguardian-security-checks.out \
    || ! grep -q '\[PASS\] Config validation' /tmp/dataguardian-security-checks.out; then
    SECRETS_STATUS="FAIL"
  fi
}

check_repo_exposure() {
  if git ls-files --error-unmatch .env >/dev/null 2>&1; then
    SECRETS_STATUS="FAIL"
  fi

  if ! git check-ignore -q .env; then
    SECRETS_STATUS="FAIL"
  fi
}

check_endpoint() {
  local name="$1"
  local url="$2"
  local response_file="$3"
  local header_file="$4"
  local body
  local status_code

  status_code="$(curl -fsS -D "$header_file" -o "$response_file" -w '%{http_code}' "$url" 2>/dev/null || true)"
  if [ "$status_code" != "200" ]; then
    printf '%s' "FAIL"
    return
  fi

  body="$(cat "$response_file" "$header_file")"
  if contains_sensitive_output "$body"; then
    printf '%s' "FAIL"
    return
  fi

  if grep -Eiq 'x-powered-by:|server: .*[0-9]+\.[0-9]+' "$header_file"; then
    printf '%s' "FAIL"
    return
  fi

  printf '%s' "PASS"
}

check_runtime() {
  BACKEND_STATUS="$(check_endpoint "backend" "http://localhost:8000/health" "/tmp/dataguardian-backend-body.out" "/tmp/dataguardian-backend-headers.out")"
  FRONTEND_STATUS="$(check_endpoint "frontend" "http://localhost:3000/login" "/tmp/dataguardian-frontend-body.out" "/tmp/dataguardian-frontend-headers.out")"
}

risk_level() {
  if [ "$DEPENDENCY_STATUS" = "FAIL" ] || [ "$SECRETS_STATUS" = "FAIL" ]; then
    printf '%s' "HIGH"
    return
  fi

  if [ "$BACKEND_STATUS" = "FAIL" ] || [ "$FRONTEND_STATUS" = "FAIL" ]; then
    printf '%s' "MEDIUM"
    return
  fi

  printf '%s' "LOW"
}

check_dependencies
check_repo_exposure
check_runtime
RISK_LEVEL="$(risk_level)"

printf '[SECURITY AUDIT REPORT]\n'
printf -- '- Dependency scan: %s\n' "$DEPENDENCY_STATUS"
printf -- '- Secrets exposure: %s\n' "$SECRETS_STATUS"
printf -- '- Backend exposure: %s\n' "$BACKEND_STATUS"
printf -- '- Frontend exposure: %s\n' "$FRONTEND_STATUS"
printf -- '- Risk level: %s\n' "$RISK_LEVEL"

if [ "$RISK_LEVEL" = "LOW" ]; then
  exit 0
fi

exit 1
