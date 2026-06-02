package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pay-dashboard/internal/db"
	"pay-dashboard/internal/handlers"
	"pay-dashboard/internal/handlers/web"
)

func TestChartsPageLoads(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.MustExec("INSERT INTO people (name) VALUES (?)", "Chris")

	mux := http.NewServeMux()
	mux.Handle("/charts", handlers.WithDB(database)(http.HandlerFunc(web.ChartsPage)))
	mux.Handle("/partials/charts/", handlers.WithDB(database)(http.HandlerFunc(web.ChartsPartialsRouter)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/charts")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestGrossPartialLoads(t *testing.T) {
	database, _ := db.Open(":memory:")
	defer database.Close()
	database.MustExec("INSERT INTO people (name) VALUES (?)", "Chris")

	mux := http.NewServeMux()
	mux.Handle("/partials/charts/", handlers.WithDB(database)(http.HandlerFunc(web.ChartsPartialsRouter)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/partials/charts/gross")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("gross partial: want 200, got %d", resp.StatusCode)
	}
}
