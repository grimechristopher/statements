package db

import (
	_ "embed"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

func Open(dsn string) (*sqlx.DB, error) {
	database, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	database.SetMaxOpenConns(1) // SQLite is single-writer
	if _, err := database.Exec(schema); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return database, nil
}
