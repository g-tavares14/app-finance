package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const migrateGracePeriod = 5 * time.Second

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migrateResult struct {
	applied bool
	err     error
}

func Migrate(ctx context.Context, dsn string) (bool, error) {
	done := make(chan migrateResult, 1)
	go func() {
		applied, err := migrateUp(ctx, dsn)
		done <- migrateResult{applied: applied, err: err}
	}()

	select {
	case result := <-done:
		return result.applied, result.err
	case <-ctx.Done():
		select {
		case <-done:
			return false, fmt.Errorf("migrations interrupted: %w", ctx.Err())
		case <-time.After(migrateGracePeriod):
			return false, fmt.Errorf("gave up waiting for migrations: %w", ctx.Err())
		}
	}
}

func migrateUp(ctx context.Context, dsn string) (bool, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return false, fmt.Errorf("opening migration connection: %w", err)
	}

	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		conn.Close()
		return false, fmt.Errorf("loading migrations: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(conn, &pgxmigrate.Config{})
	if err != nil {
		conn.Close()
		return false, fmt.Errorf("creating migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		driver.Close()
		return false, fmt.Errorf("creating migrator: %w", err)
	}

	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			m.GracefulStop <- true
		case <-finished:
		}
	}()

	err = m.Up()
	sourceErr, dbErr := m.Close()

	applied := err == nil
	if errors.Is(err, migrate.ErrNoChange) {
		err = nil
	}

	if err != nil {
		return false, fmt.Errorf("applying migrations: %w", err)
	}
	if sourceErr != nil || dbErr != nil {
		return false, fmt.Errorf("closing migrator: %w", errors.Join(sourceErr, dbErr))
	}

	return applied, nil
}
