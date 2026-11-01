package handlers

import (
	"app-finance/internal/middleware"
	"app-finance/internal/ratelimit"
	"app-finance/internal/usecases"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
	maxBodyBytes      = 1 << 20
)

const (
	maxAuthFailures   = 5
	maxEmailFailures  = 25
	authFailureWindow = 15 * time.Minute
)

type Handlers struct {
	usecases     *usecases.UseCase
	authLimiter  *ratelimit.Limiter
	emailLimiter *ratelimit.Limiter
	trustProxy   bool
}

func NewHandlers(usecases *usecases.UseCase, trustProxy bool) *Handlers {
	return &Handlers{
		usecases:     usecases,
		authLimiter:  ratelimit.New(maxAuthFailures, authFailureWindow),
		emailLimiter: ratelimit.New(maxEmailFailures, authFailureWindow),
		trustProxy:   trustProxy,
	}
}

func (h *Handlers) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		slog.Error("failed to write health response", "error", err)
	}
}

func (h *Handlers) Listen(ctx context.Context, port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	h.registerUserEndpoints(mux)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           middleware.LimitBody(middleware.Logging(mux), maxBodyBytes),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- server.ListenAndServe()
	}()

	slog.Info("Server is running", "port", port)

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down server: %w", err)
	}

	return nil
}
