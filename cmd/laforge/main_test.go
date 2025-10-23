package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// createTestDatabase creates a simple test database with some data
func createTestDatabase(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec(`
		CREATE TABLE test_table (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			value INTEGER
		)
	`)
	if err != nil {
		return err
	}

	// Insert some test data
	_, err = db.Exec("INSERT INTO test_table (name, value) VALUES ('test1', 100), ('test2', 200)")
	if err != nil {
		return err
	}

	return nil
}
