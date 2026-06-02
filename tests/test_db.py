from app.db import get_db


def test_accounts_table_created(app):
    with app.app_context():
        db = get_db()
        tables = {r["name"] for r in db.execute(
            "SELECT name FROM sqlite_master WHERE type='table'"
        ).fetchall()}
        assert "accounts" in tables
        assert "account_balances" in tables


def test_account_balances_unique_constraint(app):
    with app.app_context():
        db = get_db()
        db.execute(
            "INSERT INTO accounts (monarch_id, name, type, institution) VALUES (?,?,?,?)",
            ("m1", "Checking", "checking", "Bank")
        )
        db.commit()
        db.execute(
            "INSERT INTO account_balances (account_id, date, balance) VALUES (1, '2026-01-01', 100.0)"
        )
        db.commit()
        # Second insert with same account_id + date should replace, not duplicate
        db.execute(
            "INSERT OR REPLACE INTO account_balances (account_id, date, balance) VALUES (1, '2026-01-01', 200.0)"
        )
        db.commit()
        rows = db.execute("SELECT * FROM account_balances WHERE account_id=1 AND date='2026-01-01'").fetchall()
        assert len(rows) == 1
        assert rows[0]["balance"] == 200.0
