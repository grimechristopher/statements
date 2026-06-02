package services_test

import (
	"testing"

	"pay-dashboard/internal/services"
)

func TestGetConfig(t *testing.T) {
	database := newTestDB(t)
	database.MustExec(`INSERT INTO config (key, value) VALUES ('salary', '120000')`)
	database.MustExec(`INSERT INTO config (key, value) VALUES ('annual_rent', '51240')`)

	cfg, err := services.GetConfig(database)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Salary != 120000 {
		t.Errorf("want salary 120000, got %f", cfg.Salary)
	}
	if cfg.AnnualRent != 51240 {
		t.Errorf("want annual_rent 51240, got %f", cfg.AnnualRent)
	}
	if cfg.AGIUnemployment != 19000 {
		t.Errorf("want agi_unemployment 19000 (hardcoded), got %f", cfg.AGIUnemployment)
	}
}

func TestGetMilestoneGroups(t *testing.T) {
	database := newTestDB(t)
	database.MustExec(`INSERT INTO config (key, value) VALUES ('salary', '150000')`)
	database.MustExec(`INSERT INTO accounts (monarch_id, name, type, institution) VALUES ('m1', 'Wealthfront', 'brokerage', 'WF')`)
	database.MustExec(`INSERT INTO account_balances (account_id, date, balance) VALUES (1, '2026-01-01', 200000)`)

	groups, err := services.GetMilestoneGroups(database)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) < 2 {
		t.Fatalf("want at least 2 milestone groups, got %d", len(groups))
	}
}
