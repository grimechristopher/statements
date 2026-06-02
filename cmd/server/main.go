package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"pay-dashboard/internal/db"
	"pay-dashboard/internal/handlers"
	"pay-dashboard/internal/handlers/web"
)

func main() {
	godotenv.Load()

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "pay.db"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(handlers.WithDB(database))

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Full pages
	r.Get("/", web.TablePage)
	r.Get("/charts", web.ChartsPage)
	r.Get("/balances", web.BalancesPage)
	r.Get("/milestones", web.MilestonesPage)

	// Table HTMX routes
	r.Get("/rows/new", web.RowsRouter)
	r.Get("/rows/cancel-new", web.RowsRouter)
	r.Post("/rows", web.RowsRouter)
	r.Get("/rows/{id}/edit", web.RowsRouter)
	r.Get("/rows/{id}/cancel", web.RowsRouter)
	r.Delete("/rows/{id}/delete", web.RowsRouter)
	r.Post("/rows/{id}", web.RowsRouter)
	r.Post("/people", web.AddPerson)

	// Charts partials
	r.Get("/partials/charts/{name}", web.ChartsPartialsRouter)

	// Balances
	r.Get("/partials/balances/table", web.BalancesTablePartial)
	r.Get("/partials/charts/retirement", web.RetirementChartPartial)
	r.Post("/api/monarch/sync", web.MonarchSync)

	// Milestones partials + config
	r.Get("/partials/milestones/{name}", web.MilestonesPartialsRouter)
	r.Post("/api/config", web.SaveConfig)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("pay-dashboard listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
