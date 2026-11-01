package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Migrate(conn *sql.DB) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(conn, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("database schema is up to date")
		return nil
	}
	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	slog.Info("database migrations applied")

	return nil
}
