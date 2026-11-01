package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"app-finance/internal/db"
	"app-finance/internal/handlers"
	"app-finance/internal/repositories"
	"app-finance/internal/usecases"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return errors.New("JWT_SECRET environment variable is not set")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://finance:finance@localhost:5432/finance?sslmode=disable"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := db.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		return fmt.Errorf("migrating database: %w", err)
	}

	repos := repositories.New(conn)
	useCase := usecases.New(repos, []byte(secret))

	h := handlers.NewHandlers(useCase)
	if err := h.Listen(ctx, 8080); err != nil {
		return fmt.Errorf("running server: %w", err)
	}

	slog.Info("server stopped gracefully")

	return nil
}
