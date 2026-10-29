package handlers

import (
	"app-finance/internal/usecases"
	"fmt"
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
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
