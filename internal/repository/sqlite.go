package repository

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// OpenSQLite opens the bundled CGO-free SQLite driver with conservative cache pragmas.
func OpenSQLite(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("repository: sqlite path is required")
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000; PRAGMA trusted_schema=OFF;"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
