import uuid
from urllib.error import URLError
from urllib.request import urlopen

from playwright.sync_api import APIResponse
from playwright.sync_api import Page, expect


FRONTEND_URL = "http://localhost:3000"
BACKEND_URL = "http://localhost:8000"


def log(message: str) -> None:
    print(f"[E2E] {message}", flush=True)


def require_service(url: str) -> None:
    try:
        with urlopen(url, timeout=5) as response:
            if response.status >= 500:
                raise RuntimeError(f"{url} returned HTTP {response.status}")
    except (OSError, URLError) as exc:
        raise RuntimeError(f"Required service is not reachable: {url}") from exc


def test_full_user_flow(page: Page) -> None:
    try:
        log("Checking backend health")
        require_service(f"{BACKEND_URL}/health")
        log("Checking frontend login page")
        require_service(f"{FRONTEND_URL}/login")
        run_go_auth_flow(page)
    except Exception:
        log("E2E TEST FAILED")
        raise


def run_go_auth_flow(page: Page) -> None:
    test_email = f"e2e-{uuid.uuid4().hex[:8]}@dataguardian.dev"
    test_password = "StrongPass123"
    project_name = f"E2E Project {uuid.uuid4().hex[:6]}"

    log("Opening registration page")
    page.goto(f"{FRONTEND_URL}/register", wait_until="networkidle")
    page.get_by_label("Username").fill(test_email)
    page.get_by_label("Password").fill(test_password)
    page.get_by_role("button", name="Create account").click()
    expect(page.get_by_text("Account created. You can sign in now.")).to_be_visible(
        timeout=10_000
    )

    log("Opening login page")
    page.goto(f"{FRONTEND_URL}/login", wait_until="networkidle")
    page.get_by_label("Username").fill(test_email)
    page.get_by_label("Password").fill(test_password)
    page.get_by_role("button", name="Sign in").click()
    page.wait_for_url("**/dashboard", timeout=10_000)
    expect(page.get_by_role("heading", name="DataGuardian")).to_be_visible()
    expect(page.get_by_role("heading", name="Analysis history")).to_be_visible()
    expect(page.get_by_text(f"Signed in as {test_email}.")).to_be_visible(
        timeout=10_000
    )

    log("Creating project through dashboard UI")
    page.get_by_label("Project name").fill(project_name)
    page.get_by_label("Database target").fill("postgres://e2e-target")
    with page.expect_response(
        lambda response: response.url == f"{BACKEND_URL}/projects"
        and response.request.method == "POST"
    ) as project_response_info:
        page.get_by_role("button", name="Create project").click()
    project_response = project_response_info.value
    assert_response_ok(project_response, "project creation")
    page.goto(f"{FRONTEND_URL}/dashboard", wait_until="networkidle")
    expect(page.get_by_text(project_name).first).to_be_visible(timeout=10_000)

    log("Creating file analysis with authenticated browser session")
    pdf_bytes = (
        b"%PDF-1.7\n"
        b"<< /Author (E2E User) /OpenAction << /JS (JavaScript) >> >>\n"
        b"%%EOF"
    )
    page.get_by_label("Analysis file").set_input_files(
        {
            "name": "e2e-sample.pdf",
            "mimeType": "application/pdf",
            "buffer": pdf_bytes,
        }
    )
    with page.expect_response(
        lambda response: response.url == f"{BACKEND_URL}/analyses"
        and response.request.method == "POST"
    ) as file_response_info:
        page.get_by_role("button", name="Analyze File").click()
    file_response = file_response_info.value
    assert_ok(file_response, "file analysis")
    file_analysis = file_response.json()
    assert file_analysis["findings"], "file analysis should include findings"
    assert file_analysis["riskScore"]["level"], "file analysis should include risk score"

    log("Creating URL analysis with authenticated browser session")
    page.get_by_label("URL to analyze").fill("http://93.184.216.34")
    with page.expect_response(
        lambda response: response.url == f"{BACKEND_URL}/analyses"
        and response.request.method == "POST"
    ) as url_response_info:
        page.get_by_role("button", name="Analyze URL").click()
    url_response = url_response_info.value
    assert_ok(url_response, "URL analysis")
    url_analysis = url_response.json()
    assert url_analysis["findings"], "URL analysis should include findings"
    assert url_analysis["metadata"]["entries"], "URL analysis should include metadata"

    log("Reloading dashboard and validating analysis history")
    page.goto(f"{FRONTEND_URL}/dashboard", wait_until="networkidle")
    expect(page.get_by_role("heading", name="Analysis history")).to_be_visible(
        timeout=10_000
    )
    expect(page.get_by_role("row").filter(has_text="FILE")).to_be_visible(
        timeout=10_000
    )
    expect(page.get_by_role("row").filter(has_text="URL")).to_be_visible(
        timeout=10_000
    )
    assert page.get_by_role("button", name="View").count() >= 2

    log("Opening analysis details from dashboard UI")
    page.get_by_role("row").filter(has_text="FILE").get_by_role(
        "button", name="View"
    ).click()
    expect(page.get_by_role("heading", name="Analysis details")).to_be_visible(
        timeout=10_000
    )
    expect(page.get_by_role("heading", name="Findings")).to_be_visible()
    expect(page.get_by_role("heading", name="Metadata")).to_be_visible()
    expect(page.get_by_text("PDF JavaScript marker detected")).to_be_visible()
    expect(page.get_by_text("This PDF contains embedded JavaScript")).to_be_visible()
    expect(page.get_by_text("Mitigation:").first).to_be_visible()
    expect(page.get_by_text("author", exact=True)).to_be_visible()
    log("FULL E2E USER FLOW PASSED")


def assert_ok(response: APIResponse, label: str) -> None:
    if not response.ok:
        raise AssertionError(
            f"{label} request failed with HTTP {response.status}: {response.text()}"
        )


def assert_response_ok(response, label: str) -> None:
    if not response.ok:
        raise AssertionError(
            f"{label} request failed with HTTP {response.status}: {response.text()}"
        )
