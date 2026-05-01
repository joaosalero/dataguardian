#!/bin/bash
echo "Running tests..."
PYTEST_BIN="pytest"
if [ -x "../.venv/bin/pytest" ]; then
  PYTEST_BIN="../.venv/bin/pytest"
fi

$PYTEST_BIN -q
if [ $? -ne 0 ]; then
  echo "Tests failed"
  exit 1
fi
echo "All tests passed"
