package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/g-tavares14/go-auth-api/internal/db"
	"github.com/g-tavares14/go-auth-api/internal/handlers"
	"github.com/g-tavares14/go-auth-api/internal/repositories"
	"github.com/g-tavares14/go-auth-api/internal/usecases"
)

const minSecretBytes = 32

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < minSecretBytes {
		return fmt.Errorf("JWT_SECRET must be set and have at least %d bytes", minSecretBytes)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://finance:finance@localhost:5432/finance?sslmode=disable"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	context.AfterFunc(ctx, stop)

	conn, err := db.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close()

	slog.Info("database connection established")

	applied, err := db.Migrate(ctx, dsn)
	if err != nil {
		return fmt.Errorf("migrating database: %w", err)
	}
	if applied {
		slog.Info("database migrations applied")
	} else {
		slog.Info("database schema is up to date")
	}

	repos := repositories.New(conn)
	useCase := usecases.New(repos, []byte(secret))

	h := handlers.NewHandlers(useCase, os.Getenv("TRUST_PROXY") == "true")
	if err := h.Listen(ctx, 8080); err != nil {
		return fmt.Errorf("running server: %w", err)
	}

	slog.Info("server stopped gracefully")

	return nil
}
