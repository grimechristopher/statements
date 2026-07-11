import math
import os
from datetime import date, timedelta

from flask import Blueprint, jsonify, render_template, request

from app.db import get_db
from app.routes.balances import ANNUAL_RETURN, BIRTH_YEAR, RETIREMENT_TARGET, _build_projections, _forward_fill_totals

bp = Blueprint("milestones", __name__)

EXCLUDE = {"HSA Cash"}


def _get_config(db):
    rows = db.execute("SELECT key, value FROM config").fetchall()
    cfg = {r["key"]: r["value"] for r in rows}
    return {
        "salary":           float(cfg.get("salary", 0) or 0),
        "agi_unemployment": 19_000.0,
        "annual_rent":      float(cfg.get("annual_rent", 0) or 0),
        "first_job_salary": float(cfg.get("first_job_salary", 0) or 0),
        "home_equity":      float(cfg.get("home_equity", 0) or 0),
    }


def _current_investment(db):
    active_ids = [
        r["account_id"]
        for r in db.execute("""
            SELECT ab.account_id FROM account_balances ab
            JOIN accounts a ON ab.account_id = a.id
            WHERE a.type = 'brokerage'
              AND a.name NOT IN ({})
              AND ab.date = (SELECT MAX(date) FROM account_balances WHERE account_id = ab.account_id)
              AND ab.balance > 0
        """.format(",".join("?" * len(EXCLUDE))), list(EXCLUDE)).fetchall()
    ]
    if not active_ids:
        return 0.0
    row = db.execute("""
        SELECT SUM(ab.balance) as total FROM account_balances ab
        WHERE ab.account_id IN ({})
          AND ab.date = (SELECT MAX(date) FROM account_balances WHERE account_id = ab.account_id)
    """.format(",".join("?" * len(active_ids))), active_ids).fetchone()
    return float(row["total"] or 0)


# life event marking a large one-time balance drop (e.g. a down payment) —
# the FIRE trend line is measured from this date forward. Falls back to the
# full history if unset.
HOUSE_PURCHASE_DATE = os.environ.get("HOUSE_PURCHASE_DATE", "")


def _linear_trend(points, start_date, year_field="fire_year"):
    pts = [
        (p["date"], p[year_field] - int(p["date"][:4]))
        for p in points if p["date"] >= start_date
    ]
    if len(pts) < 2:
        return None

    x0 = date.fromisoformat(pts[0][0])
    xs = [(date.fromisoformat(d) - x0).days / 365.25 for d, _ in pts]
    ys = [y for _, y in pts]

    n = len(xs)
    mean_x = sum(xs) / n
    mean_y = sum(ys) / n
    var_x = sum((x - mean_x) ** 2 for x in xs)
    if var_x == 0:
        return None
    slope = sum((x - mean_x) * (y - mean_y) for x, y in zip(xs, ys)) / var_x

    start_value = ys[0]  # anchor to the actual observed value, not the fitted intercept
    reference_end_value = start_value - xs[-1]

    return {
        "start_date": pts[0][0],
        "end_date": pts[-1][0],
        "slope_per_year": round(slope, 3),
        "start_value": round(start_value, 3),
        "reference_end_value": round(reference_end_value, 3),
        "ahead_of_schedule": slope < -1.02,  # small tolerance for date-math float noise
    }


def _find_crossing(points, target):
    for p in points:
        if p["value"] >= target:
            year = int(p["date"][:4])
            return {"date": p["date"][:7], "age": year - BIRTH_YEAR}
    return None


def _find_historical_crossing(history, target):
    """history: ascending list of (date_str, value) tuples of actual past balances.

    Returns the most recent below->at/above transition, not the first ever
    crossing, so a big withdrawal (e.g. a house down payment) that drops you
    back under a milestone correctly requires re-crossing it."""
    crossing = None
    was_above = False
    for d, v in history:
        above = v >= target
        if above and not was_above:
            crossing = {"date": d, "age": int(d[:4]) - BIRTH_YEAR}
        was_above = above
    return crossing


def _months_since(start_date, end_date):
    start = date.fromisoformat(start_date)
    end = date.fromisoformat(end_date)
    if end < start:
        return None
    return round((end - start).days / 30.4375)


def _historical_totals(db):
    active_ids = [
        r["id"] for r in db.execute("""
            SELECT a.id FROM accounts a
            WHERE a.type = 'brokerage' AND a.name NOT IN ({})
        """.format(",".join("?" * len(EXCLUDE))), list(EXCLUDE)).fetchall()
    ]
    if not active_ids:
        return []
    rows = db.execute("""
        SELECT ab.date, a.id, ab.balance
        FROM account_balances ab
        JOIN accounts a ON ab.account_id = a.id
        WHERE a.id IN ({})
        ORDER BY ab.date ASC
    """.format(",".join("?" * len(active_ids))), active_ids).fetchall()
    total_by_date = _forward_fill_totals(rows)
    return sorted(total_by_date.items())


def _monthly_contribution(db):
    row = db.execute("""
        SELECT AVG(total_401k) as avg FROM (
            SELECT total_401k FROM pay_statements
            WHERE total_401k > 200 ORDER BY rowid DESC LIMIT 24
        )
    """).fetchone()
    monthly_401k = float(row["avg"] or 0) * 26 / 12
    monthly_roth = (7_500 * 2) / 12
    return {"total": monthly_401k + monthly_roth, "monthly_401k": monthly_401k, "monthly_roth": monthly_roth}


@bp.route("/milestones")
def milestones():
    return render_template("milestones.html")


@bp.route("/api/milestones-data")
def milestones_data():
    db = get_db()
    cfg = _get_config(db)
    salary = cfg["salary"]

    current_investment = _current_investment(db)
    current_4pct = current_investment * 0.04
    mc = _monthly_contribution(db)
    monthly_contribution = mc["total"]

    today = date.today().isoformat()
    _, contrib_pts, _, _ = _build_projections(today, current_investment, monthly_contribution)
    coast_pts, _, _, _   = _build_projections(today, current_investment, 0)
    history = _historical_totals(db)

    def nw_items():
        fixed = [
            ("300k",   300_000),
            ("Pre-house level", 380_000),
            ("500k",   500_000),
            ("1M",   1_000_000),
            ("2M",   2_000_000),
            ("3M",   3_000_000),
            ("3.5M", 3_500_000),
        ]
        # 1x/2x/3x/6x/8x match the published Fidelity by-age savings benchmarks
        multiples = [(f"{x}x salary", salary * x) for x in [0.5, 1, 2, 3, 6, 8, 10, 25]] if salary else []
        items = []
        for label, target in fixed + multiples:
            if not target:
                continue
            achieved = current_investment >= target
            crossed = _find_historical_crossing(history, target) if achieved else None
            if crossed and label == "Pre-house level":
                crossed["since_house_months"] = _months_since(HOUSE_PURCHASE_DATE, crossed["date"])
            items.append({
                "label": label,
                "target": target,
                "achieved": achieved,
                "crossed": crossed,
                "projected": _find_crossing(contrib_pts, target),
            })
        return items

    def rule4_items():
        targets = [
            ("AGI on unemployment",  cfg["agi_unemployment"]),
            ("Annual rent/mortgage", cfg["annual_rent"]),
            ("Half of salary",       salary * 0.5 if salary else 0),
            ("Salary of first job",  cfg["first_job_salary"]),
            ("Six digits",           100_000),
            ("80% of salary",        salary * 0.8 if salary else 0),
            ("Current salary",       salary),
        ]
        items = []
        for label, target in targets:
            if not target:
                continue
            nw_needed = target * 25
            achieved = current_4pct >= target
            items.append({
                "label": label,
                "target": target,
                "achieved": achieved,
                "crossed": _find_historical_crossing(history, nw_needed) if achieved else None,
                "projected": _find_crossing(contrib_pts, nw_needed),
            })
        return items

    coast_at_60_date = f"{BIRTH_YEAR + 60}-01-01"
    coast_at_60_value = next(
        (p["value"] for p in coast_pts if p["date"] >= coast_at_60_date), None
    )

    return jsonify({
        "config": cfg,
        "birth_year": BIRTH_YEAR,
        "current_investment": round(current_investment, 2),
        "current_4pct": round(current_4pct, 2),
        "monthly_contribution": round(monthly_contribution, 2),
        "monthly_401k": round(mc["monthly_401k"], 2),
        "monthly_roth": round(mc["monthly_roth"], 2),
        "groups": [
            {"label": "4% Rule of Retirement Net Worth", "current": round(current_4pct, 2), "items": rule4_items()},
            {"label": "Retirement Net Worth",   "current": round(current_investment, 2),  "items": nw_items()},
        ],
        "coast_fire": {
            "label": "Coast FIRE @ 60",
            "target": RETIREMENT_TARGET,
            "current": round(coast_at_60_value, 2) if coast_at_60_value else None,
            "achieved": (coast_at_60_value or 0) >= RETIREMENT_TARGET,
        },
    })


@bp.route("/api/fire-progress-data")
def fire_progress_data():
    db = get_db()
    mc = _monthly_contribution(db)
    monthly_contribution = mc["total"]
    monthly_401k_only = mc["monthly_401k"]

    all_brokerage_ids = [
        r["id"] for r in db.execute("""
            SELECT a.id FROM accounts a
            WHERE a.type = 'brokerage' AND a.name NOT IN ({})
        """.format(",".join("?" * len(EXCLUDE))), list(EXCLUDE)).fetchall()
    ]
    if not all_brokerage_ids:
        return jsonify({"points": [], "target_year": BIRTH_YEAR + 60, "target_swr": RETIREMENT_TARGET * 0.04})

    raw_rows = db.execute("""
        SELECT ab.date, a.id, ab.balance
        FROM account_balances ab
        JOIN accounts a ON ab.account_id = a.id
        WHERE ab.account_id IN ({})
        ORDER BY ab.date ASC
    """.format(",".join("?" * len(all_brokerage_ids))), all_brokerage_ids).fetchall()

    total_by_date = _forward_fill_totals(raw_rows)

    r = ANNUAL_RETURN / 12
    C = monthly_contribution
    C4 = monthly_401k_only
    T = RETIREMENT_TARGET

    def _fire_year_for(d, B, c):
        denom = B + c / r
        if denom <= 0:
            return int(d[:4]) + 100
        ratio = (T + c / r) / denom
        if ratio <= 1:
            return int(d[:4])
        n_months = math.log(ratio) / math.log(1 + r)
        return (date.fromisoformat(d) + timedelta(days=n_months * 30.4375)).year

    points = []
    for d, B in sorted(total_by_date.items()):
        B = float(B or 0)
        if B <= 0:
            continue

        fire_year = _fire_year_for(d, B, C)
        fire_year_401k = _fire_year_for(d, B, C4)

        start_year, start_month = int(d[:4]), int(d[5:7])
        months_to_60 = max((BIRTH_YEAR + 60 - start_year) * 12 - (start_month - 1), 0)
        coast_at_60 = B * (1 + r) ** months_to_60
        coast_swr_annual = coast_at_60 * 0.04

        points.append({
            "date": d,
            "fire_year": fire_year,
            "fire_year_401k_only": fire_year_401k,
            "coast_swr_annual": round(coast_swr_annual, 2),
            "balance": round(B, 2),
        })

    return jsonify({
        "points": points,
        "birth_year": BIRTH_YEAR,
        "target_year": BIRTH_YEAR + 60,
        "target_swr": round(T * 0.04, 2),
        "trend": _linear_trend(points, HOUSE_PURCHASE_DATE, "fire_year"),
        "trend_401k": _linear_trend(points, HOUSE_PURCHASE_DATE, "fire_year_401k_only"),
    })


@bp.route("/api/config", methods=["POST"])
def save_config():
    data = request.get_json()
    db = get_db()
    for key in ("salary", "annual_rent", "first_job_salary", "home_equity"):
        if key in data:
            db.execute(
                "INSERT INTO config (key, value) VALUES (?,?) "
                "ON CONFLICT(key) DO UPDATE SET value=excluded.value",
                (key, str(data[key])),
            )
    db.commit()
    return jsonify({"ok": True})
