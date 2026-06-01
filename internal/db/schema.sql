CREATE TABLE IF NOT EXISTS people (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS pay_statements (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id    INTEGER NOT NULL REFERENCES people(id),
    source       TEXT NOT NULL DEFAULT 'Safran',
    pay_date     TEXT NOT NULL,
    hours_worked REAL,
    gross        REAL,
    total_taxes  REAL,
    taxes_pct    REAL,
    total_401k   REAL,
    hsa          REAL,
    cash_savings REAL,
    savings_pct  REAL
);

CREATE TABLE IF NOT EXISTS accounts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    monarch_id  TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    type        TEXT,
    institution TEXT
);

CREATE TABLE IF NOT EXISTS account_balances (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL REFERENCES accounts(id),
    date       TEXT NOT NULL,
    balance    REAL NOT NULL,
    UNIQUE(account_id, date)
);

CREATE TABLE IF NOT EXISTS config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS transactions (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    date               TEXT NOT NULL,
    merchant           TEXT,
    category           TEXT,
    account            TEXT,
    original_statement TEXT,
    notes              TEXT,
    amount             REAL NOT NULL,
    tags               TEXT,
    owner              TEXT
);
CREATE INDEX IF NOT EXISTS idx_txn_date     ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_txn_category ON transactions(category);
CREATE INDEX IF NOT EXISTS idx_txn_account  ON transactions(account);
