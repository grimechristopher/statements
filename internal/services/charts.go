package services

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

// CPI-U monthly values (CUUR0000SA0) from BLS. Base month: 2022-04.
var cpi = map[string]float64{
	"2022-01": 281.148, "2022-02": 283.716, "2022-03": 287.504,
	"2022-04": 289.109, "2022-05": 292.296, "2022-06": 296.311,
	"2022-07": 296.276, "2022-08": 296.171, "2022-09": 296.808,
	"2022-10": 298.012, "2022-11": 297.711, "2022-12": 296.797,
	"2023-01": 299.170, "2023-02": 300.840, "2023-03": 301.836,
	"2023-04": 303.363, "2023-05": 304.127, "2023-06": 305.109,
	"2023-07": 305.691, "2023-08": 307.026, "2023-09": 307.789,
	"2023-10": 307.671, "2023-11": 307.051, "2023-12": 306.746,
	"2024-01": 308.417, "2024-02": 310.326, "2024-03": 312.332,
	"2024-04": 313.548, "2024-05": 314.069, "2024-06": 314.175,
	"2024-07": 314.540, "2024-08": 314.796, "2024-09": 315.301,
	"2024-10": 315.664, "2024-11": 315.493, "2024-12": 315.605,
	"2025-01": 317.671, "2025-02": 319.082, "2025-03": 319.799,
	"2025-04": 320.795, "2025-05": 321.465, "2025-06": 322.561,
	"2025-07": 323.048, "2025-08": 323.976, "2025-09": 324.800,
	"2025-10": 324.800, "2025-11": 324.122, "2025-12": 324.054,
	"2026-01": 325.252, "2026-02": 326.785, "2026-03": 330.213,
	"2026-04": 333.020, "2026-05": 333.020,
}

const cpiBase = 289.109 // CPI-U 2022-04

// baseSalaryPeriod returns the bi-weekly base salary from BASE_ANNUAL_SALARY env.
// Returns 0 if unset — inflation line is omitted from charts.
func baseSalaryPeriod() float64 {
	annual, _ := strconv.ParseFloat(os.Getenv("BASE_ANNUAL_SALARY"), 64)
	return annual / 26
}

// ToISO converts M/D/YYYY or MM/DD/YYYY to YYYY-MM-DD.
func ToISO(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 10 && s[4] == '-' {
		return s
	}
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return s
	}
	m, _ := strconv.Atoi(parts[0])
	d, _ := strconv.Atoi(parts[1])
	y, _ := strconv.Atoi(parts[2])
	if y < 100 {
		y += 2000
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
}

func inflationAdjusted(isoDate string) *float64 {
	base := baseSalaryPeriod()
	if base == 0 || len(isoDate) < 7 {
		return nil
	}
	ym := isoDate[:7]
	v, ok := cpi[ym]
	if !ok {
		return nil
	}
	result := math_round2(base * v / cpiBase)
	return &result
}

func math_round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// fv dereferences a *float64, returning 0 for nil.
func fv(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

type ChartRow struct {
	Date           string   `json:"date"`
	Gross          *float64 `json:"gross"`
	TotalTaxes     *float64 `json:"total_taxes"`
	Total401k      *float64 `json:"total_401k"`
	HSA            *float64 `json:"hsa"`
	CashSavings    *float64 `json:"cash_savings"`
	HoursWorked    *float64 `json:"hours_worked"`
	TaxesPct       *float64 `json:"taxes_pct"`
	SavingsPct     *float64 `json:"savings_pct"`
	Takehome       *float64 `json:"takehome"`
	InflationGross *float64 `json:"inflation_gross"`
	Person         string   `json:"person"`
}

type YTDYear struct {
	Gross    float64 `json:"gross"`
	Taxes    float64 `json:"taxes"`
	K401     float64 `json:"k401"`
	HSA      float64 `json:"hsa"`
	Savings  float64 `json:"savings"`
	Takehome float64 `json:"takehome"`
}

type dbChartRow struct {
	PayDate     string   `db:"pay_date"`
	Gross       *float64 `db:"gross"`
	TotalTaxes  *float64 `db:"total_taxes"`
	Total401k   *float64 `db:"total_401k"`
	HSA         *float64 `db:"hsa"`
	CashSavings *float64 `db:"cash_savings"`
	HoursWorked *float64 `db:"hours_worked"`
	PersonName  string   `db:"person_name"`
}

// GetChartData returns deduplicated chart rows and YTD aggregates.
// personID=0 means all people. Excludes EDD source rows.
// Deduplication: for rows with the same date, prefer rows with taxes>0; among those, highest gross wins.
func GetChartData(db *sqlx.DB, personID int) ([]ChartRow, map[string]YTDYear, error) {
	query := `SELECT ps.pay_date, ps.gross, ps.total_taxes, ps.total_401k,
		ps.hsa, ps.cash_savings, ps.hours_worked, p.name as person_name
		FROM pay_statements ps JOIN people p ON ps.person_id = p.id
		WHERE ps.source != 'EDD'`
	args := []any{}
	if personID > 0 {
		query += " AND ps.person_id = ?"
		args = append(args, personID)
	}
	query += " ORDER BY ps.pay_date ASC"

	var raw []dbChartRow
	if err := db.Select(&raw, query, args...); err != nil {
		return nil, nil, err
	}

	// Convert dates to ISO and sort
	converted := make([]ChartRow, 0, len(raw))
	for _, r := range raw {
		converted = append(converted, ChartRow{
			Date:        ToISO(r.PayDate),
			Gross:       r.Gross,
			TotalTaxes:  r.TotalTaxes,
			Total401k:   r.Total401k,
			HSA:         r.HSA,
			CashSavings: r.CashSavings,
			HoursWorked: r.HoursWorked,
			Person:      r.PersonName,
		})
	}
	sort.Slice(converted, func(i, j int) bool { return converted[i].Date < converted[j].Date })

	// Deduplicate same-date rows
	primary := map[string]ChartRow{}
	for _, r := range converted {
		cur, exists := primary[r.Date]
		if !exists {
			primary[r.Date] = r
			continue
		}
		curHasTax := cur.TotalTaxes != nil && *cur.TotalTaxes > 0
		rHasTax := r.TotalTaxes != nil && *r.TotalTaxes > 0
		if rHasTax && !curHasTax {
			primary[r.Date] = r
		} else if rHasTax && curHasTax && fv(r.Gross) > fv(cur.Gross) {
			primary[r.Date] = r
		}
	}

	// Sort keys and build final slice with computed fields
	dates := make([]string, 0, len(primary))
	for d := range primary {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	data := make([]ChartRow, 0, len(dates))
	ytd := map[string]YTDYear{}

	for _, d := range dates {
		r := primary[d]
		g := fv(r.Gross)
		t := fv(r.TotalTaxes)
		k := fv(r.Total401k)
		h := fv(r.HSA)
		s := fv(r.CashSavings)

		if g > 0 {
			tp := t / g * 100
			sp := (k + h + s) / g * 100
			th := g - t - k - h - s
			r.TaxesPct = &tp
			r.SavingsPct = &sp
			r.Takehome = &th
		}
		r.InflationGross = inflationAdjusted(d)
		data = append(data, r)

		year := d[:4]
		y := ytd[year]
		y.Gross += g
		y.Taxes += t
		y.K401 += k
		y.HSA += h
		y.Savings += s
		ytd[year] = y
	}
	for year, y := range ytd {
		y.Takehome = y.Gross - y.Taxes - y.K401 - y.HSA - y.Savings
		ytd[year] = y
	}

	return data, ytd, nil
}
