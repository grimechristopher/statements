import pytest

from app.db import get_db


def _seed_balances(db):
    db.execute(
        "INSERT INTO accounts (monarch_id, name, type, institution) VALUES (?,?,?,?)",
        ("m1", "Fidelity 401k", "brokerage", "Fidelity")
    )
    db.execute(
        "INSERT INTO accounts (monarch_id, name, type, institution) VALUES (?,?,?,?)",
        ("m2", "Vanguard Brokerage", "brokerage", "Vanguard")
    )
    db.execute(
        "INSERT INTO accounts (monarch_id, name, type, institution) VALUES (?,?,?,?)",
        ("m3", "Checking", "checking", "Chase")
    )
    db.commit()
    db.execute(
        "INSERT INTO account_balances (account_id, date, balance) VALUES (1, '2026-01-01', 1000.0)"
    )
    db.execute(
        "INSERT INTO account_balances (account_id, date, balance) VALUES (2, '2026-01-01', 500.0)"
    )
    db.execute(
        "INSERT INTO account_balances (account_id, date, balance) VALUES (3, '2026-01-01', 300.0)"
    )
    db.execute(
        "INSERT INTO account_balances (account_id, date, balance) VALUES (1, '2026-02-01', 1500.0)"
    )
    db.execute(
        "INSERT INTO account_balances (account_id, date, balance) VALUES (2, '2026-02-01', 600.0)"
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
    assert "total_history" in data
    assert "projections" in data


def test_balance_chart_data_totals_include_only_brokerage_accounts(client, app):
    with app.app_context():
        _seed_balances(get_db())

    response = client.get("/api/balance-chart-data")
    data = response.get_json()
    # brokerage-only totals: checking account (300.0) must be excluded
    totals = {p["date"]: p["value"] for p in data["total_history"]}
    assert totals["2026-01-01"] == pytest.approx(1500.0)
    assert totals["2026-02-01"] == pytest.approx(2100.0)


def test_sync_missing_credentials_returns_error(client, monkeypatch):
    monkeypatch.delenv("MONARCH_SESSION_ID", raising=False)
    monkeypatch.delenv("MONARCH_CSRF_TOKEN", raising=False)
    monkeypatch.delenv("MONARCH_CF_CLEARANCE", raising=False)
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

    import app.routes.balances as balances_mod
    monkeypatch.setattr(
        balances_mod, "_monarch_gql",
        lambda query, variables=None: {"data": {"accounts": fake_accounts}},
    )

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
