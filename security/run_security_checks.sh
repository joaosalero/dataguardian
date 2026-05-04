#!/usr/bin/env bash
set -u

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR" || exit 1

FAILED=0
MODE="${1:-all}"

print_result() {
  local name="$1"
  local status="$2"
  printf '[%s] %s\n' "$status" "$name"
}

mark_failure() {
  FAILED=1
}

run_pip_audit() {
  local audit_cmd=()

  if command -v pip-audit >/dev/null 2>&1; then
    audit_cmd=(pip-audit)
  elif python -m pip_audit --version >/dev/null 2>&1; then
    audit_cmd=(python -m pip_audit)
  elif [ -x ".venv/bin/python" ] && .venv/bin/python -m pip_audit --version >/dev/null 2>&1; then
    audit_cmd=(.venv/bin/python -m pip_audit)
  else
    print_result "Python dependency scan: pip-audit is not installed" "FAIL"
    mark_failure
    return
  fi

  if "${audit_cmd[@]}" -r backend/requirements.txt --progress-spinner off --cache-dir /tmp/dataguardian-pip-audit-cache >/tmp/dataguardian-pip-audit.out 2>/tmp/dataguardian-pip-audit.err; then
    print_result "Python dependency scan" "PASS"
  else
    print_result "Python dependency scan" "FAIL"
    mark_failure
  fi
}

run_npm_audit() {
  if [ ! -f "frontend/package-lock.json" ]; then
    print_result "Node dependency scan: package-lock.json missing" "FAIL"
    mark_failure
    return
  fi

  if npm audit --audit-level=high --prefix frontend >/tmp/dataguardian-npm-audit.out 2>/tmp/dataguardian-npm-audit.err; then
    print_result "Node dependency scan" "PASS"
  else
    print_result "Node dependency scan" "FAIL"
    mark_failure
  fi
}

line_is_allowlisted() {
  local file="$1"
  local line="$2"

  case "$line" in
    *"development-only-secret-key-change-before-production"*|\
    *"postgresql://dataguardian:dataguardian@localhost:5434/dataguardian"*)
      return 0
      ;;
  esac

  return 1
}

run_secret_scan() {
  local findings=0
  local file
  local line_number
  local line
  local key

  while IFS= read -r -d '' file; do
    case "$file" in
      security/*.sh|frontend/package-lock.json|frontend/package.json)
        continue
        ;;
    esac

    line_number=0
    while IFS= read -r line || [ -n "$line" ]; do
      line_number=$((line_number + 1))
      if [[ "$line" =~ (^|[^A-Za-z0-9_])(password|secret|token|api_key)= ]]; then
        if ! line_is_allowlisted "$file" "$line"; then
          key="${BASH_REMATCH[2]}"
          printf '[FAIL] Potential secret assignment: %s:%s key=%s value=<redacted>\n' "$file" "$line_number" "$key"
          findings=1
        fi
      fi
      if [[ "$line" == *"BEGIN PRIVATE KEY"* || "$line" == *"BEGIN RSA PRIVATE KEY"* ]]; then
        printf '[FAIL] Private key material detected: %s:%s value=<redacted>\n' "$file" "$line_number"
        findings=1
      fi
    done < "$file"
  done < <(git ls-files --cached --others --exclude-standard -z)

  if [ "$findings" -eq 0 ]; then
    print_result "Secret scan" "PASS"
  else
    print_result "Secret scan" "FAIL"
    mark_failure
  fi
}

run_config_validation() {
  local config_failed=0

  if git ls-files --error-unmatch .env >/dev/null 2>&1; then
    print_result "Config validation: .env is tracked" "FAIL"
    config_failed=1
  fi

  if ! git check-ignore -q .env; then
    print_result "Config validation: .env is not ignored" "FAIL"
    config_failed=1
  fi

  if [ "$config_failed" -eq 0 ]; then
    print_result "Config validation" "PASS"
  else
    mark_failure
  fi
}

printf '[SECURITY CHECKS]\n'
case "$MODE" in
  all)
    run_pip_audit
    run_npm_audit
    run_secret_scan
    run_config_validation
    ;;
  --secrets-only)
    run_secret_scan
    run_config_validation
    ;;
  *)
    printf 'Usage: %s [--secrets-only]\n' "$0"
    exit 2
    ;;
esac

if [ "$FAILED" -eq 0 ]; then
  print_result "Security checks completed" "PASS"
  exit 0
fi

print_result "Security checks completed" "FAIL"
exit 1
