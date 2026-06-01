package handlers

import (
	"context"
	"net/http"

	"github.com/jmoiron/sqlx"
)

type ctxKey int

const dbKey ctxKey = iota

func WithDB(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), dbKey, db)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func DBFrom(ctx context.Context) *sqlx.DB {
	db, _ := ctx.Value(dbKey).(*sqlx.DB)
	return db
}
