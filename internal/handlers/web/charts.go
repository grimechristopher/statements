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

func ChartsPartialsRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/partials/charts/")
	db := handlers.DBFrom(r.Context())
	personID, _ := strconv.Atoi(r.URL.Query().Get("person_id"))

	rows, ytd, err := services.GetChartData(db, personID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	switch path {
	case "gross":
		type grossData struct {
			Dates     []string   `json:"dates"`
			Gross     []*float64 `json:"gross"`
			Inflation []*float64 `json:"inflation"`
		}
		dates := make([]string, len(rows))
		gross := make([]*float64, len(rows))
		inflation := make([]*float64, len(rows))
		for i, r := range rows {
			dates[i] = r.Date
			gross[i] = r.Gross
			inflation[i] = r.InflationGross
		}
		handlers.RenderPartial(w, "charts/gross", struct{ GrossData grossData }{
			GrossData: grossData{Dates: dates, Gross: gross, Inflation: inflation},
		})

	case "taxes-pct":
		type seriesData struct {
			Dates []string   `json:"dates"`
			Vals  []*float64 `json:"vals"`
		}
		dates := make([]string, len(rows))
		vals := make([]*float64, len(rows))
		for i, r := range rows {
			dates[i] = r.Date
			vals[i] = r.TaxesPct
		}
		handlers.RenderPartial(w, "charts/taxes_pct", struct{ Data seriesData }{
			Data: seriesData{Dates: dates, Vals: vals},
		})

	case "savings-pct":
		type seriesData struct {
			Dates []string   `json:"dates"`
			Vals  []*float64 `json:"vals"`
		}
		dates := make([]string, len(rows))
		vals := make([]*float64, len(rows))
		for i, r := range rows {
			dates[i] = r.Date
			vals[i] = r.SavingsPct
		}
		handlers.RenderPartial(w, "charts/savings_pct", struct{ Data seriesData }{
			Data: seriesData{Dates: dates, Vals: vals},
		})

	case "k401":
		type seriesData struct {
			Dates []string   `json:"dates"`
			Vals  []*float64 `json:"vals"`
		}
		dates := make([]string, len(rows))
		vals := make([]*float64, len(rows))
		for i, r := range rows {
			dates[i] = r.Date
			vals[i] = r.Total401k
		}
		handlers.RenderPartial(w, "charts/k401", struct{ Data seriesData }{
			Data: seriesData{Dates: dates, Vals: vals},
		})

	case "hours":
		type seriesData struct {
			Dates []string   `json:"dates"`
			Vals  []*float64 `json:"vals"`
		}
		dates := make([]string, len(rows))
		vals := make([]*float64, len(rows))
		for i, r := range rows {
			dates[i] = r.Date
			vals[i] = r.HoursWorked
		}
		handlers.RenderPartial(w, "charts/hours", struct{ Data seriesData }{
			Data: seriesData{Dates: dates, Vals: vals},
		})

	case "annual":
		type annualData struct {
			Years   []string  `json:"years"`
			Gross   []float64 `json:"gross"`
			Taxes   []float64 `json:"taxes"`
			K401    []float64 `json:"k401"`
			HSA     []float64 `json:"hsa"`
			Savings []float64 `json:"savings"`
		}
		years := make([]string, 0, len(ytd))
		for y := range ytd {
			years = append(years, y)
		}
		sort.Strings(years)
		ad := annualData{Years: years}
		for _, y := range years {
			yr := ytd[y]
			ad.Gross = append(ad.Gross, yr.Gross)
			ad.Taxes = append(ad.Taxes, yr.Taxes)
			ad.K401 = append(ad.K401, yr.K401)
			ad.HSA = append(ad.HSA, yr.HSA)
			ad.Savings = append(ad.Savings, yr.Savings)
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
		w.Header().Set("Content-Type", "text/html")
		for _, card := range []struct {
			Title string
			Val   float64
			Class string
		}{
			{year + " Gross", ly.Gross, "text-gray-900"},
			{year + " Taxes", ly.Taxes, "text-red-500"},
			{year + " 401k", ly.K401, "text-green-600"},
			{year + " Cash Saved", ly.Savings, "text-blue-600"},
		} {
			fmt.Fprintf(w,
				`<div class="bg-white rounded-lg border border-gray-200 p-4">`+
					`<div class="text-xs text-gray-500 uppercase mb-1">%s</div>`+
					`<div class="text-2xl font-bold %s">$%s</div></div>`,
				card.Title, card.Class,
				strconv.FormatFloat(card.Val, 'f', 0, 64),
			)
		}

	default:
		http.NotFound(w, r)
	}
}
