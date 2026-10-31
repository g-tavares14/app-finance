package usecases

import (
	"app-finance/internal/models"
	"app-finance/internal/repositories"
	userrepo "app-finance/internal/repositories/users"
	"errors"
	"fmt"
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

// ErrInvalidCredentials cobre e-mail inexistente e senha errada de propósito,
// para a resposta não revelar quais e-mails estão cadastrados.
var ErrInvalidCredentials = errors.New("invalid credentials")

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

	// user.Password guarda o hash bcrypt vindo do banco
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("comparing password: %w", err)
	}

	// o token só é gerado depois que e-mail e senha foram confirmados
	claims := jwt.RegisteredClaims{
		Subject:   user.ID.String(), // o ID do usuário vai aqui
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(u.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return token, nil
}
