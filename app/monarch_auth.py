import os
import re
from pathlib import Path

_ENV_PATH = Path(__file__).parent.parent / ".env"


def _update_env_file(key: str, value: str):
    text = _ENV_PATH.read_text() if _ENV_PATH.exists() else ""
    pattern = re.compile(rf"^{re.escape(key)}=.*$", re.MULTILINE)
    new_line = f"{key}={value}"
    if pattern.search(text):
        text = pattern.sub(new_line, text)
    else:
        text = text.rstrip("\n") + f"\n{new_line}\n"
    _ENV_PATH.write_text(text)


def refresh_credentials() -> dict:
    """
    Launch a headless Chromium browser, log into Monarch, extract fresh
    session cookies, and persist them to .env + os.environ.
    """
    try:
        from playwright.sync_api import sync_playwright, TimeoutError as PWTimeout
    except ImportError:
        raise RuntimeError(
            "Playwright not installed. Run: pip install playwright && playwright install chromium"
        )

    email    = os.environ.get("MONARCH_EMAIL", "")
    password = os.environ.get("MONARCH_PASSWORD", "")
    if not email or not password:
        raise RuntimeError("MONARCH_EMAIL and MONARCH_PASSWORD must be set in .env")

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            user_agent=(
                "Mozilla/5.0 (X11; Linux x86_64) "
                "AppleWebKit/537.36 (KHTML, like Gecko) "
                "Chrome/124.0.0.0 Safari/537.36"
            )
        )
        page = context.new_page()

        page.goto("https://app.monarch.com/login", wait_until="networkidle", timeout=30_000)

        page.locator('input[name="username"]').fill(email)
        page.locator('input[name="password"]').fill(password)
        # Use the Sign In button text since there's no type="submit"
        page.get_by_role("button", name="Sign In").click()

        # Wait up to 60s for Cloudflare challenge + login redirect
        try:
            page.wait_for_url("**/dashboard**", timeout=60_000)
        except PWTimeout:
            browser.close()
            raise RuntimeError(
                "Timed out waiting for Monarch dashboard after login — "
                "check credentials or MFA settings"
            )

        cookies = {c["name"]: c["value"] for c in context.cookies()}
        browser.close()

    session_id   = cookies.get("session_id", "")
    csrf_token   = cookies.get("csrftoken", "")
    cf_clearance = cookies.get("cf_clearance", "")

    if not session_id:
        raise RuntimeError("Login appeared to succeed but no session_id cookie was found")

    for key, val in [
        ("MONARCH_SESSION_ID",   session_id),
        ("MONARCH_CSRF_TOKEN",   csrf_token),
        ("MONARCH_CF_CLEARANCE", cf_clearance),
    ]:
        _update_env_file(key, val)
        os.environ[key] = val

    return {"session_id": session_id, "csrf_token": csrf_token, "cf_clearance": cf_clearance}
