package database

import (
	"context"
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(databaseURL string) (*sql.DB, error) {
	return sql.Open("pgx", databaseURL)
}

func Ping(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}
