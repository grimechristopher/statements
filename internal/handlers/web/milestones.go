package web

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"pay-dashboard/internal/handlers"
	"pay-dashboard/internal/services"
)

type milestonesPageData struct {
	Config services.Config
}

func MilestonesPage(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	cfg, _ := services.GetConfig(db)
	handlers.RenderPage(w, "milestones", milestonesPageData{Config: cfg})
}

func MilestonesPartialsRouter(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/partials/milestones/")
	db := handlers.DBFrom(r.Context())

	switch path {
	case "cards":
		groups, _ := services.GetMilestoneGroups(db)
		coastFire, _ := services.GetCoastFire(db)
		handlers.RenderPartial(w, "milestones/cards", struct {
			Groups    []services.MilestoneGroup
			CoastFire services.CoastFire
		}{Groups: groups, CoastFire: coastFire})

	case "benchmarks":
		bm, _ := services.GetBenchmarks(db)
		handlers.RenderPartial(w, "milestones/benchmarks", bm)

	case "fire-chart":
		data, _ := services.GetFIREProgressData(db)
		handlers.RenderPartial(w, "milestones/fire_chart", struct {
			FIREData services.FIREProgressData
		}{FIREData: data})

	case "swr-chart":
		data, _ := services.GetFIREProgressData(db)
		handlers.RenderPartial(w, "milestones/swr_chart", struct {
			FIREData services.FIREProgressData
		}{FIREData: data})

	default:
		http.NotFound(w, r)
	}
}

func SaveConfig(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())

	pf := func(field string) float64 {
		v, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue(field)), 64)
		return v
	}
	services.SaveConfig(db,
		pf("salary"), pf("annual_rent"), pf("first_job_salary"), pf("home_equity"),
	)

	// Re-render milestone-content and benchmarks-card as OOB swaps
	groups, _ := services.GetMilestoneGroups(db)
	coastFire, _ := services.GetCoastFire(db)
	bm, _ := services.GetBenchmarks(db)

	var cardsHTML, bmHTML bytes.Buffer
	handlers.RenderPartialToBuffer(&cardsHTML, "milestones/cards", struct {
		Groups    []services.MilestoneGroup
		CoastFire services.CoastFire
	}{Groups: groups, CoastFire: coastFire})
	handlers.RenderPartialToBuffer(&bmHTML, "milestones/benchmarks", bm)

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w,
		`<div hx-swap-oob="innerHTML:#milestone-content">%s</div>`+
			`<div hx-swap-oob="innerHTML:#benchmarks-card">%s</div>`+
			`<div hx-swap-oob="innerHTML:#save-status">Saved &#10003;</div>`,
		cardsHTML.String(), bmHTML.String(),
	)
}
