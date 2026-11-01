package usecases

import (
	"app-finance/internal/models"
	"app-finance/internal/repositories"
	userrepo "app-finance/internal/repositories/users"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

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

var ErrInvalidInput = errors.New("invalid input")

var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

type tokenClaims struct {
	jwt.RegisteredClaims
	PasswordVersion string `json:"pwv"`
}

func New(repos *repositories.Repository, jwtSecret []byte) *UseCase {
	return &UseCase{
		repos:     repos,
		jwtSecret: jwtSecret,
	}
}

func (u *UseCase) Add(user models.User) (uuid.UUID, error) {
	user.Email = NormalizeEmail(user.Email)
	user.FirstName = strings.TrimSpace(user.FirstName)
	user.LastName = strings.TrimSpace(user.LastName)

	if err := validateName(user.FirstName); err != nil {
		return uuid.Nil, err
	}
	if err := validateName(user.LastName); err != nil {
		return uuid.Nil, err
	}
	if err := validateEmail(user.Email); err != nil {
		return uuid.Nil, err
	}
	if err := validatePassword(user.Password); err != nil {
		return uuid.Nil, err
	}

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

	err = u.repos.User.Add(repoReq)
	if errors.Is(err, userrepo.ErrEmailTaken) {
		return uuid.Nil, ErrEmailAlreadyExists
	}
	if err != nil {
		return uuid.Nil, err
	}

	return repoReq.ID, nil
}

func (u *UseCase) Login(email, password string) (string, error) {
	user, err := u.repos.User.GetByEmail(NormalizeEmail(email))
	if errors.Is(err, userrepo.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}

	if err := checkPassword(user.Password, password); err != nil {
		return "", err
	}

	claims := tokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
		PasswordVersion: u.passwordVersion(user.Password),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(u.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return token, nil
}

func (u *UseCase) Authenticate(tokenString string) (models.User, error) {
	claims := &tokenClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) { return u.jwtSecret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return models.User{}, ErrInvalidToken
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return models.User{}, ErrInvalidToken
	}

	user, err := u.repos.User.GetByID(id)
	if errors.Is(err, userrepo.ErrNotFound) {
		return models.User{}, ErrInvalidToken
	}
	if err != nil {
		return models.User{}, fmt.Errorf("loading authenticated user: %w", err)
	}

	if !hmac.Equal([]byte(claims.PasswordVersion), []byte(u.passwordVersion(user.Password))) {
		return models.User{}, ErrInvalidToken
	}

	return user, nil
}

func (u *UseCase) passwordVersion(passwordHash string) string {
	mac := hmac.New(sha256.New, u.jwtSecret)
	mac.Write([]byte(passwordHash))

	return hex.EncodeToString(mac.Sum(nil)[:8])
}

func (u *UseCase) Delete(user models.User, password string) error {
	if err := checkPassword(user.Password, password); err != nil {
		return err
	}

	err := u.repos.User.Delete(user.ID)
	if errors.Is(err, userrepo.ErrNotFound) {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}

	return nil
}

func (u *UseCase) Update(user models.User, update models.UpdateUser) error {
	nothingToUpdate := update.FirstName == nil && update.LastName == nil && update.Email == nil && update.Password == nil
	if nothingToUpdate || isBlank(update.FirstName) || isBlank(update.LastName) || isBlank(update.Email) || isBlank(update.Password) {
		return ErrInvalidUpdate
	}
	if update.FirstName != nil {
		firstName := strings.TrimSpace(*update.FirstName)
		update.FirstName = &firstName
		if err := validateName(firstName); err != nil {
			return err
		}
	}
	if update.LastName != nil {
		lastName := strings.TrimSpace(*update.LastName)
		update.LastName = &lastName
		if err := validateName(lastName); err != nil {
			return err
		}
	}
	if update.Email != nil {
		email := NormalizeEmail(*update.Email)
		update.Email = &email
		if err := validateEmail(email); err != nil {
			return err
		}
	}
	if update.Password != nil {
		if err := validatePassword(*update.Password); err != nil {
			return err
		}
	}

	if update.Email != nil || update.Password != nil {
		if update.CurrentPassword == nil {
			return ErrInvalidCredentials
		}
		if err := checkPassword(user.Password, *update.CurrentPassword); err != nil {
			return err
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

	err := u.repos.User.Update(user.ID, changes)
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

func checkPassword(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("comparing password: %w", err)
	}

	return nil
}

func isBlank(s *string) bool {
	return s != nil && strings.TrimSpace(*s) == ""
}

const (
	minPasswordLength = 5
	maxPasswordBytes  = 72
	maxNameLength     = 100
	MaxEmailLength    = 254
)

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: name must not be blank", ErrInvalidInput)
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		return fmt.Errorf("%w: name must have at most %d characters", ErrInvalidInput, maxNameLength)
	}

	return nil
}

func validateEmail(email string) error {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > MaxEmailLength {
		return fmt.Errorf("%w: invalid email format", ErrInvalidInput)
	}

	return nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: password must have at least %d characters", ErrInvalidInput, minPasswordLength)
	}
	if len(password) > maxPasswordBytes {
		return fmt.Errorf("%w: password must have at most %d bytes", ErrInvalidInput, maxPasswordBytes)
	}

	return nil
}
