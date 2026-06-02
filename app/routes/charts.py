from flask import Blueprint, render_template, jsonify, request
from app.db import get_db

bp = Blueprint("charts", __name__)

# CPI-U monthly values (CUUR0000SA0) sourced from BLS, Apr 2022 – Apr 2026.
# Base month is 2022-04 (first paycheck). Inflate salary by CPI ratio for each period.
_BASE_SALARY_BIWEEKLY = BASE_ANNUAL_SALARY / 26  # ~$3,346 per bi-weekly period

_CPI = {
    "2022-01": 281.148, "2022-02": 283.716, "2022-03": 287.504,
    "2022-04": 289.109, "2022-05": 292.296, "2022-06": 296.311,
    "2022-07": 296.276, "2022-08": 296.171, "2022-09": 296.808,
    "2022-10": 298.012, "2022-11": 297.711, "2022-12": 296.797,
    "2023-01": 299.170, "2023-02": 300.840, "2023-03": 301.836,
    "2023-04": 303.363, "2023-05": 304.127, "2023-06": 305.109,
    "2023-07": 305.691, "2023-08": 307.026, "2023-09": 307.789,
    "2023-10": 307.671, "2023-11": 307.051, "2023-12": 306.746,
    "2024-01": 308.417, "2024-02": 310.326, "2024-03": 312.332,
    "2024-04": 313.548, "2024-05": 314.069, "2024-06": 314.175,
    "2024-07": 314.540, "2024-08": 314.796, "2024-09": 315.301,
    "2024-10": 315.664, "2024-11": 315.493, "2024-12": 315.605,
    "2025-01": 317.671, "2025-02": 319.082, "2025-03": 319.799,
    "2025-04": 320.795, "2025-05": 321.465, "2025-06": 322.561,
    "2025-07": 323.048, "2025-08": 323.976, "2025-09": 324.800,
    "2025-10": 324.800, "2025-11": 324.122, "2025-12": 324.054,
    "2026-01": 325.252, "2026-02": 326.785, "2026-03": 330.213,
    "2026-04": 333.020, "2026-05": 333.020,
}
_CPI_BASE = _CPI["2022-04"]

def _inflation_adjusted(iso_date):
    ym = iso_date[:7]  # "YYYY-MM"
    cpi = _CPI.get(ym)
    if cpi is None:
        return None
    return round(_BASE_SALARY_BIWEEKLY * (cpi / _CPI_BASE), 2)

def to_iso(date_str):
    """Convert M/D/YYYY or MM/DD/YYYY to YYYY-MM-DD for Chart.js time scale."""
    s = date_str.strip()
    if "-" in s and len(s) == 10:
        return s  # already YYYY-MM-DD
    try:
        parts = s.split("/")
        if len(parts) == 3:
            m, d, y = int(parts[0]), int(parts[1]), int(parts[2])
            if y < 100:
                y += 2000
            return f"{y:04d}-{m:02d}-{d:02d}"
    except (ValueError, IndexError):
        pass
    return s

@bp.route("/charts")
def charts():
    db = get_db()
    people = db.execute("SELECT id, name FROM people ORDER BY name").fetchall()
    return render_template("charts.html", people=people)

@bp.route("/api/chart-data")
def chart_data():
    db = get_db()
    person_id = request.args.get("person_id", "")

    query = """
        SELECT ps.pay_date, ps.gross, ps.total_taxes, ps.taxes_pct,
               ps.total_401k, ps.hsa, ps.cash_savings, ps.savings_pct,
               ps.hours_worked, p.name as person_name
        FROM pay_statements ps
        JOIN people p ON ps.person_id = p.id
        WHERE ps.source != 'EDD'
    """
    params = []
    if person_id:
        query += " AND ps.person_id = ?"
        params.append(person_id)
    query += " ORDER BY ps.pay_date ASC"

    rows = db.execute(query, params).fetchall()

    def v(val):
        return float(val) if val is not None else None

    raw = sorted([{
        "date": to_iso(r["pay_date"]),
        "gross": v(r["gross"]),
        "total_taxes": v(r["total_taxes"]),
        "total_401k": v(r["total_401k"]),
        "hsa": v(r["hsa"]),
        "cash_savings": v(r["cash_savings"]),
        "hours_worked": v(r["hours_worked"]),
        "person": r["person_name"],
    } for r in rows], key=lambda r: r["date"])

    # For duplicate dates keep only the primary paycheck so supplemental
    # disbursements don't skew trend lines. Prefer rows with taxes > 0;
    # among those, take the highest gross.
    primary = {}
    for r in raw:
        key = r["date"]
        if key not in primary:
            primary[key] = r
        else:
            cur = primary[key]
            cur_has_tax = (cur["total_taxes"] or 0) > 0
            r_has_tax   = (r["total_taxes"] or 0) > 0
            if r_has_tax and not cur_has_tax:
                primary[key] = r
            elif r_has_tax and cur_has_tax and (r["gross"] or 0) > (cur["gross"] or 0):
                primary[key] = r

    data = []
    for r in primary.values():
        gross = r["gross"] or 0
        taxes = r["total_taxes"] or 0
        k401  = r["total_401k"] or 0
        hsa   = r["hsa"] or 0
        cash  = r["cash_savings"] or 0
        r["taxes_pct"]      = round(taxes / gross * 100, 2) if gross else None
        r["savings_pct"]    = round((k401 + hsa + cash) / gross * 100, 2) if gross else None
        r["takehome"]       = round(gross - taxes - k401 - hsa - cash, 2) if gross else None
        r["inflation_gross"] = _inflation_adjusted(r["date"])
        data.append(r)
    data.sort(key=lambda r: r["date"])

    # YTD summary: group by year
    # Dates are M/D/YYYY or MM/DD/YYYY — split on "/" and take the last part
    ytd = {}
    for r in data:
        if "/" in r["date"]:
            year = r["date"].split("/")[-1]
        else:
            year = r["date"][:4]
        if year not in ytd:
            ytd[year] = {"gross": 0, "taxes": 0, "k401": 0, "hsa": 0, "savings": 0}
        ytd[year]["gross"]   += r["gross"] or 0
        ytd[year]["taxes"]   += r["total_taxes"] or 0
        ytd[year]["k401"]    += r["total_401k"] or 0
        ytd[year]["hsa"]     += r["hsa"] or 0
        ytd[year]["savings"] += r["cash_savings"] or 0

    for y in ytd.values():
        y["takehome"] = y["gross"] - y["taxes"] - y["k401"] - y["hsa"] - y["savings"]

    return jsonify({"rows": data, "ytd": ytd})
