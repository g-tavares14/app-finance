package usecases

import (
	"app-finance/internal/models"
	"app-finance/internal/repositories"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UseCase struct {
	repos *repositories.Repository
}

var ErrEmailAlreadyExists = errors.New("email already exists")

func New(repos *repositories.Repository) *UseCase {
	return &UseCase{
		repos: repos,
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
