package services

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

type Config struct {
	Salary          float64
	AGIUnemployment float64
	AnnualRent      float64
	FirstJobSalary  float64
	HomeEquity      float64
}

func GetConfig(db *sqlx.DB) (Config, error) {
	rows, err := db.Query("SELECT key, value FROM config")
	if err != nil {
		return Config{}, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		m[k] = v
	}
	pf := func(key string) float64 {
		v, _ := strconv.ParseFloat(m[key], 64)
		return v
	}
	return Config{
		Salary:          pf("salary"),
		AGIUnemployment: 19_000,
		AnnualRent:      pf("annual_rent"),
		FirstJobSalary:  pf("first_job_salary"),
		HomeEquity:      pf("home_equity"),
	}, nil
}

func SaveConfig(db *sqlx.DB, salary, annualRent, firstJobSalary, homeEquity float64) error {
	type kv struct {
		k string
		v float64
	}
	pairs := []kv{
		{"salary", salary},
		{"annual_rent", annualRent},
		{"first_job_salary", firstJobSalary},
		{"home_equity", homeEquity},
	}
	for _, p := range pairs {
		_, err := db.Exec(
			`INSERT INTO config (key, value) VALUES (?,?)
			 ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
			p.k, strconv.FormatFloat(p.v, 'f', -1, 64),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func getCurrentInvestment(db *sqlx.DB) (float64, error) {
	var total float64
	err := db.QueryRow(`SELECT COALESCE(SUM(ab.balance),0)
		FROM account_balances ab
		JOIN accounts a ON ab.account_id = a.id
		WHERE a.type = 'brokerage' AND a.name != 'HSA Cash'
		  AND ab.balance > 0
		  AND ab.date = (SELECT MAX(date) FROM account_balances WHERE account_id = ab.account_id)
	`).Scan(&total)
	return total, err
}

type MilestoneItem struct {
	Label     string    `json:"label"`
	Target    float64   `json:"target"`
	Achieved  bool      `json:"achieved"`
	Projected *Crossing `json:"projected"`
	Pct       int       `json:"pct"`
}

type MilestoneGroup struct {
	Label   string          `json:"label"`
	Current float64         `json:"current"`
	Items   []MilestoneItem `json:"items"`
}

type CoastFire struct {
	Label    string   `json:"label"`
	Target   float64  `json:"target"`
	Current  *float64 `json:"current"`
	Achieved bool     `json:"achieved"`
	Pct      int      `json:"pct"`
}

func findCrossing(points []ProjectionPoint, target float64, birthYear int) *Crossing {
	for _, p := range points {
		if p.Value >= target {
			yr, _ := strconv.Atoi(p.Date[:4])
			return &Crossing{Date: p.Date[:7], Age: yr - birthYear}
		}
	}
	return nil
}

func clampPct(current, target float64) int {
	if target == 0 {
		return 0
	}
	p := int(current / target * 100)
	if p > 100 {
		return 100
	}
	if p < 0 {
		return 0
	}
	return p
}

func formatMultiple(x float64) string {
	if x == float64(int(x)) {
		return strconv.Itoa(int(x)) + "x"
	}
	return strconv.FormatFloat(x, 'f', 1, 64) + "x"
}

func GetMilestoneGroups(db *sqlx.DB) ([]MilestoneGroup, error) {
	cfg, err := GetConfig(db)
	if err != nil {
		return nil, err
	}
	inv, err := getCurrentInvestment(db)
	if err != nil {
		return nil, err
	}
	mc, err := GetMonthlyContrib(db)
	if err != nil {
		return nil, err
	}

	birthYear := BirthYear()
	today := time.Now().Format("2006-01-02")
	contribProj := BuildProjections(today, inv, mc.Total, ChartEndYear)
	current4Pct := inv * 0.04

	makeItem := func(label string, target float64) MilestoneItem {
		if target <= 0 {
			return MilestoneItem{}
		}
		achieved := inv >= target
		var proj *Crossing
		if !achieved {
			proj = findCrossing(contribProj.CurrentRate, target, birthYear)
		}
		return MilestoneItem{
			Label:     label,
			Target:    target,
			Achieved:  achieved,
			Projected: proj,
			Pct:       clampPct(inv, target),
		}
	}

	make4PctItem := func(label string, incomeTarget float64) MilestoneItem {
		if incomeTarget <= 0 {
			return MilestoneItem{}
		}
		nwNeeded := incomeTarget * 25
		achieved := current4Pct >= incomeTarget
		var proj *Crossing
		if !achieved {
			proj = findCrossing(contribProj.CurrentRate, nwNeeded, birthYear)
		}
		return MilestoneItem{
			Label:     label,
			Target:    incomeTarget,
			Achieved:  achieved,
			Projected: proj,
			Pct:       clampPct(current4Pct, incomeTarget),
		}
	}

	salary := cfg.Salary

	nwItems := []MilestoneItem{}
	for _, pair := range [][2]interface{}{
		{"300k", 300_000.0}, {"500k", 500_000.0}, {"1M", 1_000_000.0},
		{"2M", 2_000_000.0}, {"3M", 3_000_000.0}, {"3.5M", 3_500_000.0},
	} {
		nwItems = append(nwItems, makeItem(pair[0].(string), pair[1].(float64)))
	}
	if salary > 0 {
		for _, x := range []float64{0.5, 1, 2, 3, 5, 10, 25} {
			nwItems = append(nwItems, makeItem(formatMultiple(x)+" salary", salary*x))
		}
	}

	rule4Items := []MilestoneItem{}
	rule4Targets := [][2]interface{}{
		{"AGI on unemployment", cfg.AGIUnemployment},
		{"Annual rent/mortgage", cfg.AnnualRent},
		{"Salary of first job", cfg.FirstJobSalary},
		{"Six digits", 100_000.0},
	}
	if salary > 0 {
		rule4Targets = append(rule4Targets,
			[2]interface{}{"Half of salary", salary * 0.5},
			[2]interface{}{"80% of salary", salary * 0.8},
			[2]interface{}{"Current salary", salary},
		)
	}
	for _, pair := range rule4Targets {
		item := make4PctItem(pair[0].(string), pair[1].(float64))
		if item.Target > 0 {
			rule4Items = append(rule4Items, item)
		}
	}

	return []MilestoneGroup{
		{Label: "4% Rule of Retirement Net Worth", Current: current4Pct, Items: rule4Items},
		{Label: "Retirement Net Worth", Current: inv, Items: nwItems},
	}, nil
}

func GetCoastFire(db *sqlx.DB) (CoastFire, error) {
	inv, err := getCurrentInvestment(db)
	if err != nil {
		return CoastFire{}, err
	}
	birthYear := BirthYear()
	today := time.Now().Format("2006-01-02")
	coastProj := BuildProjections(today, inv, 0, ChartEndYear)

	coastAt60Date := strconv.Itoa(birthYear+60) + "-01-01"
	var coastAt60 *float64
	for _, p := range coastProj.Coast {
		if p.Date >= coastAt60Date {
			v := p.Value
			coastAt60 = &v
			break
		}
	}
	achieved := coastAt60 != nil && *coastAt60 >= RetirementTarget
	pct := 0
	if coastAt60 != nil {
		pct = clampPct(*coastAt60, RetirementTarget)
	}
	return CoastFire{
		Label:    "Coast FIRE @ 60",
		Target:   RetirementTarget,
		Current:  coastAt60,
		Achieved: achieved,
		Pct:      pct,
	}, nil
}

// --- Benchmarks ---

type BenchmarkTick struct {
	Pct        float64 `json:"pct"`
	ShortLabel string  `json:"short_label"`
	ValueStr   string  `json:"value_str"`
}

type PercentileBar struct {
	UserPct   float64         `json:"user_pct"`
	BadgeText string          `json:"badge_text"`
	Ticks     []BenchmarkTick `json:"ticks"`
}

type FidelityCard struct {
	ByAge    int
	Multiple float64
	Label    string
	Achieved bool
	Progress int
}

type Benchmarks struct {
	Investments PercentileBar
	NetWorth    PercentileBar
	Salary      PercentileBar
	SalaryNote  string
	SavingsRate PercentileBar
	PAWScore    PercentileBar
	PAWNote     string
	Fidelity    []FidelityCard
}

func estimatePct(value float64, lookup [][2]float64) float64 {
	if value <= lookup[0][0] {
		return lookup[0][1]
	}
	if value >= lookup[len(lookup)-1][0] {
		return lookup[len(lookup)-1][1]
	}
	for i := 1; i < len(lookup); i++ {
		if value <= lookup[i][0] {
			v0, p0 := lookup[i-1][0], lookup[i-1][1]
			v1, p1 := lookup[i][0], lookup[i][1]
			return p0 + (value-v0)/(v1-v0)*(p1-p0)
		}
	}
	return 99
}

func clampBar(pct float64) float64 {
	if pct < 3 {
		return 3
	}
	if pct > 97 {
		return 97
	}
	return pct
}

func formatDollarShort(v float64) string {
	if v >= 1_000_000 {
		return "$" + strconv.FormatFloat(v/1_000_000, 'f', 2, 64) + "M"
	}
	if v >= 1_000 {
		return "$" + strconv.FormatFloat(math.Round(v/1000), 'f', 0, 64) + "k"
	}
	return "$" + strconv.FormatFloat(v, 'f', 2, 64)
}

func GetBenchmarks(db *sqlx.DB) (Benchmarks, error) {
	cfg, err := GetConfig(db)
	if err != nil {
		return Benchmarks{}, err
	}
	inv, err := getCurrentInvestment(db)
	if err != nil {
		return Benchmarks{}, err
	}
	mc, err := GetMonthlyContrib(db)
	if err != nil {
		return Benchmarks{}, err
	}

	netWorth := inv + cfg.HomeEquity
	birthYear := BirthYear()
	age := time.Now().Year() - birthYear

	invLookup := [][2]float64{{0, 2}, {1000, 10}, {6000, 25}, {18880, 50}, {55000, 75}, {130000, 90}, {220000, 95}, {500000, 99}}
	nwLookup := [][2]float64{{-50000, 5}, {5000, 25}, {39040, 50}, {120000, 75}, {280000, 90}, {450000, 95}, {1000000, 99}}

	bm := Benchmarks{
		Investments: PercentileBar{
			UserPct:   clampBar(estimatePct(inv, invLookup)),
			BadgeText: formatDollarShort(inv),
			Ticks: []BenchmarkTick{
				{Pct: 25, ShortLabel: "25th", ValueStr: "$6k"},
				{Pct: 50, ShortLabel: "median", ValueStr: "$18.9k"},
				{Pct: 75, ShortLabel: "75th", ValueStr: "$55k"},
				{Pct: 90, ShortLabel: "90th", ValueStr: "$130k"},
			},
		},
		NetWorth: PercentileBar{
			UserPct:   clampBar(estimatePct(netWorth, nwLookup)),
			BadgeText: formatDollarShort(netWorth),
			Ticks: []BenchmarkTick{
				{Pct: 25, ShortLabel: "25th", ValueStr: "$5k"},
				{Pct: 50, ShortLabel: "median", ValueStr: "$39k"},
				{Pct: 75, ShortLabel: "75th", ValueStr: "$120k"},
				{Pct: 90, ShortLabel: "90th", ValueStr: "$280k"},
			},
		},
	}

	if cfg.Salary > 0 {
		salaryLookup := [][2]float64{{78000, 10}, {100000, 25}, {136000, 50}, {165000, 75}, {196000, 90}}
		salPct := estimatePct(cfg.Salary, salaryLookup)
		pctOfMedian := int(cfg.Salary / 136_000 * 100)
		bm.Salary = PercentileBar{
			UserPct:   clampBar(salPct),
			BadgeText: formatDollarShort(cfg.Salary),
			Ticks: []BenchmarkTick{
				{Pct: 10, ShortLabel: "10th", ValueStr: "$78k"},
				{Pct: 50, ShortLabel: "median", ValueStr: "$136k"},
				{Pct: 90, ShortLabel: "90th", ValueStr: "$196k"},
			},
		}
		bm.SalaryNote = strconv.Itoa(pctOfMedian) + "% of local median"

		savingsRate := (mc.Monthly401k + mc.MonthlyRoth) / (cfg.Salary / 12) * 100
		srPos := math.Min(100, savingsRate/50*100)
		bm.SavingsRate = PercentileBar{
			UserPct:   clampBar(srPos),
			BadgeText: fmt.Sprintf("%.1f%% saved", savingsRate),
			Ticks: []BenchmarkTick{
				{Pct: 10, ShortLabel: "avg American", ValueStr: "~5%"},
				{Pct: 30, ShortLabel: "recommended", ValueStr: "15%"},
				{Pct: 50, ShortLabel: "FIRE", ValueStr: "25%"},
				{Pct: 100, ShortLabel: "extreme FIRE", ValueStr: "50%+"},
			},
		}

		expectedNW := float64(age) * cfg.Salary / 10
		pawScore := 0.0
		if expectedNW > 0 {
			pawScore = netWorth / expectedNW
		}
		pawPos := math.Min(97, pawScore/3*100)
		pawLabel := "UAW"
		if pawScore >= 2 {
			pawLabel = "PAW"
		} else if pawScore >= 1 {
			pawLabel = "Avg"
		}
		bm.PAWScore = PercentileBar{
			UserPct:   clampBar(pawPos),
			BadgeText: fmt.Sprintf("%.2f× expected", pawScore),
			Ticks: []BenchmarkTick{
				{Pct: math.Round(0.5 / 3 * 100), ShortLabel: "UAW", ValueStr: "0.5×"},
				{Pct: math.Round(1.0 / 3 * 100), ShortLabel: "avg", ValueStr: "1.0×"},
				{Pct: math.Round(2.0 / 3 * 100), ShortLabel: "PAW", ValueStr: "2.0×"},
			},
		}
		bm.PAWNote = fmt.Sprintf("Expected NW at %d: %s — you're %.1f× that (%s)",
			age, formatDollarShort(expectedNW), pawScore, pawLabel)

		bm.Fidelity = []FidelityCard{
			{ByAge: 30, Multiple: 1, Label: "1× salary", Achieved: true, Progress: 100},
			{ByAge: 35, Multiple: 2, Label: "2× salary", Achieved: inv >= cfg.Salary*2, Progress: clampPct(inv, cfg.Salary*2)},
			{ByAge: 40, Multiple: 3, Label: "3× salary", Achieved: inv >= cfg.Salary*3, Progress: clampPct(inv, cfg.Salary*3)},
			{ByAge: 50, Multiple: 6, Label: "6× salary", Achieved: inv >= cfg.Salary*6, Progress: clampPct(inv, cfg.Salary*6)},
			{ByAge: 60, Multiple: 8, Label: "8× salary", Achieved: inv >= cfg.Salary*8, Progress: clampPct(inv, cfg.Salary*8)},
		}
	}

	return bm, nil
}

// FIREProgressData is the data for the fire-year and SWR charts.
type FIREPoint struct {
	Date             string  `json:"date"`
	FIREYear         int     `json:"fire_year"`
	FIREYear401kOnly int     `json:"fire_year_401k_only"`
	CoastSWRAnnual   float64 `json:"coast_swr_annual"`
	Balance          float64 `json:"balance"`
}

type FIREProgressData struct {
	Points     []FIREPoint `json:"points"`
	BirthYear  int         `json:"birth_year"`
	TargetYear int         `json:"target_year"`
	TargetSWR  float64     `json:"target_swr"`
}

func GetFIREProgressData(db *sqlx.DB) (FIREProgressData, error) {
	mc, err := GetMonthlyContrib(db)
	if err != nil {
		return FIREProgressData{}, err
	}
	birthYear := BirthYear()

	var ids []int
	db.Select(&ids, `SELECT a.id FROM accounts a
		WHERE a.type = 'brokerage' AND a.name != 'HSA Cash'`)
	if len(ids) == 0 {
		return FIREProgressData{
			BirthYear:  birthYear,
			TargetYear: birthYear + 60,
			TargetSWR:  math.Round(RetirementTarget*0.04*100) / 100,
		}, nil
	}

	query, args, _ := sqlx.In(`
		SELECT ab.date, SUM(ab.balance) as total
		FROM account_balances ab WHERE ab.account_id IN (?)
		GROUP BY ab.date ORDER BY ab.date ASC`, ids)
	query = db.Rebind(query)

	type row struct {
		Date  string  `db:"date"`
		Total float64 `db:"total"`
	}
	var rows []row
	if err := db.Select(&rows, query, args...); err != nil {
		return FIREProgressData{}, err
	}

	r := AnnualReturn / 12
	C := mc.Total
	C4 := mc.Monthly401k
	T := RetirementTarget

	fireYearFor := func(dateStr string, B, c float64) int {
		denom := B + c/r
		if denom <= 0 {
			yr, _ := strconv.Atoi(dateStr[:4])
			return yr + 100
		}
		ratio := (T + c/r) / denom
		if ratio <= 1 {
			yr, _ := strconv.Atoi(dateStr[:4])
			return yr
		}
		nMonths := math.Log(ratio) / math.Log(1+r)
		t, _ := time.Parse("2006-01-02", dateStr)
		return t.AddDate(0, int(nMonths), 0).Year()
	}

	points := make([]FIREPoint, 0, len(rows))
	for _, row := range rows {
		B := row.Total
		if B <= 0 {
			continue
		}
		startYear, _ := strconv.Atoi(row.Date[:4])
		startMonth, _ := strconv.Atoi(row.Date[5:7])
		monthsTo60 := (birthYear+60-startYear)*12 - (startMonth - 1)
		if monthsTo60 < 0 {
			monthsTo60 = 0
		}
		coastAt60 := B * math.Pow(1+r, float64(monthsTo60))

		points = append(points, FIREPoint{
			Date:             row.Date,
			FIREYear:         fireYearFor(row.Date, B, C),
			FIREYear401kOnly: fireYearFor(row.Date, B, C4),
			CoastSWRAnnual:   math.Round(coastAt60*0.04*100) / 100,
			Balance:          math.Round(B*100) / 100,
		})
	}

	return FIREProgressData{
		Points:     points,
		BirthYear:  birthYear,
		TargetYear: birthYear + 60,
		TargetSWR:  math.Round(T*0.04*100) / 100,
	}, nil
}
