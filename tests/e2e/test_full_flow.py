import os
import uuid
from urllib.error import URLError
from urllib.request import urlopen

from playwright.sync_api import Page, expect, sync_playwright


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


def test_full_user_flow() -> None:
    headless = os.getenv("E2E_HEADLESS", "true").lower() not in {"0", "false", "no"}

    try:
        log("Checking backend health")
        require_service(f"{BACKEND_URL}/health")
        log("Checking frontend login page")
        require_service(f"{FRONTEND_URL}/login")
        run_go_auth_flow(headless)
    except Exception:
        log("E2E TEST FAILED")
        raise


def run_go_auth_flow(headless: bool) -> None:
    test_email = f"e2e-{uuid.uuid4().hex[:8]}@dataguardian.dev"
    test_password = "StrongPass123"

    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(
            headless=headless,
            slow_mo=300 if not headless else 0,
        )
        page: Page = browser.new_page()
        try:
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
            expect(page.get_by_role("heading", name="Dashboard")).to_be_visible()
            expect(page.get_by_text(f"Signed in as {test_email}.")).to_be_visible(
                timeout=10_000
            )
            log("GO AUTH E2E TEST PASSED")
        finally:
            browser.close()
