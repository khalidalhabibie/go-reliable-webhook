package database

import (
	"context"
	"database/sql"
)

func Ping(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}
