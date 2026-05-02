package db

import (
	"database/sql"
	"errors"
	"log/slog"

	_ "github.com/mattn/go-sqlite3"
)

// InitDB initializes the database connection.
func InitDB(dataSourceName string) (*sql.DB, error) {
	if dataSourceName == "" {
		return nil, errors.New("missing database DSN")
	}

	slog.Info("Initializing database connection", slog.String("dsn", dataSourceName))
	var err error
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
  		PRAGMA synchronous = NORMAL;
  		PRAGMA temp_store = MEMORY;
  		PRAGMA busy_timeout = 5000;`)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}
