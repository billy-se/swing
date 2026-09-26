package db

import (
	"database/sql"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(db *sql.DB) {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("Could not start postgres migration driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://db/migrations",
		"postgres", driver,
	)

	if err != nil {
		log.Fatalf("Migration failed to initialize: %v", err)
	}

	/*if err := m.Force(8); err != nil {
		log.Fatalf("Failed to force migration version: %v", err)
	}*/

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occured while running migrations: %v", err)
	}

	log.Printf("Migration ran successfully")
}
