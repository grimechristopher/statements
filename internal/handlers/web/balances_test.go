package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pay-dashboard/internal/db"
	"pay-dashboard/internal/handlers"
	"pay-dashboard/internal/handlers/web"
)

func TestBalancesPageLoads(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	mux := http.NewServeMux()
	mux.Handle("/balances", handlers.WithDB(database)(http.HandlerFunc(web.BalancesPage)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/balances")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestBalancesTablePartialLoads(t *testing.T) {
	database, _ := db.Open(":memory:")
	defer database.Close()
	database.MustExec(`INSERT INTO accounts (monarch_id, name, type, institution) VALUES ('m1','Checking','checking','Chase')`)
	database.MustExec(`INSERT INTO account_balances (account_id, date, balance) VALUES (1,'2026-01-01',1000)`)

	mux := http.NewServeMux()
	mux.Handle("/partials/balances/table", handlers.WithDB(database)(http.HandlerFunc(web.BalancesTablePartial)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/partials/balances/table")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}
