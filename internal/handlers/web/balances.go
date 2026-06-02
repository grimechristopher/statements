package web

import (
	"encoding/json"
	"net/http"

	"pay-dashboard/internal/handlers"
	"pay-dashboard/internal/services"
)

type balanceRow struct {
	Date        string  `db:"date"`
	Name        string  `db:"name"`
	Type        string  `db:"type"`
	Institution string  `db:"institution"`
	Balance     float64 `db:"balance"`
}

type accountRow struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func BalancesPage(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	accountFilter := r.URL.Query().Get("account_id")

	var accounts []accountRow
	db.Select(&accounts, "SELECT id, name FROM accounts ORDER BY name")

	handlers.RenderPage(w, "balances", struct {
		Accounts      []accountRow
		AccountFilter string
	}{Accounts: accounts, AccountFilter: accountFilter})
}

func BalancesTablePartial(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	accountID := r.URL.Query().Get("account_id")

	query := `SELECT ab.date, a.name, a.type, a.institution, ab.balance
		FROM account_balances ab JOIN accounts a ON ab.account_id = a.id`
	args := []interface{}{}
	if accountID != "" {
		query += " WHERE a.id = ?"
		args = append(args, accountID)
	}
	query += " ORDER BY ab.date DESC, a.name ASC"

	var rows []balanceRow
	db.Select(&rows, query, args...)
	handlers.RenderPartial(w, "balances/table", struct{ Rows []balanceRow }{Rows: rows})
}

func RetirementChartPartial(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	data, err := services.GetBalanceChartData(db)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	handlers.RenderPartial(w, "charts/retirement", struct {
		ChartData services.BalanceChartData
	}{ChartData: data})
}

func MonarchSync(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	result, err := services.MonarchSync(db)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]int{
		"synced":    result.Synced,
		"snapshots": result.Snapshots,
	})
}
