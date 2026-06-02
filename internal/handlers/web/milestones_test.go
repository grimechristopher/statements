package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pay-dashboard/internal/db"
	"pay-dashboard/internal/handlers"
	"pay-dashboard/internal/handlers/web"
)

func TestMilestonesPageLoads(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.MustExec(`INSERT INTO config (key, value) VALUES ('salary', '120000')`)

	mux := http.NewServeMux()
	mux.Handle("/milestones", handlers.WithDB(database)(http.HandlerFunc(web.MilestonesPage)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/milestones")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestMilestonesCardsPartial(t *testing.T) {
	database, _ := db.Open(":memory:")
	defer database.Close()
	database.MustExec(`INSERT INTO config (key, value) VALUES ('salary', '120000')`)
	database.MustExec(`INSERT INTO accounts (monarch_id, name, type, institution) VALUES ('m1','WF','brokerage','WF')`)
	database.MustExec(`INSERT INTO account_balances (account_id, date, balance) VALUES (1,'2026-01-01',200000)`)

	mux := http.NewServeMux()
	mux.Handle("/partials/milestones/", handlers.WithDB(database)(http.HandlerFunc(web.MilestonesPartialsRouter)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, path := range []string{"cards", "benchmarks", "fire-chart", "swr-chart"} {
		resp, err := http.Get(srv.URL + "/partials/milestones/" + path)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("/partials/milestones/%s: want 200, got %d", path, resp.StatusCode)
		}
	}
}
