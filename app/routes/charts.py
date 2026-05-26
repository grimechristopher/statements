from flask import Blueprint, render_template, jsonify, request
from app.db import get_db

bp = Blueprint("charts", __name__)

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
        WHERE 1=1
    """
    params = []
    if person_id:
        query += " AND ps.person_id = ?"
        params.append(person_id)
    query += " ORDER BY ps.pay_date ASC"

    rows = db.execute(query, params).fetchall()

    def v(val):
        return float(val) if val is not None else None

    data = [{
        "date": r["pay_date"],
        "gross": v(r["gross"]),
        "total_taxes": v(r["total_taxes"]),
        "taxes_pct": v(r["taxes_pct"]),
        "total_401k": v(r["total_401k"]),
        "hsa": v(r["hsa"]),
        "cash_savings": v(r["cash_savings"]),
        "savings_pct": v(r["savings_pct"]),
        "hours_worked": v(r["hours_worked"]),
        "person": r["person_name"],
    } for r in rows]

    # YTD summary: group by year
    # Dates are M/D/YYYY or MM/DD/YYYY — split on "/" and take the last part
    ytd = {}
    for r in data:
        if "/" in r["date"]:
            year = r["date"].split("/")[-1]
        else:
            year = r["date"][:4]
        if year not in ytd:
            ytd[year] = {"gross": 0, "taxes": 0, "k401": 0, "savings": 0}
        ytd[year]["gross"]   += r["gross"] or 0
        ytd[year]["taxes"]   += r["total_taxes"] or 0
        ytd[year]["k401"]    += r["total_401k"] or 0
        ytd[year]["savings"] += r["cash_savings"] or 0

    return jsonify({"rows": data, "ytd": ytd})
