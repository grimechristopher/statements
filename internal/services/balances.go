package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	AnnualReturn     = 0.10
	RetirementTarget = 3_500_000.0
	ChartEndYear     = 2069
)

var nonInvestmentTypes = map[string]bool{
	"checking": true, "savings": true, "credit": true, "credit_card": true,
	"loan": true, "mortgage": true, "real_estate": true, "vehicle": true,
	"other_liability": true, "cash": true,
}

func BirthYear() int {
	v, _ := strconv.Atoi(os.Getenv("BIRTH_YEAR"))
	if v == 0 {
		return 1990
	}
	return v
}

func isInvestment(accountType string) bool {
	return !nonInvestmentTypes[accountType] && accountType != ""
}

type ProjectionPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

type Crossing struct {
	Date string `json:"date"`
	Age  int    `json:"age"`
}

type Projections struct {
	Coast                  []ProjectionPoint `json:"coast"`
	CurrentRate            []ProjectionPoint `json:"current_rate"`
	CoastCrossed           *Crossing         `json:"coast_crossed"`
	ContribCrossed         *Crossing         `json:"contrib_crossed"`
	Contrib401kOnly        []ProjectionPoint `json:"contrib_401k_only"`
	Contrib401kOnlyCrossed *Crossing         `json:"contrib_401k_only_crossed"`
}

// BuildProjections runs monthly compound interest from startDate to endYear.
// Returns coast (no contrib) and currentRate (with monthlyContrib) series.
func BuildProjections(startDate string, startBalance, monthlyContrib float64, endYear int) Projections {
	r := AnnualReturn / 12
	birthYear := BirthYear()

	start, _ := time.Parse("2006-01-02", startDate)
	y, m := start.Year(), int(start.Month())

	coastBal := startBalance
	contribBal := startBalance

	var coastPts, contribPts []ProjectionPoint
	var coastCrossed, contribCrossed *Crossing

	for y <= endYear {
		d := fmt.Sprintf("%04d-%02d-01", y, m)
		coastPts = append(coastPts, ProjectionPoint{Date: d, Value: math.Round(coastBal*100) / 100})
		contribPts = append(contribPts, ProjectionPoint{Date: d, Value: math.Round(contribBal*100) / 100})

		if coastCrossed == nil && coastBal >= RetirementTarget {
			coastCrossed = &Crossing{Date: d[:7], Age: y - birthYear}
		}
		if contribCrossed == nil && contribBal >= RetirementTarget {
			contribCrossed = &Crossing{Date: d[:7], Age: y - birthYear}
		}

		coastBal = coastBal * (1 + r)
		contribBal = contribBal*(1+r) + monthlyContrib

		m++
		if m > 12 {
			m = 1
			y++
		}
	}

	return Projections{
		Coast:          coastPts,
		CurrentRate:    contribPts,
		CoastCrossed:   coastCrossed,
		ContribCrossed: contribCrossed,
	}
}

type BalancePoint struct {
	Date  string  `db:"date"  json:"date"`
	Value float64 `db:"total" json:"value"`
}

// GetBalanceHistory returns summed brokerage balances by date (ascending),
// excluding accounts named "HSA Cash".
func GetBalanceHistory(db *sqlx.DB) ([]BalancePoint, error) {
	var ids []int
	if err := db.Select(&ids, `SELECT a.id FROM accounts a
		WHERE a.type = 'brokerage' AND a.name != 'HSA Cash'`); err != nil || len(ids) == 0 {
		return nil, err
	}

	query, args, _ := sqlx.In(`
		SELECT ab.date, ROUND(SUM(ab.balance),2) as total
		FROM account_balances ab WHERE ab.account_id IN (?)
		GROUP BY ab.date ORDER BY ab.date ASC`, ids)
	query = db.Rebind(query)

	var pts []BalancePoint
	return pts, db.Select(&pts, query, args...)
}

type MonthlyContrib struct {
	Total       float64
	Monthly401k float64
	MonthlyRoth float64
	AvgBiweekly float64
}

func GetMonthlyContrib(db *sqlx.DB) (MonthlyContrib, error) {
	var avg float64
	err := db.QueryRow(`SELECT COALESCE(AVG(total_401k),0) FROM (
		SELECT total_401k FROM pay_statements
		WHERE total_401k > 200 ORDER BY rowid DESC LIMIT 24
	)`).Scan(&avg)
	if err != nil {
		return MonthlyContrib{}, err
	}
	monthly401k := avg * 26 / 12
	monthlyRoth := (7500.0 * 2) / 12
	return MonthlyContrib{
		Total:       monthly401k + monthlyRoth,
		Monthly401k: monthly401k,
		MonthlyRoth: monthlyRoth,
		AvgBiweekly: avg,
	}, nil
}

type BalanceChartData struct {
	TotalHistory        []BalancePoint `json:"total_history"`
	Projections         Projections    `json:"projections"`
	MonthlyContribution float64        `json:"monthly_contribution"`
	Monthly401k         float64        `json:"monthly_401k"`
	MonthlyRoth         float64        `json:"monthly_roth"`
	AvgBiweekly401k     float64        `json:"avg_biweekly_401k"`
	ActiveBalance       float64        `json:"active_balance"`
	AnnualReturn        float64        `json:"annual_return"`
	BirthYear           int            `json:"birth_year"`
	Target              float64        `json:"target"`
	ChartStart          string         `json:"chart_start"`
}

func GetBalanceChartData(db *sqlx.DB) (BalanceChartData, error) {
	mc, err := GetMonthlyContrib(db)
	if err != nil {
		return BalanceChartData{}, err
	}

	history, err := GetBalanceHistory(db)
	if err != nil {
		return BalanceChartData{}, err
	}

	var activeBalance float64
	db.QueryRow(`SELECT COALESCE(SUM(ab.balance),0) FROM account_balances ab
		JOIN accounts a ON ab.account_id = a.id
		WHERE a.type = 'brokerage' AND a.name != 'HSA Cash'
		  AND ab.balance > 0
		  AND ab.date = (SELECT MAX(date) FROM account_balances WHERE account_id = ab.account_id)
	`).Scan(&activeBalance)

	var proj Projections
	if len(history) > 0 {
		last := history[len(history)-1]
		proj = BuildProjections(last.Date, activeBalance, mc.Total, ChartEndYear)
		p401k := BuildProjections(last.Date, activeBalance, mc.Monthly401k, ChartEndYear)
		proj.Contrib401kOnly = p401k.CurrentRate
		proj.Contrib401kOnlyCrossed = p401k.ContribCrossed
	}

	// Chart start date from oldest pay statement
	chartStart := "2022-04-01"
	var rawDate string
	if err := db.QueryRow("SELECT pay_date FROM pay_statements ORDER BY rowid ASC LIMIT 1").Scan(&rawDate); err == nil {
		iso := ToISO(rawDate)
		if len(iso) >= 7 {
			chartStart = iso[:7] + "-01"
		}
	}

	return BalanceChartData{
		TotalHistory:        history,
		Projections:         proj,
		MonthlyContribution: mc.Total,
		Monthly401k:         mc.Monthly401k,
		MonthlyRoth:         mc.MonthlyRoth,
		AvgBiweekly401k:     mc.AvgBiweekly,
		ActiveBalance:       activeBalance,
		AnnualReturn:        AnnualReturn,
		BirthYear:           BirthYear(),
		Target:              RetirementTarget,
		ChartStart:          chartStart,
	}, nil
}

// SyncResult holds counts from a Monarch sync.
type SyncResult struct {
	Synced    int
	Snapshots int
}

// MonarchSync fetches current balances and historical snapshots from Monarch Money.
func MonarchSync(db *sqlx.DB) (SyncResult, error) {
	sessionID := os.Getenv("MONARCH_SESSION_ID")
	csrfToken := os.Getenv("MONARCH_CSRF_TOKEN")
	cfClearance := os.Getenv("MONARCH_CF_CLEARANCE")

	// Config table overrides env
	rows, _ := db.Query(`SELECT key, value FROM config WHERE key IN (
		'monarch_session_id','monarch_csrf_token','monarch_cf_clearance')`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var k, v string
			rows.Scan(&k, &v)
			switch k {
			case "monarch_session_id":
				sessionID = v
			case "monarch_csrf_token":
				csrfToken = v
			case "monarch_cf_clearance":
				cfClearance = v
			}
		}
	}

	if sessionID == "" || csrfToken == "" {
		return SyncResult{}, fmt.Errorf("MONARCH_SESSION_ID and MONARCH_CSRF_TOKEN must be set")
	}

	gql := func(query string, variables map[string]interface{}) (map[string]interface{}, error) {
		body, _ := json.Marshal(map[string]interface{}{"query": query, "variables": variables})
		req, _ := http.NewRequest("POST", "https://api.monarch.com/graphql", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", fmt.Sprintf("cf_clearance=%s; csrftoken=%s; session_id=%s", cfClearance, csrfToken, sessionID))
		req.Header.Set("x-csrftoken", csrfToken)
		req.Header.Set("client-platform", "web")
		req.Header.Set("Origin", "https://app.monarch.com")
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:150.0) Gecko/20100101 Firefox/150.0")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("Monarch session expired — run monarch_reauth.py")
		}
		var out map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&out)
		return out, nil
	}

	result, err := gql(`{ accounts { id displayName currentBalance type { name } institution { name } } }`, nil)
	if err != nil {
		return SyncResult{}, err
	}

	data, _ := result["data"].(map[string]interface{})
	accounts, _ := data["accounts"].([]interface{})
	today := time.Now().Format("2006-01-02")
	res := SyncResult{}

	for _, a := range accounts {
		acc, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		monarchID, _ := acc["id"].(string)
		name, _ := acc["displayName"].(string)
		accType := ""
		if t, ok := acc["type"].(map[string]interface{}); ok {
			accType, _ = t["name"].(string)
		}
		institution := ""
		if inst, ok := acc["institution"].(map[string]interface{}); ok {
			institution, _ = inst["name"].(string)
		}
		balance, _ := acc["currentBalance"].(float64)

		db.Exec(`INSERT INTO accounts (monarch_id, name, type, institution) VALUES (?,?,?,?)
			ON CONFLICT(monarch_id) DO UPDATE SET name=excluded.name, type=excluded.type, institution=excluded.institution`,
			monarchID, name, accType, institution)

		var accountID int
		db.QueryRow("SELECT id FROM accounts WHERE monarch_id=?", monarchID).Scan(&accountID)

		db.Exec(`INSERT OR REPLACE INTO account_balances (account_id, date, balance) VALUES (?,?,?)`,
			accountID, today, balance)
		res.Synced++

		if isInvestment(accType) {
			time.Sleep(300 * time.Millisecond)
			hist, err := gql(`query($id: UUID!) {
				snapshots: snapshotsForAccount(accountId: $id) { date signedBalance }
			}`, map[string]interface{}{"id": monarchID})
			if err != nil {
				continue
			}
			hdata, _ := hist["data"].(map[string]interface{})
			snaps, _ := hdata["snapshots"].([]interface{})
			for _, s := range snaps {
				snap, ok := s.(map[string]interface{})
				if !ok {
					continue
				}
				d, _ := snap["date"].(string)
				if len(d) > 10 {
					d = d[:10]
				}
				v, _ := snap["signedBalance"].(float64)
				if d != "" {
					db.Exec(`INSERT OR REPLACE INTO account_balances (account_id, date, balance) VALUES (?,?,?)`,
						accountID, d, v)
					res.Snapshots++
				}
			}
		}
	}
	return res, nil
}
