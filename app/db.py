import sqlite3
from pathlib import Path

DB_PATH = Path(__file__).parent.parent / "pay.db"

def get_db():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn

def init_db():
    conn = get_db()
    conn.executescript("""
        CREATE TABLE IF NOT EXISTS people (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL UNIQUE
        );

        CREATE TABLE IF NOT EXISTS pay_statements (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            person_id INTEGER NOT NULL REFERENCES people(id),
            source TEXT NOT NULL DEFAULT 'Safran',
            pay_date TEXT NOT NULL,
            hours_worked REAL,
            gross REAL,
            total_taxes REAL,
            taxes_pct REAL,
            total_401k REAL,
            hsa REAL,
            cash_savings REAL,
            savings_pct REAL
        );
    """)
    conn.commit()
    conn.close()
