package handlers

import (
	"app-finance/internal/models"
	"app-finance/internal/usecases"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

func (h *Handlers) registerUserEndpoints() {
	http.HandleFunc("GET /users", h.getAll)
	http.HandleFunc("POST /users", h.addUser)
}

func (h *Handlers) getAll(w http.ResponseWriter, r *http.Request) {
	users, err := h.usecases.GetAll()
	if err != nil {
		slog.Error("failed to get users", "error", err)
		http.Error(w, "failed to get users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(users); err != nil {
		slog.Error("failed to encode users response", "error", err)
	}
}

func (h *Handlers) addUser(w http.ResponseWriter, r *http.Request) {
	var request models.CreateUser

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid user request", http.StatusBadRequest)
		return
	}

	id, err := h.usecases.Add(models.User{
		FirstName: request.FirstName,
		LastName:  request.LastName,
		Email:     request.Email,
		Password:  request.Password,
	})
	if err != nil {
		if errors.Is(err, usecases.ErrEmailAlreadyExists) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		slog.Error("failed to create user", "error", err)
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	slog.Info("user created", "user_id", id)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(models.CreateUserResponse{NewUserID: id}); err != nil {
		slog.Error("failed to encode create-user response", "error", err)
		return
	}
}
