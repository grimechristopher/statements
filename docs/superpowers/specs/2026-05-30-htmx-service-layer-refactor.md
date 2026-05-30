# HTMX + Service Layer Refactor

**Date:** 2026-05-30  
**Status:** Approved for implementation

## Goal

Refactor the Pay Dashboard from a JSON-API + JS-rendered SPA pattern to a hypermedia-first architecture using HTMX, Alpine.js, and a proper service layer. The result should have near-zero hand-written JS, a clean layered architecture ready for a future mobile JSON API, and HTMX best practices throughout.

## Tech Stack (unchanged)

- Flask + Jinja2 + SQLite
- HTMX (HTML over the wire)
- Alpine.js (local toggle state only)
- Chart.js (canvas rendering only — reads data from DOM)
- Tailwind CSS

---

## Architecture: Layered (MTV + Service Layer)

```
Models      → SQLite tables, raw SQL queries (app/db.py — unchanged)
Services    → Business logic, calculations, data assembly (app/services/)
Controllers → Thin Flask routes, call services, return HTML or JSON (app/routes/)
Views       → Jinja2 templates (app/templates/)
```

### Why this structure

Currently all business logic lives inside route handlers, making routes 100-300 lines long. Extracting to a service layer means:

- Controllers stay thin (call service → render template)
- Services are testable Python functions with no HTTP context
- A future JSON API controller calls the same services — no duplication

---

## File Structure

### New directories

```
app/
  services/
    __init__.py
    milestones.py     ← milestone groups, benchmarks, config, PAW score
    balances.py       ← projection building, balance history, coast calc
    charts.py         ← chart data assembly (gross, 401k, annual, retirement)

  routes/
    web/
      __init__.py
      milestones.py   ← full page + all milestones partials
      charts.py       ← full page + all chart partials
      balances.py     ← full page + balances partials
      table.py        ← unchanged (already correct HTMX)
    api/
      __init__.py
      ← stub only, no implemented endpoints in this refactor

  NOTE: POST /api/config lives in routes/web/milestones.py despite the /api/ URL
  prefix — it returns OOB HTML, not JSON. URL prefix is kept for backwards
  compatibility with any bookmarked calls; the handler is a web controller.

  templates/
    partials/
      milestones/
        cards.html         ← milestone card groups (4% rule, NW, Fidelity)
        benchmarks.html    ← percentile bars, PAW score, savings rate
        fire_chart.html    ← canvas + embedded JSON
        swr_chart.html     ← canvas + embedded JSON
      charts/
        gross.html         ← canvas + embedded JSON
        annual.html        ← canvas + embedded JSON
        retirement.html    ← canvas + embedded JSON
      balances/
        summary.html       ← sync status + current stats

  static/
    js/               ← new directory, does not exist yet
      charts.js       ← Chart.js init only (~50 lines). No fetch, no DOM building.
```

### Removed

- `app/routes/milestones.py` (replaced by web/ + services/)
- `app/routes/balances.py` (replaced by web/ + services/)
- `app/routes/charts.py` (replaced by web/ + services/)

### JSON API routes removed

These endpoints return JSON today and will be replaced by HTML partials:

```
/api/milestones-data       → /partials/milestones/cards + /partials/milestones/benchmarks
/api/fire-progress-data    → /partials/milestones/fire-chart
/api/chart-data            → /partials/charts/gross + /partials/charts/annual
/api/balance-chart-data    → /partials/charts/retirement + /partials/balances/summary
```

---

## HTMX Patterns Used

### 1. Lazy partial loading (hx-trigger="load")

Each data section loads independently on page load. The page shell is served instantly; partials fill in as the server computes them.

```html
<div hx-get="/partials/milestones/cards"
     hx-trigger="load"
     hx-swap="innerHTML">
  <!-- skeleton placeholder -->
</div>
```

### 2. Form save with OOB swaps (hx-post + hx-swap-oob)

Config save triggers a single POST. The server returns multiple OOB fragments to update every affected section in one round trip.

```html
<form hx-post="/api/config" hx-swap="none">
  ...inputs...
  <button type="submit">Save</button>
</form>
```

Server response:
```html
<div hx-swap-oob="innerHTML:#milestone-cards">...re-rendered cards...</div>
<div hx-swap-oob="innerHTML:#benchmarks">...re-rendered benchmarks...</div>
<div hx-swap-oob="innerHTML:#save-status">Saved</div>
```

### 3. Chart series toggles (Alpine.js + hx-get)

Alpine owns toggle state locally. When state changes, HTMX fetches a new chart partial with active series as a query param. Server re-renders only the affected chart's embedded JSON.

```html
<div x-data="{
  series: ['actual','contributing','coast'],
  toggle(s) { this.series.includes(s) ? this.series = this.series.filter(x=>x!==s) : this.series.push(s) }
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

### 4. Chart data embedding (server-rendered JSON in HTML)

Chart partials embed data server-side. Chart.js reads from the DOM — no fetch required.

```html
<!-- /partials/charts/retirement -->
<script type="application/json" id="retirement-data">
  {{ chart_data | tojson }}
</script>
<canvas id="chart-retirement"></canvas>
```

```js
// charts.js — runs on htmx:afterSwap
document.addEventListener("htmx:afterSwap", (e) => {
  const dataEl = e.target.querySelector("script[type='application/json']");
  if (dataEl) initChart(e.target.querySelector("canvas"), JSON.parse(dataEl.textContent));
});
```

### 5. Person filter (hx-get with query params)

```html
<select hx-get="/partials/charts/gross"
        hx-target="#gross-chart-container"
        hx-trigger="change"
        name="person">
  <option value="all">All People</option>
</select>
```

---

## Service Layer

### app/services/milestones.py

Extracted from current `app/routes/milestones.py`:

```python
def get_config(db) -> dict
def get_current_investment(db) -> float
def get_monthly_contribution(db) -> dict
def get_milestone_groups(db, cfg, investment) -> list[dict]
def get_benchmarks(db, cfg, investment) -> dict
def get_coast_at_60(investment, birth_year) -> dict
```

### app/services/balances.py

Extracted from current `app/routes/balances.py`:

```python
def build_projections(start_date, balance, monthly) -> dict
def get_balance_history(db) -> list[dict]
def get_retirement_chart_data(db) -> dict
```

### app/services/charts.py

Extracted from current `app/routes/charts.py`:

```python
def get_gross_chart_data(db, person_id) -> dict
def get_annual_chart_data(db, person_id) -> dict
```

---

## Controller Responsibilities

### Web controllers (return `render_template`)

Each partial route:
1. Calls one or more service functions
2. Passes result to `render_template("partials/...")`
3. Returns the fragment

Each full page route:
1. Returns `render_template("milestones.html")` — the shell only
2. Shell contains `hx-get` triggers that load partials

### API controller (returns OOB HTML)

`POST /api/config`:
1. Saves config to DB
2. Calls service functions to recompute affected data
3. Returns OOB HTML response updating cards, benchmarks, and status indicator

---

## What Does Not Change

- `app/db.py` — schema and connection unchanged
- `app/templates/table.html` and `partials/row*.html` — already correct HTMX
- `app/monarch_auth.py` — unchanged
- Tailwind CSS classes — visual design unchanged
- SQLite schema — no migrations needed

---

## Scope Boundaries

**In scope:**
- Extract service layer from all three route files
- Restructure routes into web/ and api/ controllers
- Convert all milestones JS to server-rendered HTML + HTMX
- Convert all charts JS plumbing to HTMX + embedded JSON
- Convert balances fetch() calls to HTMX
- Write `static/js/charts.js` as the single JS file

**Out of scope:**
- Visual design changes
- New features
- JSON API controllers (stub `api/` package only — no implemented endpoints)
- Mobile app
- Authentication
