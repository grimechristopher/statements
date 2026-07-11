import json
import os
import urllib.error
import urllib.request
from datetime import date

from flask import Blueprint, render_template, jsonify, request
from app.db import get_db

bp = Blueprint("balances", __name__)

_GQL_URL = "https://api.monarch.com/graphql"


def _monarch_credentials():
    """DB config takes precedence over .env for session tokens."""
    db = get_db()
    rows = {r["key"]: r["value"] for r in db.execute(
        "SELECT key, value FROM config WHERE key IN (?,?,?)",
        ("monarch_session_id", "monarch_csrf_token", "monarch_cf_clearance"),
    ).fetchall()}
    return (
        rows.get("monarch_session_id")   or os.environ.get("MONARCH_SESSION_ID", ""),
        rows.get("monarch_csrf_token")   or os.environ.get("MONARCH_CSRF_TOKEN", ""),
        rows.get("monarch_cf_clearance") or os.environ.get("MONARCH_CF_CLEARANCE", ""),
    )


def _monarch_gql(query, variables=None):
    session_id, csrf_token, cf_clearance = _monarch_credentials()
    if not session_id or not csrf_token:
        raise RuntimeError("MONARCH_SESSION_ID and MONARCH_CSRF_TOKEN must be set in .env")
    cookies = f"cf_clearance={cf_clearance}; csrftoken={csrf_token}; session_id={session_id}"
    body = json.dumps({"query": query, "variables": variables or {}}).encode()
    req = urllib.request.Request(_GQL_URL, data=body, headers={
        "Content-Type": "application/json",
        "Cookie": cookies,
        "x-csrftoken": csrf_token,
        "client-platform": "web",
        "Origin": "https://app.monarch.com",
        "User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:150.0) Gecko/20100101 Firefox/150.0",
    })
    try:
        resp = urllib.request.urlopen(req)
        return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        if e.code in (401, 403):
            raise RuntimeError(
                "Monarch session expired — run: python3 monarch_reauth.py"
            )
        raise


def _forward_fill_totals(rows):
    """Sum account balances by date, carrying each account's last known value forward.

    Prevents dates where only one account has a snapshot (e.g. Coinbase daily)
    from dragging the total down while others are silent.
    """
    from collections import defaultdict
    acct_history = defaultdict(list)
    for r in rows:
        acct_history[r["id"]].append((r["date"], r["balance"] or 0))

    all_dates = sorted({r["date"] for r in rows})
    latest = {}
    total_by_date = {}
    for d in all_dates:
        for aid, history in acct_history.items():
            for (date, bal) in history:
                if date == d:
                    latest[aid] = bal
        total_by_date[d] = sum(latest.values())
    return total_by_date


def _balance_rows(db, account_id=None):
    if account_id:
        return db.execute("""
            SELECT ab.date, a.name, a.type, a.institution, ab.balance
            FROM account_balances ab
            JOIN accounts a ON ab.account_id = a.id
            WHERE a.id = ?
            ORDER BY ab.date DESC, a.name ASC
        """, (account_id,)).fetchall()
    return db.execute("""
        SELECT ab.date, a.name, a.type, a.institution, ab.balance
        FROM account_balances ab
        JOIN accounts a ON ab.account_id = a.id
        ORDER BY ab.date DESC, a.name ASC
    """).fetchall()


@bp.route("/balances")
def balances():
    db = get_db()
    accounts = db.execute("SELECT id, name FROM accounts ORDER BY name").fetchall()
    rows = _balance_rows(db, request.args.get("account_id"))
    return render_template("balances.html", accounts=accounts, rows=rows)


@bp.route("/balances-table")
def balances_table():
    db = get_db()
    rows = _balance_rows(db, request.args.get("account_id"))
    return render_template("partials/balances_table.html", rows=rows)


@bp.route("/api/monarch/sync", methods=["POST"])
def monarch_sync():
    import time

    try:
        result = _monarch_gql("""{ accounts {
            id displayName currentBalance
            type { name }
            institution { name }
        } }""")
    except RuntimeError as e:
        return jsonify({"error": str(e)}), 500
    except Exception as e:
        return jsonify({"error": f"Monarch request failed: {e}"}), 500

    accounts = result.get("data", {}).get("accounts", [])
    db = get_db()
    today = date.today().isoformat()
    synced_accounts = 0
    synced_snapshots = 0

    for acc in accounts:
        monarch_id = acc.get("id")
        name = acc.get("displayName") or ""
        acc_type = (acc.get("type") or {}).get("name", "")
        institution = (acc.get("institution") or {}).get("name", "")
        balance = acc.get("currentBalance")
        if monarch_id is None:
            continue

        db.execute(
            "INSERT INTO accounts (monarch_id, name, type, institution) VALUES (?,?,?,?) "
            "ON CONFLICT(monarch_id) DO UPDATE SET name=excluded.name, type=excluded.type, institution=excluded.institution",
            (monarch_id, name, acc_type, institution),
        )
        db.commit()
        account_id = db.execute(
            "SELECT id FROM accounts WHERE monarch_id=?", (monarch_id,)
        ).fetchone()["id"]

        if balance is not None:
            db.execute(
                "INSERT OR REPLACE INTO account_balances (account_id, date, balance) VALUES (?,?,?)",
                (account_id, today, balance),
            )
            db.commit()
            synced_accounts += 1

        # Pull full snapshot history to fill gaps — skip if synced within 7 days
        if _is_investment(acc_type):
            recent = db.execute("""
                SELECT MAX(date) as last FROM account_balances WHERE account_id=?
            """, (account_id,)).fetchone()["last"]
            from datetime import timedelta
            cutoff = (date.today() - timedelta(days=7)).isoformat()
            if recent and recent >= cutoff:
                continue
            try:
                hist = _monarch_gql("""
                    query($id: UUID!) {
                        snapshots: snapshotsForAccount(accountId: $id) {
                            date
                            signedBalance
                        }
                    }
                """, {"id": monarch_id})
                snaps = hist.get("data", {}).get("snapshots", []) or []
                for s in snaps:
                    d = (s.get("date") or "")[:10]
                    v = s.get("signedBalance")
                    if d and v is not None:
                        db.execute(
                            "INSERT OR REPLACE INTO account_balances (account_id, date, balance) VALUES (?,?,?)",
                            (account_id, d, v),
                        )
                        synced_snapshots += 1
                db.commit()
                time.sleep(0.3)
            except Exception:
                pass

    return jsonify({"synced": synced_accounts, "snapshots": synced_snapshots})


_NON_INVESTMENT_TYPES = {
    "checking", "savings", "credit", "credit_card", "loan",
    "mortgage", "real_estate", "vehicle", "other_liability", "cash",
}

BIRTH_YEAR = int(os.environ.get("BIRTH_YEAR", 1990))
RETIREMENT_TARGET = 3_500_000
ANNUAL_RETURN = 0.10
CHART_END_YEAR = 2069  # age 75


def _is_investment(account_type: str) -> bool:
    t = (account_type or "").lower().strip()
    return t not in _NON_INVESTMENT_TYPES and t != ""


def _build_projections(start_date_str: str, start_balance: float, monthly_contribution: float):
    from datetime import date as date_cls
    import calendar

    r = ANNUAL_RETURN / 12
    start = date_cls.fromisoformat(start_date_str)

    coast_points, contrib_points = [], []
    coast_crossed = contrib_crossed = None

    coast_bal = contrib_bal = start_balance
    y, m = start.year, start.month

    while y <= CHART_END_YEAR:
        d = f"{y:04d}-{m:02d}-01"
        coast_points.append({"date": d, "value": round(coast_bal, 2)})
        contrib_points.append({"date": d, "value": round(contrib_bal, 2)})

        if coast_crossed is None and coast_bal >= RETIREMENT_TARGET:
            coast_crossed = {"date": d, "age": y - BIRTH_YEAR}
        if contrib_crossed is None and contrib_bal >= RETIREMENT_TARGET:
            contrib_crossed = {"date": d, "age": y - BIRTH_YEAR}

        coast_bal = coast_bal * (1 + r)
        contrib_bal = contrib_bal * (1 + r) + monthly_contribution

        m += 1
        if m > 12:
            m = 1
            y += 1

    return coast_points, contrib_points, coast_crossed, contrib_crossed


@bp.route("/api/balance-chart-data")
def balance_chart_data():
    db = get_db()

    EXCLUDE = {'HSA Cash'}

    # All brokerage accounts for historical totals (includes Investment History etc.)
    all_brokerage_ids = {
        r["id"]
        for r in db.execute("""
            SELECT a.id FROM accounts a WHERE a.type = 'brokerage' AND a.name NOT IN ({})
        """.format(",".join("?" * len(EXCLUDE))), list(EXCLUDE)).fetchall()
    }

    # Only accounts with positive current balance for projection starting point
    active_ids = {
        r["account_id"]
        for r in db.execute("""
            SELECT ab.account_id
            FROM account_balances ab
            JOIN accounts a ON ab.account_id = a.id
            WHERE a.type = 'brokerage'
              AND a.name NOT IN ({})
              AND ab.date = (SELECT MAX(date) FROM account_balances WHERE account_id = ab.account_id)
              AND ab.balance > 0
        """.format(",".join("?" * len(EXCLUDE))), list(EXCLUDE)).fetchall()
    }

    if not all_brokerage_ids:
        return jsonify({"total_history": [], "projections": {}, "monthly_contribution": 0, "target": RETIREMENT_TARGET})

    placeholders = ",".join("?" * len(all_brokerage_ids))
    rows = db.execute(f"""
        SELECT ab.date, a.id, ab.balance
        FROM account_balances ab
        JOIN accounts a ON ab.account_id = a.id
        WHERE a.id IN ({placeholders})
        ORDER BY ab.date ASC
    """, list(all_brokerage_ids)).fetchall()

    total_by_date = _forward_fill_totals(rows)
    sorted_dates = sorted(total_by_date)
    total_history = [{"date": d, "value": round(total_by_date[d], 2)} for d in sorted_dates]

    # Average 401k from last 24 regular paychecks (excludes tiny supplemental entries)
    contrib_row = db.execute("""
        SELECT AVG(total_401k) as avg FROM (
            SELECT total_401k FROM pay_statements
            WHERE total_401k > 200
            ORDER BY rowid DESC LIMIT 24
        )
    """).fetchone()
    avg_biweekly_401k = float(contrib_row["avg"] or 0)
    monthly_401k = avg_biweekly_401k * 26 / 12

    # Max Roth IRA for 2 people ($7,500/person/year as of 2026)
    monthly_roth = (7_500 * 2) / 12

    monthly_contribution = monthly_401k + monthly_roth

    # Projection starts from current active balance only (not zeroed-out historical accounts)
    active_current = db.execute(f"""
        SELECT SUM(ab.balance) as total
        FROM account_balances ab
        WHERE ab.account_id IN ({",".join("?" * len(active_ids))})
          AND ab.date = (SELECT MAX(date) FROM account_balances WHERE account_id = ab.account_id)
    """, list(active_ids)).fetchone()["total"] or 0

    projections = {}
    if total_history:
        last = total_history[-1]
        coast, contrib, coast_x, contrib_x = _build_projections(
            last["date"], round(active_current, 2), monthly_contribution
        )
        _, contrib_401k, _, contrib_401k_x = _build_projections(
            last["date"], round(active_current, 2), monthly_401k
        )
        projections = {
            "coast": coast,
            "current_rate": contrib,
            "coast_crossed": coast_x,
            "contrib_crossed": contrib_x,
            "contrib_401k_only": contrib_401k,
            "contrib_401k_only_crossed": contrib_401k_x,
        }

    start_row = db.execute("""
        SELECT pay_date FROM pay_statements ORDER BY rowid ASC LIMIT 1
    """).fetchone()
    raw = start_row["pay_date"] if start_row else "4/1/2022"
    parts = raw.split("/")
    chart_start = f"{int(parts[2]):04d}-{int(parts[0]):02d}-01"

    return jsonify({
        "total_history": total_history,
        "projections": projections,
        "monthly_contribution": round(monthly_contribution, 2),
        "monthly_401k": round(monthly_401k, 2),
        "monthly_roth": round(monthly_roth, 2),
        "avg_biweekly_401k": round(avg_biweekly_401k, 2),
        "active_balance": round(active_current, 2),
        "annual_return": ANNUAL_RETURN,
        "birth_year": BIRTH_YEAR,
        "target": RETIREMENT_TARGET,
        "chart_start": chart_start,
    })
