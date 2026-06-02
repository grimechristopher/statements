package web

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"pay-dashboard/internal/handlers"
	"pay-dashboard/internal/services"
)

type chartsPageData struct {
	People []services.Person
}

func ChartsPage(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	people, _ := services.GetPeople(db)
	handlers.RenderPage(w, "charts", chartsPageData{People: people})
}

// chartPoint is one {x, y} datum for a time-scale chart.
type chartPoint struct {
	X string  `json:"x"`
	Y float64 `json:"y"`
}

// chartPoint2 carries an extra field (inflation or gross) for rich tooltips.
type chartPoint2 struct {
	X          string   `json:"x"`
	Y          float64  `json:"y"`
	Inflation  *float64 `json:"inflation,omitempty"`
	Gross      *float64 `json:"gross,omitempty"`
}

type personSeries struct {
	Person string       `json:"person"`
	Points []chartPoint `json:"points"`
}

type personSeries2 struct {
	Person string        `json:"person"`
	Points []chartPoint2 `json:"points"`
}

func ChartsPartialsRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/partials/charts/")
	db := handlers.DBFrom(r.Context())
	personID, _ := strconv.Atoi(r.URL.Query().Get("person_id"))

	rows, ytd, err := services.GetChartData(db, personID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Group rows by person
	personOrder := []string{}
	byPerson := map[string][]services.ChartRow{}
	for _, r := range rows {
		if _, ok := byPerson[r.Person]; !ok {
			personOrder = append(personOrder, r.Person)
		}
		byPerson[r.Person] = append(byPerson[r.Person], r)
	}
	multi := len(personOrder) > 1

	switch path {
	case "gross":
		type grossData struct {
			Series      []personSeries2 `json:"series"`
			MultiPerson bool            `json:"multi_person"`
		}
		series := make([]personSeries2, 0, len(personOrder))
		for _, p := range personOrder {
			pts := make([]chartPoint2, 0, len(byPerson[p]))
			for _, r := range byPerson[p] {
				if r.Gross == nil {
					continue
				}
				pts = append(pts, chartPoint2{X: r.Date, Y: *r.Gross, Inflation: r.InflationGross})
			}
			series = append(series, personSeries2{Person: p, Points: pts})
		}
		handlers.RenderPartial(w, "charts/gross", struct{ GrossData grossData }{
			GrossData: grossData{Series: series, MultiPerson: multi},
		})

	case "taxes-pct":
		type pctData struct {
			Series      []personSeries `json:"series"`
			MultiPerson bool           `json:"multi_person"`
		}
		series := buildPctSeries(personOrder, byPerson, func(r services.ChartRow) *float64 { return r.TaxesPct })
		handlers.RenderPartial(w, "charts/taxes_pct", struct{ Data pctData }{
			Data: pctData{Series: series, MultiPerson: multi},
		})

	case "savings-pct":
		type pctData struct {
			Series      []personSeries `json:"series"`
			MultiPerson bool           `json:"multi_person"`
		}
		series := buildPctSeries(personOrder, byPerson, func(r services.ChartRow) *float64 { return r.SavingsPct })
		handlers.RenderPartial(w, "charts/savings_pct", struct{ Data pctData }{
			Data: pctData{Series: series, MultiPerson: multi},
		})

	case "k401":
		type k401Data struct {
			Series      []personSeries2 `json:"series"`
			MultiPerson bool            `json:"multi_person"`
		}
		series := make([]personSeries2, 0, len(personOrder))
		for _, p := range personOrder {
			pts := make([]chartPoint2, 0, len(byPerson[p]))
			for _, r := range byPerson[p] {
				if r.Total401k == nil {
					continue
				}
				pts = append(pts, chartPoint2{X: r.Date, Y: *r.Total401k, Gross: r.Gross})
			}
			series = append(series, personSeries2{Person: p, Points: pts})
		}
		handlers.RenderPartial(w, "charts/k401", struct{ Data k401Data }{
			Data: k401Data{Series: series, MultiPerson: multi},
		})

	case "hours":
		type hoursData struct {
			Points []chartPoint `json:"points"`
		}
		pts := make([]chartPoint, 0, len(rows))
		for _, r := range rows {
			if r.HoursWorked != nil && *r.HoursWorked > 0 {
				pts = append(pts, chartPoint{X: r.Date, Y: *r.HoursWorked})
			}
		}
		handlers.RenderPartial(w, "charts/hours", struct{ Data hoursData }{
			Data: hoursData{Points: pts},
		})

	case "annual":
		type annualPerson struct {
			Person  string    `json:"person"`
			Gross   []float64 `json:"gross"`
			Taxes   []float64 `json:"taxes"`
			K401    []float64 `json:"k401"`
			Takehome []float64 `json:"takehome"`
		}
		type annualData struct {
			Years      []string       `json:"years"`
			Gross      []float64      `json:"gross"`
			Taxes      []float64      `json:"taxes"`
			K401       []float64      `json:"k401"`
			Takehome   []float64      `json:"takehome"`
			MultiPerson bool          `json:"multi_person"`
			ByPerson   []annualPerson `json:"by_person"`
		}
		years := make([]string, 0, len(ytd))
		for y := range ytd {
			years = append(years, y)
		}
		sort.Strings(years)

		// Per-person annual breakdown from rows
		pYtd := map[string]map[string]struct{ gross, taxes, k401, takehome float64 }{}
		for _, r := range rows {
			yr := r.Date[:4]
			if pYtd[r.Person] == nil {
				pYtd[r.Person] = map[string]struct{ gross, taxes, k401, takehome float64 }{}
			}
			entry := pYtd[r.Person][yr]
			entry.gross += fv64(r.Gross)
			entry.taxes += fv64(r.TotalTaxes)
			entry.k401 += fv64(r.Total401k)
			entry.takehome += fv64(r.Takehome)
			pYtd[r.Person][yr] = entry
		}
		byPersonAnnual := make([]annualPerson, 0, len(personOrder))
		for _, p := range personOrder {
			ap := annualPerson{Person: p}
			for _, y := range years {
				e := pYtd[p][y]
				ap.Gross = append(ap.Gross, e.gross)
				ap.Taxes = append(ap.Taxes, e.taxes)
				ap.K401 = append(ap.K401, e.k401)
				ap.Takehome = append(ap.Takehome, e.takehome)
			}
			byPersonAnnual = append(byPersonAnnual, ap)
		}

		ad := annualData{Years: years, MultiPerson: multi, ByPerson: byPersonAnnual}
		for _, y := range years {
			yr := ytd[y]
			ad.Gross = append(ad.Gross, yr.Gross)
			ad.Taxes = append(ad.Taxes, yr.Taxes)
			ad.K401 = append(ad.K401, yr.K401)
			ad.Takehome = append(ad.Takehome, yr.Takehome)
		}
		handlers.RenderPartial(w, "charts/annual", struct{ AnnualData annualData }{AnnualData: ad})

	case "summary-cards":
		years := make([]string, 0, len(ytd))
		for y := range ytd {
			years = append(years, y)
		}
		sort.Strings(years)
		if len(years) == 0 {
			return
		}
		ly := ytd[years[len(years)-1]]
		year := years[len(years)-1]
		cardLabel := ""
		if multi {
			cardLabel = "Combined "
		} else if len(personOrder) > 0 {
			cardLabel = personOrder[0] + " "
		}
		w.Header().Set("Content-Type", "text/html")
		for _, card := range []struct {
			Title string
			Val   float64
			Class string
		}{
			{year + " " + cardLabel + "Gross", ly.Gross, "text-gray-900"},
			{year + " " + cardLabel + "Taxes", ly.Taxes, "text-red-500"},
			{year + " " + cardLabel + "401k", ly.K401, "text-green-600"},
			{year + " " + cardLabel + "Cash Saved", ly.Savings, "text-blue-600"},
		} {
			fmt.Fprintf(w,
				`<div class="bg-white rounded-lg border border-gray-200 p-4">`+
					`<div class="text-xs text-gray-500 uppercase mb-1">%s</div>`+
					`<div class="text-2xl font-bold %s">$%s</div></div>`,
				card.Title, card.Class,
				strconv.FormatFloat(card.Val, 'f', 0, 64),
			)
		}

	case "retirement-summary":
		retData, err := services.GetBalanceChartData(db)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if len(retData.TotalHistory) == 0 {
			fmt.Fprint(w, `<p class="text-sm text-gray-400 col-span-4">No investment data — sync from the Balances tab.</p>`)
			return
		}
		latest := retData.TotalHistory[len(retData.TotalHistory)-1]
		cx := retData.Projections.ContribCrossed
		coastX := retData.Projections.CoastCrossed
		cxLabel := "Beyond 2069"
		if cx != nil {
			cxLabel = cx.Date[:4] + " · age " + strconv.Itoa(cx.Age)
		}
		coastLabel := "Beyond 2069"
		if coastX != nil {
			coastLabel = coastX.Date[:4] + " · age " + strconv.Itoa(coastX.Age)
		}
		for _, card := range []struct{ Title, Val, Class string }{
			{"Current Balance", "$" + strconv.FormatFloat(latest.Value, 'f', 0, 64), "text-gray-900"},
			{fmt.Sprintf("Monthly 401k + Roth"), "$" + strconv.FormatFloat(retData.MonthlyContribution, 'f', 0, 64) + "/mo", "text-indigo-600"},
			{"$3.5M w/ contributions", cxLabel, "text-green-600"},
			{"$3.5M coast only", coastLabel, "text-blue-600"},
		} {
			fmt.Fprintf(w,
				`<div class="bg-white rounded-lg border border-gray-200 p-4">`+
					`<div class="text-xs text-gray-500 uppercase mb-1">%s</div>`+
					`<div class="text-xl font-bold %s">%s</div></div>`,
				card.Title, card.Class, card.Val,
			)
		}

	default:
		http.NotFound(w, r)
	}
}

func buildPctSeries(personOrder []string, byPerson map[string][]services.ChartRow, field func(services.ChartRow) *float64) []personSeries {
	series := make([]personSeries, 0, len(personOrder))
	for _, p := range personOrder {
		pts := make([]chartPoint, 0, len(byPerson[p]))
		for _, r := range byPerson[p] {
			v := field(r)
			if v == nil {
				continue
			}
			pts = append(pts, chartPoint{X: r.Date, Y: *v})
		}
		series = append(series, personSeries{Person: p, Points: pts})
	}
	return series
}

func fv64(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}
