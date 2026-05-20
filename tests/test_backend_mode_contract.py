from pathlib import Path


ROOT_DIR = Path(__file__).resolve().parents[1]


def test_runtime_uses_go_backend_only() -> None:
    start_script = (ROOT_DIR / "start.sh").read_text()
    compose = (ROOT_DIR / "docker-compose.yml").read_text()
    workflow = (ROOT_DIR / ".github" / "workflows" / "ci.yml").read_text()
    dependabot = (ROOT_DIR / ".github" / "dependabot.yml").read_text()
    go_mod = (ROOT_DIR / "backend-go" / "go.mod").read_text()

    assert "backend-go" in compose
    assert "container_name: dataguardian_backend_go" in compose
    assert "context: ./backend-go" in compose
    assert "image: python" not in compose
    assert "uvicorn" not in compose
    assert "go 1.25.0" in go_mod

    assert "compose up -d --build db backend-go" in start_script
    assert "compose up -d --build --force-recreate frontend" in start_script
    assert "go run ./cmd/server" not in compose
    assert "uvicorn" not in start_script
    assert "--backend=python" not in start_script
    assert "BACKEND_MODE" not in start_script
    assert "[WARN] pytest not found. Skipping E2E tests." in start_script

    assert "go test ./..." in workflow
    assert "pytest backend" not in workflow
    assert "backend/requirements.txt" not in workflow
    assert 'package-ecosystem: "gomod"' in dependabot
    assert 'directory: "/backend-go"' in dependabot
    assert 'package-ecosystem: "pip"' not in dependabot
