package db_test

import (
	"testing"

	"pay-dashboard/internal/db"
)

func TestOpenAndSchema(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM pay_statements").Scan(&count)
	if err != nil {
		t.Fatalf("pay_statements table missing: %v", err)
	}
	err = database.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	if err != nil {
		t.Fatalf("accounts table missing: %v", err)
	}
	err = database.QueryRow("SELECT COUNT(*) FROM config").Scan(&count)
	if err != nil {
		t.Fatalf("config table missing: %v", err)
	}
}
