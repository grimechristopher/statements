# HTMX + Service Layer Refactor (Go Port)

**Date:** 2026-05-30  
**Status:** Approved for implementation

## Goal

Port the Pay Dashboard from Python/Flask to Go, and simultaneously refactor from a
JSON-API + JS-rendered SPA pattern to a hypermedia-first architecture using HTMX.
Same SQLite schema, same visual design, same features. Near-zero hand-written JS.
Clean layered Go architecture ready for a future mobile JSON API.

---

## Tech Stack

### Backend (new)
- **Go** — standard library `net/http` as the foundation
- **chi** — idiomatic router, wraps `net/http`, no framework magic
- **`html/template`** — stdlib server-side templates, replaces Jinja2
- **`sqlx`** — thin wrapper over `database/sql`, struct scanning for rows
- **`modernc.org/sqlite`** — pure-Go SQLite driver, no CGO required
- **`godotenv`** — `.env` file loading (equivalent to python-dotenv)

### Frontend (unchanged)
- **HTMX** — HTML over the wire, all server communication
- **Alpine.js** — local toggle state only (series visibility buttons)
- **Chart.js** — canvas rendering only, reads data from DOM
- **Tailwind CSS** — unchanged

---

## Architecture: Layered (Handler → Service → DB)

```
DB          → SQLite via sqlx (internal/db/)
Services    → Business logic, calculations (internal/services/)
Handlers    → Thin chi handlers, call services, render templates (internal/handlers/)
Templates   → html/template partials, rendered server-side (templates/)
```

### Why this structure

- Handlers stay thin: call service → execute template → done
- Services are pure Go functions, no HTTP context, fully testable
- A future JSON API handler calls the same services — no duplication
- `internal/` package prevents external import (Go convention)

---

## Project Layout

```
pay-dashboard/
  cmd/
    server/
      main.go             ← entry point: load env, open DB, build router, listen

  internal/
    db/
      db.go               ← sqlx.Open, schema init (port of app/db.py)
      schema.sql          ← CREATE TABLE statements (extracted from db.go)

    services/
      milestones.go       ← GetConfig, GetCurrentInvestment, GetMonthlyContribution,
                             GetMilestoneGroups, GetBenchmarks, GetCoastAt60
      balances.go         ← BuildProjections, GetBalanceHistory, GetRetirementChartData
      charts.go           ← GetGrossChartData, GetAnnualChartData

    handlers/
      web/
        milestones.go     ← full page + all milestones partials + POST /api/config
        charts.go         ← full page + all chart partials
        balances.go       ← full page + balances partial
        table.go          ← statements table (already correct HTMX, port only)
      middleware.go       ← DB injection middleware

  templates/
    base.html
    table.html
    charts.html
    milestones.html
    balances.html
    partials/
      milestones/
        cards.html        ← milestone card groups (4% rule, NW, Fidelity)
        benchmarks.html   ← percentile bars, PAW score, savings rate
        fire_chart.html   ← canvas + embedded JSON
        swr_chart.html    ← canvas + embedded JSON
      charts/
        gross.html        ← canvas + embedded JSON
        annual.html       ← canvas + embedded JSON
        retirement.html   ← canvas + embedded JSON
      balances/
        summary.html      ← sync status + current stats
      row.html            ← statement row (unchanged)
      row_form.html       ← inline edit form (unchanged)

  static/
    js/
      charts.js           ← Chart.js init only (~50 lines), no fetch, no DOM building

  go.mod
  go.sum
  .env
  .env.example
  .gitignore
```

---

## Routing (chi)

```go
// cmd/server/main.go
r := chi.NewRouter()
r.Use(middleware.WithDB(db))

// Full pages
r.Get("/",           web.TablePage)
r.Get("/charts",     web.ChartsPage)
r.Get("/milestones", web.MilestonesPage)
r.Get("/balances",   web.BalancesPage)

// HTMX partials — milestones
r.Get("/partials/milestones/cards",      web.MilestoneCards)
r.Get("/partials/milestones/benchmarks", web.MilestoneBenchmarks)
r.Get("/partials/milestones/fire-chart", web.FireChart)
r.Get("/partials/milestones/swr-chart",  web.SWRChart)

// HTMX partials — charts
r.Get("/partials/charts/gross",      web.GrossChart)
r.Get("/partials/charts/annual",     web.AnnualChart)
r.Get("/partials/charts/retirement", web.RetirementChart)

// HTMX partials — balances
r.Get("/partials/balances/summary", web.BalancesSummary)

// Form actions
r.Post("/api/config", web.SaveConfig)  ← returns OOB HTML, not JSON

// Static
r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

// Statements CRUD (HTMX, unchanged behaviour)
r.Get("/api/statements",         web.StatementRows)
r.Post("/api/statements",        web.AddStatement)
r.Get("/api/statements/{id}",    web.StatementRow)
r.Put("/api/statements/{id}",    web.UpdateStatement)
r.Delete("/api/statements/{id}", web.DeleteStatement)
```

---

## Handler Pattern

Every handler follows the same shape — thin, no business logic:

```go
func MilestoneCards(w http.ResponseWriter, r *http.Request) {
    db := middleware.DBFrom(r.Context())
    cfg := services.GetConfig(db)
    inv := services.GetCurrentInvestment(db)
    groups := services.GetMilestoneGroups(db, cfg, inv)
    render(w, "partials/milestones/cards.html", groups)
}
```

`render()` is a shared helper that executes the named template into `w`.

---

## Template Syntax Port (Jinja2 → html/template)

| Jinja2 | html/template |
|--------|---------------|
| `{{ var }}` | `{{ .Var }}` |
| `{% for x in items %}` | `{{ range .Items }}` |
| `{% if cond %}` | `{{ if .Cond }}` |
| `{% extends "base.html" %}` | `{{ template "base" . }}` |
| `{% block content %}` | `{{ define "content" }}` |
| `{{ val \| tojson }}` | `{{ .Val \| toJSON }}` (custom func) |

One custom template function needed: `toJSON` — marshals a Go value to a JSON string
for embedding chart data in `<script type="application/json">` tags.

---

## HTMX Patterns (unchanged from Python spec)

### 1. Lazy partial loading

```html
<div hx-get="/partials/milestones/cards"
     hx-trigger="load"
     hx-swap="innerHTML">
  <!-- skeleton -->
</div>
```

### 2. Config save with OOB swaps

```html
<form hx-post="/api/config" hx-swap="none">...</form>
```

Server response (from `SaveConfig` handler):

```html
<div hx-swap-oob="innerHTML:#milestone-cards">{{ re-rendered cards }}</div>
<div hx-swap-oob="innerHTML:#benchmarks">{{ re-rendered benchmarks }}</div>
<div hx-swap-oob="innerHTML:#save-status">Saved</div>
```

### 3. Chart series toggles (Alpine.js + hx-get)

```html
<div x-data="{
  series: ['actual','contributing','coast'],
  toggle(s) { this.series.includes(s)
    ? this.series = this.series.filter(x=>x!==s)
    : this.series.push(s) }
}">
  <button @click="toggle('coast')" :class="series.includes('coast') && 'active'">
    Coast FIRE
  </button>
  <div hx-get="/partials/charts/retirement"
       hx-trigger="seriesChange from:closest div"
       hx-vals="js:{series: series.join(',')}">
  </div>
</div>
```

### 4. Chart data embedding

```html
<!-- templates/partials/charts/retirement.html -->
<script type="application/json" id="retirement-data">
  {{ .ChartData | toJSON }}
</script>
<canvas id="chart-retirement"></canvas>
```

```js
// static/js/charts.js
document.addEventListener("htmx:afterSwap", (e) => {
  const dataEl = e.target.querySelector("script[type='application/json']");
  if (dataEl) initChart(
    e.target.querySelector("canvas"),
    JSON.parse(dataEl.textContent)
  );
});
```

---

## Service Layer

### internal/services/milestones.go

```go
func GetConfig(db *sqlx.DB) Config
func GetCurrentInvestment(db *sqlx.DB) float64
func GetMonthlyContribution(db *sqlx.DB) MonthlyContribution
func GetMilestoneGroups(db *sqlx.DB, cfg Config, investment float64) []MilestoneGroup
func GetBenchmarks(db *sqlx.DB, cfg Config, investment float64) Benchmarks
func GetCoastAt60(investment float64, birthYear int) CoastProjection
```

### internal/services/balances.go

```go
func BuildProjections(startDate time.Time, balance, monthly float64) Projections
func GetBalanceHistory(db *sqlx.DB) []BalancePoint
func GetRetirementChartData(db *sqlx.DB) RetirementChartData
```

### internal/services/charts.go

```go
func GetGrossChartData(db *sqlx.DB, personID int) GrossChartData
func GetAnnualChartData(db *sqlx.DB, personID int) AnnualChartData
```

All service functions take a `*sqlx.DB` and return typed structs. No `http.Request`,
no `http.ResponseWriter`. Fully testable without an HTTP server.

---

## DB / sqlx

Schema is identical to the Python app — no migrations needed. Connection setup:

```go
// internal/db/db.go
func Open(path string) (*sqlx.DB, error) {
    db, err := sqlx.Open("sqlite", path)
    // run schema.sql to create tables if not exist
    return db, err
}
```

`modernc.org/sqlite` registers as `"sqlite"` driver. No CGO, works on all platforms.

---

## Environment Variables (unchanged)

```
SECRET_KEY=...
BIRTH_YEAR=1999
PERSON_NAME=Me
EMPLOYER=Employer
MONARCH_EMAIL=...
MONARCH_PASSWORD=...
MONARCH_MFA_SECRET=...
```

Loaded via `godotenv.Load()` in `main.go` before anything else.

---

## What Does Not Change

- SQLite schema — no migrations
- All template HTML structure and Tailwind classes — mechanical syntax port only
- HTMX attributes — identical
- Alpine.js — identical
- Chart.js — identical
- `static/js/charts.js` — identical
- `.env.example` — identical
- `.gitignore` — updated for Go (`*.exe`, `pay-dashboard` binary)

---

## Scope Boundaries

**In scope:**
- Full Go port of Python/Flask backend
- Service layer extraction (milestones, balances, charts)
- chi router setup with all existing routes
- html/template port of all Jinja2 templates
- HTMX refactor: replace all JSON APIs with HTML partials
- `static/js/charts.js` — single JS file for Chart.js init
- `go.mod` with all dependencies

**Out of scope:**
- Visual design changes
- New features
- JSON API controllers for mobile (stub `handlers/api/` package only)
- Monarch Money sync (port the auth logic but keep it as a standalone script)
- Authentication / multi-user
