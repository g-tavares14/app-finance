package handlers

import (
	"app-finance/internal/middleware"
	"app-finance/internal/models"
	"app-finance/internal/usecases"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

const maxEmailKeyLength = 254

func (h *Handlers) registerUserEndpoints(mux *http.ServeMux) {
	mux.HandleFunc("POST /users", h.addUser)
	mux.HandleFunc("POST /login", h.login)
	mux.HandleFunc("GET /users/me", middleware.Auth(h.usecases)(h.getMe))
	mux.HandleFunc("DELETE /users/me", middleware.Auth(h.usecases)(h.deleteUser))
	mux.HandleFunc("PATCH /users/me", middleware.Auth(h.usecases)(h.updateUser))
}

func (h *Handlers) getMe(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserID(r.Context())
	if !ok {
		slog.Error("user id missing from request context")
		http.Error(w, "failed to get user", http.StatusInternalServerError)
		return
	}

	user, err := h.usecases.GetByID(id)
	if err != nil {
		if errors.Is(err, usecases.ErrUserNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		slog.Error("failed to get user", "error", err)
		http.Error(w, "failed to get user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		slog.Error("failed to encode user response", "error", err)
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
		if errors.Is(err, usecases.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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

	done, ok := h.takeAttempts(w, loginKeys(r, request.Email))
	if !ok {
		return
	}

	token, err := h.usecases.Login(request.Email, request.Password)
	done(errors.Is(err, usecases.ErrInvalidCredentials))
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

func (h *Handlers) takeAttempts(w http.ResponseWriter, keys []string) (func(failed bool), bool) {
	taken := make([]string, 0, len(keys))
	release := func() {
		for _, key := range taken {
			h.authLimiter.Release(key)
		}
	}

	for _, key := range keys {
		wait, ok := h.authLimiter.Take(key)
		if !ok {
			release()
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			http.Error(w, "too many attempts", http.StatusTooManyRequests)
			return nil, false
		}
		taken = append(taken, key)
	}

	return func(failed bool) {
		if !failed {
			release()
		}
	}, true
}

func loginKeys(r *http.Request, email string) []string {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > maxEmailKeyLength {
		email = email[:maxEmailKeyLength]
	}
	keys := []string{"email:" + email}

	if ip, ok := clientIP(r); ok {
		keys = append(keys, "ip:"+ip)
	}

	return keys
}

func clientIP(r *http.Request) (string, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}
	addr = addr.Unmap()
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() {
		return "", false
	}

	return addr.String(), true
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

	done, ok := h.takeAttempts(w, []string{"user:" + id.String()})
	if !ok {
		return
	}

	err := h.usecases.Delete(id, request.Password)
	done(errors.Is(err, usecases.ErrInvalidCredentials))
	if err != nil {
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

func (h *Handlers) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserID(r.Context())
	if !ok {
		slog.Error("user id missing from request context")
		http.Error(w, "failed to update user", http.StatusInternalServerError)
		return
	}

	var request models.UpdateUser
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid update user request", http.StatusBadRequest)
		return
	}

	done, ok := h.takeAttempts(w, []string{"user:" + id.String()})
	if !ok {
		return
	}

	err := h.usecases.Update(id, request)
	done(errors.Is(err, usecases.ErrInvalidCredentials))
	if err != nil {
		if errors.Is(err, usecases.ErrInvalidUpdate) || errors.Is(err, usecases.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, usecases.ErrInvalidCredentials) {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if errors.Is(err, usecases.ErrEmailAlreadyExists) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if errors.Is(err, usecases.ErrUserNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		slog.Error("failed to update user", "error", err)
		http.Error(w, "failed to update user", http.StatusInternalServerError)
		return
	}

	slog.Info("user updated", "user_id", id)

	w.WriteHeader(http.StatusNoContent)
}
