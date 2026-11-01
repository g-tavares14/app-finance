package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const migrateGracePeriod = 5 * time.Second

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Migrate(ctx context.Context, dsn string) error {
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- migrateUp(dsn, stop)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		close(stop)

		select {
		case <-done:
			return fmt.Errorf("migrations interrupted: %w", ctx.Err())
		case <-time.After(migrateGracePeriod):
			return fmt.Errorf("gave up waiting for migrations: %w", ctx.Err())
		}
	}
}

func migrateUp(dsn string, stop <-chan struct{}) error {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("opening migration connection: %w", err)
	}

	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		conn.Close()
		return fmt.Errorf("loading migrations: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(conn, &pgxmigrate.Config{})
	if err != nil {
		conn.Close()
		return fmt.Errorf("creating migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		driver.Close()
		return fmt.Errorf("creating migrator: %w", err)
	}

	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-stop:
			m.GracefulStop <- true
		case <-finished:
		}
	}()

	err = m.Up()
	sourceErr, dbErr := m.Close()

	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("database schema is up to date")
		err = nil
	} else if err == nil {
		slog.Info("database migrations applied")
	}

	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}
	if sourceErr != nil || dbErr != nil {
		return fmt.Errorf("closing migrator: %w", errors.Join(sourceErr, dbErr))
	}

	return nil
}
