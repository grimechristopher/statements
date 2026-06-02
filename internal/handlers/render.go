package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
)

// RenderPageWithPartials is like RenderPage but also parses additional partial files
// (needed when a page template calls {{ template "partial" . }} inline).
func RenderPageWithPartials(w http.ResponseWriter, page string, partials []string, data any) {
	files := []string{"templates/base.html", filepath.Join("templates", page+".html")}
	for _, p := range partials {
		files = append(files, filepath.Join("templates/partials", p+".html"))
	}
	t, err := template.New("").Funcs(templateFuncs).ParseFiles(files...)
	if err != nil {
		log.Printf("parse page %s: %v", page, err)
		http.Error(w, "template error", 500)
		return
	}
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("execute page %s: %v", page, err)
	}
}

var templateFuncs = template.FuncMap{
	"toJSON": func(v any) (template.JS, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return template.JS(b), nil
	},
	"deref": func(f *float64) float64 {
		if f == nil {
			return 0
		}
		return *f
	},
	"fmtF": func(f *float64) string {
		if f == nil {
			return ""
		}
		return fmt.Sprintf("%.2f", *f)
	},
	"fmtPct": func(f *float64) string {
		if f == nil {
			return ""
		}
		return fmt.Sprintf("%.2f%%", *f)
	},
	"dollar": func(f float64) string {
		return fmt.Sprintf("$%.0f", f)
	},
	"dollarF": func(f float64) string {
		return fmt.Sprintf("$%.2f", f)
	},
	"not": func(b bool) bool { return !b },
	"dict": func(pairs ...interface{}) map[string]interface{} {
		m := map[string]interface{}{}
		for i := 0; i+1 < len(pairs); i += 2 {
			m[pairs[i].(string)] = pairs[i+1]
		}
		return m
	},
}

// RenderPage parses base.html + the named page template and executes "base",
// which calls {{template "content" .}} defined in the page file.
func RenderPage(w http.ResponseWriter, page string, data any) {
	t, err := template.New("").Funcs(templateFuncs).ParseFiles(
		"templates/base.html",
		filepath.Join("templates", page+".html"),
	)
	if err != nil {
		log.Printf("parse page %s: %v", page, err)
		http.Error(w, "template error", 500)
		return
	}
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("execute page %s: %v", page, err)
	}
}

// RenderPartial parses a single partial file and executes the "partial" template.
// All partial files must contain {{ define "partial" }}...{{ end }}.
func RenderPartial(w http.ResponseWriter, partial string, data any) {
	t, err := template.New("").Funcs(templateFuncs).ParseFiles(
		filepath.Join("templates/partials", partial+".html"),
	)
	if err != nil {
		log.Printf("parse partial %s: %v", partial, err)
		http.Error(w, "template error", 500)
		return
	}
	if err := t.ExecuteTemplate(w, "partial", data); err != nil {
		log.Printf("execute partial %s: %v", partial, err)
	}
}

// RenderPartialToBuffer renders a partial into a buffer (for OOB HTMX swaps).
func RenderPartialToBuffer(buf *bytes.Buffer, partial string, data any) error {
	t, err := template.New("").Funcs(templateFuncs).ParseFiles(
		filepath.Join("templates/partials", partial+".html"),
	)
	if err != nil {
		return err
	}
	return t.ExecuteTemplate(buf, "partial", data)
}
