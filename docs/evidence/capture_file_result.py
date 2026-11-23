"""Capture an inspection from a fresh, local DataGuardian demo stack."""

import argparse
from io import BytesIO
from pathlib import Path
import secrets

from PIL import Image
from playwright.sync_api import expect, sync_playwright


def synthetic_pdf() -> bytes:
    objects = [
        b"<< /Type /Catalog /Pages 2 0 R >>",
        b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
        b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 300] /Resources << >> >>",
        b"<< /Author (Fixture Author) /Title (Public fixture) >>",
    ]
    document = bytearray(b"%PDF-1.7\n")
    offsets = []
    for number, obj in enumerate(objects, 1):
        offsets.append(len(document))
        document.extend(f"{number} 0 obj\n".encode() + obj + b"\nendobj\n")
    xref = len(document)
    document.extend(b"xref\n0 5\n0000000000 65535 f \n")
    for offset in offsets:
        document.extend(f"{offset:010} 00000 n \n".encode())
    document.extend(f"trailer\n<< /Size 5 /Root 1 0 R /Info 4 0 R >>\nstartxref\n{xref}\n%%EOF\n".encode())
    return bytes(document)


def capture(output: Path, channel: str | None) -> None:
    with sync_playwright() as p:
        browser = p.chromium.launch(channel=channel)
        page = browser.new_page(viewport={"width": 1280, "height": 850})
        user = f"fixture-{secrets.token_hex(5)}@example.test"
        password = secrets.token_urlsafe(24)
        page.goto("http://localhost:3000/register")
        page.get_by_label("Username").fill(user)
        page.get_by_label("Password").fill(password)
        page.get_by_role("button", name="Create account").click()
        expect(page.get_by_text("Account created. You can sign in now.")).to_be_visible()
        page.goto("http://localhost:3000/login")
        page.get_by_label("Username").fill(user)
        page.get_by_label("Password").fill(password)
        page.get_by_role("button", name="Sign in").click()
        page.wait_for_url("**/dashboard")
        pdf = synthetic_pdf()
        page.get_by_label("Analysis file").set_input_files({
            "name": "public-fixture.pdf", "mimeType": "application/pdf", "buffer": pdf
        })
        with page.expect_response(lambda r: r.url == "http://localhost:8000/analyses" and r.request.method == "POST") as submitted:
            page.get_by_role("button", name="Analyze File").click()
        assert submitted.value.status == 201, submitted.value.status
        report = submitted.value.json()
        assert [finding["code"] for finding in report["findings"]] == ["METADATA_AUTHOR_PRESENT"]
        assert report["riskScore"]["score"] == 55 and report["riskScore"]["level"] == "MEDIUM"
        page.get_by_role("row").filter(has_text="FILE").get_by_role("button", name="View").click()
        details = page.locator("section").filter(has=page.get_by_role("heading", name="Analysis details"))
        expect(details.get_by_text("METADATA_AUTHOR_PRESENT", exact=True).first).to_be_visible()
        expect(details.get_by_text("MEDIUM risk", exact=False).first).to_be_visible()
        # The top of the actual result panel contains the score, finding and risk drivers.
        with Image.open(BytesIO(details.screenshot())) as panel:
            panel.crop((0, 0, panel.width, min(panel.height, 490))).save(output, "WEBP", lossless=True)
        print(f"HTTP 201; PDF bytes={len(pdf)}; finding={report['findings'][0]['code']}; "
              f"score={report['riskScore']['score']}; level={report['riskScore']['level']}")
        browser.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path)
    parser.add_argument("--channel", help="Optional installed Chromium channel, e.g. chrome")
    args = parser.parse_args()
    capture(args.output, args.channel)
