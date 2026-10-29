package main

import (
	"app-finance/internal/handlers"
	"app-finance/internal/repositories"
	"app-finance/internal/usecases"
)

func main() {

	repos := repositories.New()
	useCase := usecases.New(repos)

	h := handlers.NewHandlers(useCase)
	err := h.Listen(8080)
	if err != nil {
		return
	}

}
