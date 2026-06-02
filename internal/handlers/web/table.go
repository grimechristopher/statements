package web

import (
	"net/http"
	"strconv"
	"strings"

	"pay-dashboard/internal/handlers"
	"pay-dashboard/internal/services"
)

func floatPtr(r *http.Request, field string) *float64 {
	s := strings.TrimSpace(r.FormValue(field))
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func inputFromForm(r *http.Request) services.StatementInput {
	personID, _ := strconv.Atoi(r.FormValue("person_id"))
	return services.StatementInput{
		PersonID:    personID,
		Source:      r.FormValue("source"),
		PayDate:     r.FormValue("pay_date"),
		HoursWorked: floatPtr(r, "hours_worked"),
		Gross:       floatPtr(r, "gross"),
		TotalTaxes:  floatPtr(r, "total_taxes"),
		Total401k:   floatPtr(r, "total_401k"),
		HSA:         floatPtr(r, "hsa"),
		CashSavings: floatPtr(r, "cash_savings"),
	}
}

type tablePageData struct {
	Rows         []services.Statement
	People       []services.Person
	Sources      []string
	PersonFilter string
	SourceFilter string
}

type rowFormData struct {
	Row    *services.Statement
	People []services.Person
	Mode   string
}

func TablePage(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	personFilter := r.URL.Query().Get("person_id")
	sourceFilter := r.URL.Query().Get("source")

	personID, _ := strconv.Atoi(personFilter)
	rows, _ := services.GetStatements(db, personID, sourceFilter)
	people, _ := services.GetPeople(db)
	sources, _ := services.GetSources(db)

	handlers.RenderPageWithPartials(w, "table", []string{"row"}, tablePageData{
		Rows:         rows,
		People:       people,
		Sources:      sources,
		PersonFilter: personFilter,
		SourceFilter: sourceFilter,
	})
}

// RowsRouter handles all /rows/* HTMX endpoints.
func RowsRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case path == "/rows/new" && r.Method == http.MethodGet:
		newRowForm(w, r)
	case path == "/rows/cancel-new" && r.Method == http.MethodGet:
		w.WriteHeader(200)
	case path == "/rows" && r.Method == http.MethodPost:
		addRow(w, r)
	case strings.HasSuffix(path, "/edit") && r.Method == http.MethodGet:
		editRow(w, r, rowIDFromPath(path, "/edit"))
	case strings.HasSuffix(path, "/cancel") && r.Method == http.MethodGet:
		cancelEdit(w, r, rowIDFromPath(path, "/cancel"))
	case strings.HasSuffix(path, "/delete") && r.Method == http.MethodDelete:
		deleteRow(w, r, rowIDFromPath(path, "/delete"))
	default:
		id := rowIDFromPath(path, "")
		if id > 0 && r.Method == http.MethodPost {
			updateRow(w, r, id)
		}
	}
}

func rowIDFromPath(path, suffix string) int {
	p := strings.TrimPrefix(path, "/rows/")
	p = strings.TrimSuffix(p, suffix)
	p = strings.TrimSuffix(p, "/")
	id, _ := strconv.Atoi(p)
	return id
}

func newRowForm(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	people, _ := services.GetPeople(db)
	handlers.RenderPartial(w, "row_form", rowFormData{Row: nil, People: people, Mode: "new"})
}

func editRow(w http.ResponseWriter, r *http.Request, id int) {
	db := handlers.DBFrom(r.Context())
	row, _ := services.GetStatement(db, id)
	people, _ := services.GetPeople(db)
	handlers.RenderPartial(w, "row_form", rowFormData{Row: &row, People: people, Mode: "edit"})
}

func cancelEdit(w http.ResponseWriter, r *http.Request, id int) {
	db := handlers.DBFrom(r.Context())
	row, _ := services.GetStatement(db, id)
	handlers.RenderPartial(w, "row", row)
}

func addRow(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	services.AddStatement(db, inputFromForm(r))
	id, _ := services.GetLastInsertID(db)
	row, _ := services.GetStatement(db, id)
	handlers.RenderPartial(w, "row", row)
}

func updateRow(w http.ResponseWriter, r *http.Request, id int) {
	db := handlers.DBFrom(r.Context())
	services.UpdateStatement(db, id, inputFromForm(r))
	row, _ := services.GetStatement(db, id)
	handlers.RenderPartial(w, "row", row)
}

func deleteRow(w http.ResponseWriter, r *http.Request, id int) {
	db := handlers.DBFrom(r.Context())
	services.DeleteStatement(db, id)
	w.WriteHeader(200)
}

func AddPerson(w http.ResponseWriter, r *http.Request) {
	db := handlers.DBFrom(r.Context())
	name := strings.TrimSpace(r.FormValue("name"))
	if name != "" {
		services.AddPerson(db, name)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
