from datetime import datetime
from flask import Blueprint, render_template, request, redirect, url_for
from app.db import get_db

bp = Blueprint("table", __name__)

def get_people():
    db = get_db()
    return db.execute("SELECT id, name FROM people ORDER BY name").fetchall()

def fmt(val, decimals=2):
    if val is None:
        return ""
    return f"{val:.{decimals}f}"

@bp.route("/")
def index():
    db = get_db()
    person_filter = request.args.get("person_id", "")
    source_filter = request.args.get("source", "")

    query = "SELECT ps.*, p.name as person_name FROM pay_statements ps JOIN people p ON ps.person_id = p.id WHERE 1=1"
    params = []
    if person_filter:
        query += " AND ps.person_id = ?"
        params.append(person_filter)
    if source_filter:
        query += " AND ps.source = ?"
        params.append(source_filter)
    rows = db.execute(query, params).fetchall()

    def _parse_date(s):
        s = (s or "").strip()
        for fmt in ("%m/%d/%Y", "%Y-%m-%d"):
            try:
                return datetime.strptime(s, fmt)
            except ValueError:
                pass
        return datetime.min

    rows = sorted(rows, key=lambda r: _parse_date(r["pay_date"]), reverse=True)
    people = get_people()
    sources = [r[0] for r in db.execute("SELECT DISTINCT source FROM pay_statements ORDER BY source").fetchall()]
    return render_template("table.html", rows=rows, people=people, sources=sources,
                           person_filter=person_filter, source_filter=source_filter)

@bp.route("/rows/<int:row_id>/edit")
def edit_row(row_id):
    db = get_db()
    row = db.execute("SELECT ps.*, p.name as person_name FROM pay_statements ps JOIN people p ON ps.person_id = p.id WHERE ps.id = ?", (row_id,)).fetchone()
    people = get_people()
    return render_template("partials/row_form.html", row=row, people=people, mode="edit")

@bp.route("/rows/<int:row_id>", methods=["POST"])
def update_row(row_id):
    db = get_db()
    f = request.form
    def fv(key):
        val = f.get(key, "").strip().replace(",", "")
        return float(val) if val else None

    gross = fv("gross")
    taxes = fv("total_taxes")
    k401  = fv("total_401k")
    hsa   = fv("hsa")
    cash  = fv("cash_savings")
    taxes_pct   = round(taxes / gross * 100, 2) if gross and taxes else None
    savings_pct = round(((k401 or 0) + (hsa or 0) + (cash or 0)) / gross * 100, 2) if gross else None

    db.execute("""
        UPDATE pay_statements SET
            person_id=?, source=?, pay_date=?, hours_worked=?, gross=?,
            total_taxes=?, taxes_pct=?, total_401k=?, hsa=?, cash_savings=?, savings_pct=?
        WHERE id=?
    """, (f["person_id"], f["source"], f["pay_date"],
          fv("hours_worked"), gross, taxes,
          taxes_pct, k401, hsa,
          cash, savings_pct, row_id))
    db.commit()
    row = db.execute("SELECT ps.*, p.name as person_name FROM pay_statements ps JOIN people p ON ps.person_id = p.id WHERE ps.id = ?", (row_id,)).fetchone()
    people = get_people()
    return render_template("partials/row.html", row=row, people=people)

@bp.route("/rows/<int:row_id>/cancel")
def cancel_edit(row_id):
    db = get_db()
    row = db.execute("SELECT ps.*, p.name as person_name FROM pay_statements ps JOIN people p ON ps.person_id = p.id WHERE ps.id = ?", (row_id,)).fetchone()
    people = get_people()
    return render_template("partials/row.html", row=row, people=people)

@bp.route("/rows/new")
def new_row_form():
    people = get_people()
    return render_template("partials/row_form.html", row=None, people=people, mode="new")

@bp.route("/rows", methods=["POST"])
def add_row():
    db = get_db()
    f = request.form
    def fv(key):
        val = f.get(key, "").strip().replace(",", "")
        return float(val) if val else None

    gross = fv("gross")
    taxes = fv("total_taxes")
    k401  = fv("total_401k")
    hsa   = fv("hsa")
    cash  = fv("cash_savings")
    taxes_pct   = round(taxes / gross * 100, 2) if gross and taxes else None
    savings_pct = round(((k401 or 0) + (hsa or 0) + (cash or 0)) / gross * 100, 2) if gross else None

    db.execute("""
        INSERT INTO pay_statements
        (person_id, source, pay_date, hours_worked, gross, total_taxes,
         taxes_pct, total_401k, hsa, cash_savings, savings_pct)
        VALUES (?,?,?,?,?,?,?,?,?,?,?)
    """, (f["person_id"], f["source"], f["pay_date"],
          fv("hours_worked"), gross, taxes,
          taxes_pct, k401, hsa,
          cash, savings_pct))
    db.commit()
    new_id = db.execute("SELECT last_insert_rowid()").fetchone()[0]
    row = db.execute("SELECT ps.*, p.name as person_name FROM pay_statements ps JOIN people p ON ps.person_id = p.id WHERE ps.id = ?", (new_id,)).fetchone()
    people = get_people()
    return render_template("partials/row.html", row=row, people=people)

@bp.route("/rows/<int:row_id>/delete", methods=["DELETE"])
def delete_row(row_id):
    db = get_db()
    db.execute("DELETE FROM pay_statements WHERE id = ?", (row_id,))
    db.commit()
    return ""

@bp.route("/rows/new/cancel-new")
def cancel_new():
    return ""

@bp.route("/people", methods=["POST"])
def add_person():
    db = get_db()
    name = request.form.get("name", "").strip()
    if name:
        db.execute("INSERT OR IGNORE INTO people (name) VALUES (?)", (name,))
        db.commit()
    return redirect(url_for("table.index"))
