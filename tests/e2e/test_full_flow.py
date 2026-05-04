import os
from pathlib import Path
import sys
import time
from urllib.error import URLError
from urllib.request import urlopen

from playwright.sync_api import Page, expect, sync_playwright


ROOT_DIR = Path(__file__).resolve().parents[2]
BACKEND_DIR = ROOT_DIR / "backend"
sys.path.insert(0, str(BACKEND_DIR))

from app.auth import hash_password
from app.core.database import Base, SessionLocal, engine
from app.models.user import User


FRONTEND_URL = "http://localhost:3000"
BACKEND_URL = "http://localhost:8000"
TEST_EMAIL = "e2e@dataguardian.dev"
TEST_PASSWORD = "strong-password"


def log(message: str) -> None:
    print(f"[E2E] {message}", flush=True)


def require_service(url: str) -> None:
    try:
        with urlopen(url, timeout=5) as response:
            if response.status >= 500:
                raise RuntimeError(f"{url} returned HTTP {response.status}")
    except (OSError, URLError) as exc:
        raise RuntimeError(f"Required service is not reachable: {url}") from exc


def ensure_test_user() -> None:
    Base.metadata.create_all(bind=engine)
    db = SessionLocal()
    try:
        user = db.query(User).filter(User.email == TEST_EMAIL).one_or_none()
        hashed_password = hash_password(TEST_PASSWORD)
        if user is None:
            db.add(User(email=TEST_EMAIL, hashed_password=hashed_password))
        else:
            user.hashed_password = hashed_password
        db.commit()
    finally:
        db.close()


def test_full_user_flow() -> None:
    project_name = f"E2E Project {int(time.time())}"

    try:
        log("Checking backend health")
        require_service(f"{BACKEND_URL}/health")
        log("Checking frontend login page")
        require_service(f"{FRONTEND_URL}/login")
        log("Seeding test user")
        ensure_test_user()

        with sync_playwright() as playwright:
            browser = playwright.chromium.launch(
                headless=os.getenv("PLAYWRIGHT_HEADLESS", "1") != "0",
                slow_mo=350 if os.getenv("PLAYWRIGHT_HEADLESS") == "0" else 0,
            )
            page: Page = browser.new_page()
            try:
                log("Opening login page")
                page.goto(f"{FRONTEND_URL}/login", wait_until="networkidle")

                log("Filling login form")
                page.get_by_label("Username").fill(TEST_EMAIL)
                page.get_by_label("Password").fill(TEST_PASSWORD)

                log("Submitting login")
                page.get_by_role("button", name="Sign in").click()
                page.wait_for_url("**/dashboard", timeout=10_000)
                expect(page.get_by_role("heading", name="Projects")).to_be_visible()

                log("Creating project")
                page.get_by_placeholder("Project name").fill(project_name)
                page.get_by_placeholder("Description").fill("Created by Playwright E2E")
                page.get_by_role("button", name="Create").click()
                expect(page.get_by_text("Project created.")).to_be_visible(timeout=10_000)
                expect(page.get_by_text(project_name)).to_be_visible()

                log("Opening project page")
                page.get_by_text(project_name).click()
                page.wait_for_url("**/projects/**", timeout=10_000)
                expect(page.get_by_role("heading", name=project_name)).to_be_visible()

                log("Running audit")
                page.get_by_role("button", name="Run audit").click()
                expect(page.get_by_text("Audit completed.")).to_be_visible(timeout=15_000)

                log("Validating audit result")
                expect(page.get_by_text("Latest audit")).to_be_visible()
                expect(page.get_by_text("Score").first).to_be_visible()
                expect(
                    page.get_by_role("heading", name="Sensitive field detected")
                ).to_be_visible()
                expect(page.get_by_text("Audit history")).to_be_visible()

                log("E2E TEST PASSED")
            finally:
                browser.close()
    except Exception:
        log("E2E TEST FAILED")
        raise
