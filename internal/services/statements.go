package services

import (
	"github.com/jmoiron/sqlx"
)

type Person struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

type Statement struct {
	ID          int      `db:"id"`
	PersonID    int      `db:"person_id"`
	PersonName  string   `db:"person_name"`
	Source      string   `db:"source"`
	PayDate     string   `db:"pay_date"`
	HoursWorked *float64 `db:"hours_worked"`
	Gross       *float64 `db:"gross"`
	TotalTaxes  *float64 `db:"total_taxes"`
	TaxesPct    *float64 `db:"taxes_pct"`
	Total401k   *float64 `db:"total_401k"`
	HSA         *float64 `db:"hsa"`
	CashSavings *float64 `db:"cash_savings"`
	SavingsPct  *float64 `db:"savings_pct"`
}

type StatementInput struct {
	PersonID    int
	Source      string
	PayDate     string
	HoursWorked *float64
	Gross       *float64
	TotalTaxes  *float64
	Total401k   *float64
	HSA         *float64
	CashSavings *float64
}

func GetPeople(db *sqlx.DB) ([]Person, error) {
	var people []Person
	return people, db.Select(&people, "SELECT id, name FROM people ORDER BY name")
}

func AddPerson(db *sqlx.DB, name string) error {
	_, err := db.Exec("INSERT OR IGNORE INTO people (name) VALUES (?)", name)
	return err
}

func GetSources(db *sqlx.DB) ([]string, error) {
	var sources []string
	return sources, db.Select(&sources, "SELECT DISTINCT source FROM pay_statements ORDER BY source")
}

func GetStatements(db *sqlx.DB, personID int, source string) ([]Statement, error) {
	query := `SELECT ps.*, p.name as person_name
		FROM pay_statements ps JOIN people p ON ps.person_id = p.id WHERE 1=1`
	args := []any{}
	if personID > 0 {
		query += " AND ps.person_id = ?"
		args = append(args, personID)
	}
	if source != "" {
		query += " AND ps.source = ?"
		args = append(args, source)
	}
	query += " ORDER BY ps.pay_date DESC"
	var rows []Statement
	return rows, db.Select(&rows, query, args...)
}

func GetStatement(db *sqlx.DB, id int) (Statement, error) {
	var s Statement
	err := db.Get(&s, `SELECT ps.*, p.name as person_name
		FROM pay_statements ps JOIN people p ON ps.person_id = p.id WHERE ps.id = ?`, id)
	return s, err
}

func calcPcts(input StatementInput) (*float64, *float64) {
	if input.Gross == nil || *input.Gross == 0 {
		return nil, nil
	}
	g := *input.Gross
	var taxPct *float64
	if input.TotalTaxes != nil {
		v := *input.TotalTaxes / g * 100
		taxPct = &v
	}
	k401 := 0.0
	if input.Total401k != nil {
		k401 = *input.Total401k
	}
	hsa := 0.0
	if input.HSA != nil {
		hsa = *input.HSA
	}
	cash := 0.0
	if input.CashSavings != nil {
		cash = *input.CashSavings
	}
	v := (k401 + hsa + cash) / g * 100
	savPct := &v
	return taxPct, savPct
}

func AddStatement(db *sqlx.DB, in StatementInput) error {
	taxPct, savPct := calcPcts(in)
	_, err := db.Exec(`INSERT INTO pay_statements
		(person_id, source, pay_date, hours_worked, gross, total_taxes,
		 taxes_pct, total_401k, hsa, cash_savings, savings_pct)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		in.PersonID, in.Source, in.PayDate,
		in.HoursWorked, in.Gross, in.TotalTaxes,
		taxPct, in.Total401k, in.HSA, in.CashSavings, savPct)
	return err
}

func UpdateStatement(db *sqlx.DB, id int, in StatementInput) error {
	taxPct, savPct := calcPcts(in)
	_, err := db.Exec(`UPDATE pay_statements SET
		person_id=?, source=?, pay_date=?, hours_worked=?, gross=?,
		total_taxes=?, taxes_pct=?, total_401k=?, hsa=?, cash_savings=?, savings_pct=?
		WHERE id=?`,
		in.PersonID, in.Source, in.PayDate,
		in.HoursWorked, in.Gross, in.TotalTaxes,
		taxPct, in.Total401k, in.HSA, in.CashSavings, savPct, id)
	return err
}

func DeleteStatement(db *sqlx.DB, id int) error {
	_, err := db.Exec("DELETE FROM pay_statements WHERE id = ?", id)
	return err
}

func GetLastInsertID(db *sqlx.DB) (int, error) {
	var id int
	err := db.QueryRow("SELECT last_insert_rowid()").Scan(&id)
	return id, err
}
