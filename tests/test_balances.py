import json
from app.db import get_db


def _seed_balances(db):
    db.execute(
        "INSERT INTO accounts (monarch_id, name, type, institution) VALUES (?,?,?,?)",
        ("m1", "Checking", "checking", "Chase")
    )
    db.execute(
        "INSERT INTO accounts (monarch_id, name, type, institution) VALUES (?,?,?,?)",
        ("m2", "Credit Card", "credit", "Chase")
    )
    db.commit()
    db.execute(
        "INSERT INTO account_balances (account_id, date, balance) VALUES (1, '2026-01-01', 1000.0)"
    )
    db.execute(
        "INSERT INTO account_balances (account_id, date, balance) VALUES (2, '2026-01-01', -200.0)"
    )
    db.execute(
        "INSERT INTO account_balances (account_id, date, balance) VALUES (1, '2026-02-01', 1500.0)"
    )
    db.execute(
        "INSERT INTO account_balances (account_id, date, balance) VALUES (2, '2026-02-01', -300.0)"
    )
    db.commit()


def test_balances_page_returns_200(client):
    response = client.get("/balances")
    assert response.status_code == 200


def test_balance_chart_data_returns_expected_structure(client, app):
    with app.app_context():
        _seed_balances(get_db())

    response = client.get("/api/balance-chart-data")
    assert response.status_code == 200
    data = response.get_json()
    assert "accounts" in data
    assert "net_worth" in data


def test_balance_chart_data_net_worth_is_assets_minus_liabilities(client, app):
    with app.app_context():
        _seed_balances(get_db())

    response = client.get("/api/balance-chart-data")
    data = response.get_json()
    # net worth on 2026-01-01: 1000 + (-200) = 800
    nw = {p["date"]: p["value"] for p in data["net_worth"]}
    assert nw["2026-01-01"] == pytest.approx(800.0)
    assert nw["2026-02-01"] == pytest.approx(1200.0)


def test_sync_missing_credentials_returns_error(client, monkeypatch):
    monkeypatch.delenv("MONARCH_EMAIL", raising=False)
    monkeypatch.delenv("MONARCH_PASSWORD", raising=False)
    response = client.post("/api/monarch/sync")
    assert response.status_code == 500
    data = response.get_json()
    assert "error" in data


def test_sync_stores_accounts_and_balances(client, app, monkeypatch):
    fake_accounts = [
        {
            "id": "acc1",
            "displayName": "Savings",
            "type": {"name": "savings"},
            "institution": {"name": "Ally"},
            "currentBalance": 5000.0,
        }
    ]

    class FakeMonarch:
        async def login(self, email, password, mfa_secret_key=None):
            pass

        async def get_accounts(self):
            return {"accounts": fake_accounts}

    monkeypatch.setenv("MONARCH_EMAIL", "test@example.com")
    monkeypatch.setenv("MONARCH_PASSWORD", "secret")

    import app.routes.balances as balances_mod
    monkeypatch.setattr(balances_mod, "_make_monarch_client", lambda: FakeMonarch())

    response = client.post("/api/monarch/sync")
    assert response.status_code == 200
    data = response.get_json()
    assert data["synced"] >= 1

    with app.app_context():
        db = get_db()
        acc = db.execute("SELECT * FROM accounts WHERE monarch_id='acc1'").fetchone()
        assert acc is not None
        assert acc["name"] == "Savings"
        bal = db.execute(
            "SELECT * FROM account_balances WHERE account_id=?", (acc["id"],)
        ).fetchone()
        assert bal is not None
        assert bal["balance"] == 5000.0


import pytest
