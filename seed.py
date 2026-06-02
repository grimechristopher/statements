import csv
import os
from pathlib import Path
from app.db import get_db, init_db
from app import create_app

CSV_PATH = Path(__file__).parent / "pay_summary.csv"


def v(row, key):
    val = row.get(key, "").strip()
    return float(val) if val else None


app = create_app()

with app.app_context():
    db = get_db()

    person_name = os.environ.get("PERSON_NAME", "Me")
    employer    = os.environ.get("EMPLOYER", "Employer")

    db.execute("INSERT OR IGNORE INTO people (name) VALUES (?)", (person_name,))
    db.commit()
    person_id = db.execute("SELECT id FROM people WHERE name = ?", (person_name,)).fetchone()["id"]

    # Clear existing seeded data to allow re-runs
    db.execute("DELETE FROM pay_statements WHERE person_id = ?", (person_id,))
    db.commit()

    with open(CSV_PATH) as f:
        for row in csv.DictReader(f):
            db.execute("""
                INSERT INTO pay_statements
                (person_id, source, pay_date, hours_worked, gross, total_taxes,
                 taxes_pct, total_401k, hsa, cash_savings, savings_pct)
                VALUES (?,?,?,?,?,?,?,?,?,?,?)
            """, (
                person_id, employer,
                row["pay_date"].strip(),
                v(row, "hours_worked"), v(row, "gross"), v(row, "total_taxes"),
                v(row, "taxes_pct"), v(row, "total_401k"), v(row, "hsa"),
                v(row, "cash_savings"), v(row, "savings_pct"),
            ))

    db.commit()
    db.close()
    count = get_db().execute("SELECT COUNT(*) as c FROM pay_statements").fetchone()["c"]
    print(f"Seeded {count} rows for {person_name} from {employer}")
