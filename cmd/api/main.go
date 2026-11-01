package main

import (
	"context"
	"log/slog"
	"os"

	"app-finance/internal/db"
	"app-finance/internal/handlers"
	"app-finance/internal/repositories"
	"app-finance/internal/usecases"
)

func main() {

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		slog.Error("JWT_SECRET environment variable is not set")
		os.Exit(1)
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://finance:finance@localhost:5432/finance?sslmode=disable"
	}

	conn, err := db.Open(context.Background(), dsn)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := db.Migrate(conn); err != nil {
		slog.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}

	repos := repositories.New(conn)
	useCase := usecases.New(repos, []byte(secret))

	h := handlers.NewHandlers(useCase)
	err = h.Listen(8080)
	if err != nil {
		slog.Error("server stopped", "error", err)
		return
	}

}
