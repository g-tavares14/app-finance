package handlers

import (
	"app-finance/internal/middleware"
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
	http.HandleFunc("POST /login", h.login)
	http.HandleFunc("DELETE /users/me", middleware.Auth(h.usecases)(h.deleteUser))
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

func (h *Handlers) login(w http.ResponseWriter, r *http.Request) {
	var request models.Login

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid login request", http.StatusBadRequest)
		return
	}

	token, err := h.usecases.Login(request.Email, request.Password)
	if err != nil {
		if errors.Is(err, usecases.ErrInvalidCredentials) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		slog.Error("failed to login user", "error", err)
		http.Error(w, "failed to login user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(models.LoginResponse{Token: token}); err != nil {
		slog.Error("failed to encode login response", "error", err)
	}
}

func (h *Handlers) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserID(r.Context())
	if !ok {
		slog.Error("user id missing from request context")
		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	var request models.DeleteUser

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid delete user request", http.StatusBadRequest)
		return
	}

	if err := h.usecases.Delete(id, request.Password); err != nil {
		if errors.Is(err, usecases.ErrInvalidCredentials) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if errors.Is(err, usecases.ErrUserNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		slog.Error("failed to delete user", "error", err)
		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	slog.Info("user deleted", "user_id", id)

	w.WriteHeader(http.StatusNoContent)
}
