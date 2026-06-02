package services_test

import (
	"testing"

	"pay-dashboard/internal/services"
)

func TestBuildProjections(t *testing.T) {
	pts := services.BuildProjections("2026-01-01", 100_000, 1000, 2069)

	if len(pts.Coast) == 0 {
		t.Fatal("coast points empty")
	}
	if len(pts.CurrentRate) == 0 {
		t.Fatal("current_rate points empty")
	}
	// With contributions, current_rate should exceed coast at end
	last := len(pts.Coast) - 1
	if pts.CurrentRate[last].Value <= pts.Coast[last].Value {
		t.Error("current_rate should exceed coast at end of projection")
	}
}

func TestGetBalanceHistory(t *testing.T) {
	database := newTestDB(t)
	database.MustExec(`INSERT INTO accounts (monarch_id, name, type, institution) VALUES (?,?,?,?)`,
		"m1", "Wealthfront", "brokerage", "Wealthfront")
	database.MustExec(`INSERT INTO account_balances (account_id, date, balance) VALUES (1, '2026-01-01', 50000)`)
	database.MustExec(`INSERT INTO account_balances (account_id, date, balance) VALUES (1, '2026-02-01', 52000)`)

	history, err := services.GetBalanceHistory(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("want 2 points, got %d", len(history))
	}
	if history[0].Date != "2026-01-01" {
		t.Errorf("want first date 2026-01-01, got %s", history[0].Date)
	}
}
