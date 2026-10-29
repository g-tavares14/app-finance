package handlers

import (
	"app-finance/internal/usecases"
	"fmt"
	"log/slog"
	"net/http"
)

type Handlers struct {
	usecases *usecases.UseCase
}

func NewHandlers(usecases *usecases.UseCase) *Handlers {
	return &Handlers{
		usecases: usecases,
	}
}

func (h *Handlers) Listen(port int) error {
	h.registerUserEndpoints()

	slog.Info("Server is running", "port", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
