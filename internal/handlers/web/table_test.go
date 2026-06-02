package web_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"pay-dashboard/internal/db"
	"pay-dashboard/internal/handlers"
	"pay-dashboard/internal/handlers/web"
)

// TestMain changes the working directory to the project root so that
// relative template paths (templates/base.html, etc.) resolve correctly.
func TestMain(m *testing.M) {
	// Walk up until we find go.mod (project root).
	for {
		if _, err := os.Stat("go.mod"); err == nil {
			break
		}
		if err := os.Chdir(".."); err != nil {
			panic("could not find project root: " + err.Error())
		}
	}
	os.Exit(m.Run())
}

func newTestServer(t *testing.T, routes func(*http.ServeMux, interface{})) *httptest.Server {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.MustExec("INSERT INTO people (name) VALUES (?)", "Chris")
	t.Cleanup(func() { database.Close() })

	mux := http.NewServeMux()
	dbMiddleware := handlers.WithDB(database)
	mux.Handle("/", dbMiddleware(http.HandlerFunc(web.TablePage)))
	mux.Handle("/rows/new", dbMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		web.RowsRouter(w, r)
	})))
	mux.Handle("/rows", dbMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		web.RowsRouter(w, r)
	})))
	mux.Handle("/people", dbMiddleware(http.HandlerFunc(web.AddPerson)))
	return httptest.NewServer(mux)
}

func TestTablePageLoads(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestAddRow(t *testing.T) {
	srv := newTestServer(t, nil)
	defer srv.Close()

	form := strings.NewReader("person_id=1&source=Safran&pay_date=1%2F5%2F2024&gross=3500&total_taxes=840&total_401k=450")
	resp, err := http.Post(srv.URL+"/rows", "application/x-www-form-urlencoded", form)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}
