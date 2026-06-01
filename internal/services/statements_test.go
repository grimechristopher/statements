package services_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
	"pay-dashboard/internal/db"
	"pay-dashboard/internal/services"
)

func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestGetPeople(t *testing.T) {
	database := newTestDB(t)
	database.MustExec("INSERT INTO people (name) VALUES (?)", "Chris")
	database.MustExec("INSERT INTO people (name) VALUES (?)", "Ashley")

	people, err := services.GetPeople(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 2 {
		t.Fatalf("want 2 people, got %d", len(people))
	}
}

func TestAddAndGetStatements(t *testing.T) {
	database := newTestDB(t)
	database.MustExec("INSERT INTO people (name) VALUES (?)", "Chris")

	gross := 3500.0
	taxes := 840.0
	err := services.AddStatement(database, services.StatementInput{
		PersonID: 1, Source: "Safran", PayDate: "1/5/2024",
		Gross: &gross, TotalTaxes: &taxes,
	})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := services.GetStatements(database, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].Source != "Safran" {
		t.Errorf("want source Safran, got %s", rows[0].Source)
	}
}

func TestDeleteStatement(t *testing.T) {
	database := newTestDB(t)
	database.MustExec("INSERT INTO people (name) VALUES (?)", "Chris")
	gross := 3500.0
	services.AddStatement(database, services.StatementInput{
		PersonID: 1, Source: "Safran", PayDate: "1/5/2024", Gross: &gross,
	})

	rows, _ := services.GetStatements(database, 0, "")
	id := rows[0].ID

	if err := services.DeleteStatement(database, id); err != nil {
		t.Fatal(err)
	}
	rows, _ = services.GetStatements(database, 0, "")
	if len(rows) != 0 {
		t.Fatal("expected no rows after delete")
	}
}
