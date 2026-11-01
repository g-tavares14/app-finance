package usecases

import (
	"app-finance/internal/models"
	"app-finance/internal/repositories"
	userrepo "app-finance/internal/repositories/users"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UseCase struct {
	repos     *repositories.Repository
	jwtSecret []byte
}

var ErrEmailAlreadyExists = errors.New("email already exists")

var ErrInvalidCredentials = errors.New("invalid credentials")

var ErrUserNotFound = errors.New("user not found")

var ErrInvalidToken = errors.New("invalid token")

var ErrInvalidUpdate = errors.New("invalid update data")

func New(repos *repositories.Repository, jwtSecret []byte) *UseCase {
	return &UseCase{
		repos:     repos,
		jwtSecret: jwtSecret,
	}
}

func (u *UseCase) GetAll() ([]models.User, error) {
	users, err := u.repos.User.GetAll()

	if err != nil {
		return nil, err
	}

	return users, nil
}

func (u *UseCase) Add(user models.User) (uuid.UUID, error) {
	userExist, err := u.repos.User.EmailExists(user.Email)
	if err != nil {
		return uuid.Nil, err
	}

	if userExist {
		return uuid.Nil, ErrEmailAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hashing password: %w", err)
	}

	repoReq := models.User{
		ID:        uuid.New(),
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Password:  string(hash),
	}

	if err := u.repos.User.Add(repoReq); err != nil {
		return uuid.Nil, err
	}

	return repoReq.ID, nil
}

func (u *UseCase) Login(email, password string) (string, error) {
	user, err := u.repos.User.GetByEmail(email)
	if errors.Is(err, userrepo.ErrNotFound) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("comparing password: %w", err)
	}

	claims := jwt.RegisteredClaims{
		Subject:   user.ID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(u.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return token, nil
}

func (u *UseCase) Authenticate(tokenString string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) { return u.jwtSecret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return uuid.Nil, ErrInvalidToken
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}

	return id, nil
}

func (u *UseCase) Delete(id uuid.UUID, password string) error {
	user, err := u.repos.User.GetByID(id)
	if errors.Is(err, userrepo.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("comparing password: %w", err)
	}

	err = u.repos.User.Delete(id)
	if errors.Is(err, userrepo.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}

	return nil
}

func (u *UseCase) Update(id uuid.UUID, update models.UpdateUser) error {
	nothingToUpdate := update.FirstName == nil && update.LastName == nil && update.Email == nil && update.Password == nil
	if nothingToUpdate || isBlank(update.FirstName) || isBlank(update.LastName) || isBlank(update.Email) || isBlank(update.Password) {
		return ErrInvalidUpdate
	}

	user, err := u.repos.User.GetByID(id)
	if errors.Is(err, userrepo.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}

	if update.Email != nil || update.Password != nil {
		if update.CurrentPassword == nil {
			return ErrInvalidCredentials
		}
		err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(*update.CurrentPassword))
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrInvalidCredentials
		}
		if err != nil {
			return fmt.Errorf("comparing password: %w", err)
		}
	}

	if update.Email != nil && *update.Email != user.Email {
		emailExists, err := u.repos.User.EmailExists(*update.Email)
		if err != nil {
			return err
		}
		if emailExists {
			return ErrEmailAlreadyExists
		}
	}

	changes := models.UpdateUser{
		FirstName: update.FirstName,
		LastName:  update.LastName,
		Email:     update.Email,
	}
	if update.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*update.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hashing password: %w", err)
		}
		hashed := string(hash)
		changes.Password = &hashed
	}

	err = u.repos.User.Update(id, changes)
	if errors.Is(err, userrepo.ErrNotFound) {
		return ErrUserNotFound
	}
	if errors.Is(err, userrepo.ErrEmailTaken) {
		return ErrEmailAlreadyExists
	}
	if err != nil {
		return err
	}

	return nil
}

func isBlank(s *string) bool {
	return s != nil && strings.TrimSpace(*s) == ""
}
