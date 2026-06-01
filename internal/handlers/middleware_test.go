package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
	"pay-dashboard/internal/handlers"
)

func TestDBMiddleware(t *testing.T) {
	db, _ := sqlx.Open("sqlite", ":memory:")
	defer db.Close()

	called := false
	handler := handlers.WithDB(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := handlers.DBFrom(r.Context())
		if got == nil {
			t.Fatal("expected db in context, got nil")
		}
		called = true
	}))

	req := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Fatal("handler was not called")
	}
}
