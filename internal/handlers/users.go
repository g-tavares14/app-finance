package handlers

import (
	"encoding/json"
	"errors"
	"github.com/g-tavares14/go-auth-api/internal/middleware"
	"github.com/g-tavares14/go-auth-api/internal/models"
	"github.com/g-tavares14/go-auth-api/internal/ratelimit"
	"github.com/g-tavares14/go-auth-api/internal/usecases"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

type attemptKey struct {
	limiter *ratelimit.Limiter
	key     string
}

func (h *Handlers) registerUserEndpoints(mux *http.ServeMux) {
	auth := middleware.Auth(h.usecases, usecases.ErrInvalidToken)

	mux.HandleFunc("POST /users", h.addUser)
	mux.HandleFunc("POST /login", h.login)
	mux.HandleFunc("GET /users/me", auth(h.getMe))
	mux.HandleFunc("DELETE /users/me", auth(h.deleteUser))
	mux.HandleFunc("PATCH /users/me", auth(h.updateUser))
}

func decodeFailed(w http.ResponseWriter, err error, message string) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, message, http.StatusBadRequest)
}

func writeError(w http.ResponseWriter, err error, message string) {
	switch {
	case errors.Is(err, usecases.ErrInvalidInput), errors.Is(err, usecases.ErrInvalidUpdate):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, usecases.ErrInvalidCredentials):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, usecases.ErrUserNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, usecases.ErrEmailAlreadyExists):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		slog.Error(message, "error", err)
		http.Error(w, message, http.StatusInternalServerError)
	}
}

func (h *Handlers) getMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.User(r.Context())
	if !ok {
		slog.Error("user missing from request context")
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
		decodeFailed(w, err, "invalid user request")
		return
	}

	id, err := h.usecases.Add(r.Context(), models.User{
		FirstName: request.FirstName,
		LastName:  request.LastName,
		Email:     request.Email,
		Password:  request.Password,
	})
	if err != nil {
		writeError(w, err, "failed to create user")
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
		decodeFailed(w, err, "invalid login request")
		return
	}

	done, ok := takeAttempts(w, h.loginKeys(r, request.Email))
	if !ok {
		return
	}

	token, err := h.usecases.Login(r.Context(), request.Email, request.Password)
	done(err)
	if err != nil {
		writeError(w, err, "failed to login user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(models.LoginResponse{Token: token}); err != nil {
		slog.Error("failed to encode login response", "error", err)
	}
}

func takeAttempts(w http.ResponseWriter, keys []attemptKey) (func(err error), bool) {
	for i, k := range keys {
		wait, ok := k.limiter.Take(k.key)
		if !ok {
			release(keys[:i])
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			http.Error(w, "too many attempts", http.StatusTooManyRequests)
			return nil, false
		}
	}

	return func(err error) {
		if !errors.Is(err, usecases.ErrInvalidCredentials) {
			release(keys)
		}
	}, true
}

func release(keys []attemptKey) {
	for _, k := range keys {
		k.limiter.Release(k.key)
	}
}

func (h *Handlers) userKeys(user models.User) []attemptKey {
	return []attemptKey{{h.authLimiter, "user:" + user.ID.String()}}
}

func (h *Handlers) loginKeys(r *http.Request, email string) []attemptKey {
	email = usecases.NormalizeEmail(email)
	if len(email) > usecases.MaxEmailLength {
		email = email[:usecases.MaxEmailLength]
	}

	ip, ok := h.clientIP(r)
	if !ok {
		return []attemptKey{{h.authLimiter, "email:" + email}}
	}

	return []attemptKey{
		{h.emailLimiter, "email:" + email},
		{h.authLimiter, "email+ip:" + email + "|" + ip},
		{h.authLimiter, "ip:" + ip},
	}
}

func (h *Handlers) clientIP(r *http.Request) (string, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if h.trustProxy {
		if forwarded := strings.Join(r.Header.Values("X-Forwarded-For"), ","); forwarded != "" {
			host = strings.TrimSpace(forwarded[strings.LastIndexByte(forwarded, ',')+1:])
		}
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}
	addr = addr.Unmap()
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() {
		return "", false
	}
	if addr.Is6() {
		return netip.PrefixFrom(addr, 64).Masked().String(), true
	}

	return addr.String(), true
}

func (h *Handlers) deleteUser(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.User(r.Context())
	if !ok {
		slog.Error("user missing from request context")
		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	var request models.DeleteUser

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		decodeFailed(w, err, "invalid delete user request")
		return
	}

	done, ok := takeAttempts(w, h.userKeys(user))
	if !ok {
		return
	}

	err := h.usecases.Delete(r.Context(), user, request.Password)
	done(err)
	if err != nil {
		writeError(w, err, "failed to delete user")
		return
	}

	slog.Info("user deleted", "user_id", user.ID)

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) updateUser(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.User(r.Context())
	if !ok {
		slog.Error("user missing from request context")
		http.Error(w, "failed to update user", http.StatusInternalServerError)
		return
	}

	var request models.UpdateUser
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		decodeFailed(w, err, "invalid update user request")
		return
	}

	done := func(error) {}
	if request.ChangesCredentials() {
		done, ok = takeAttempts(w, h.userKeys(user))
		if !ok {
			return
		}
	}

	err := h.usecases.Update(r.Context(), user, request)
	done(err)
	if err != nil {
		writeError(w, err, "failed to update user")
		return
	}

	slog.Info("user updated", "user_id", user.ID)

	w.WriteHeader(http.StatusNoContent)
}
