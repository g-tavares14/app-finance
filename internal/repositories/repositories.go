package repositories

import (
	"app-finance/internal/models"
	"app-finance/internal/repositories/users"
)

type Repository struct {
	User interface {
		GetAll() ([]models.User, error)
		Add(newUser models.User) error
		EmailExists(email string) bool
	}
}

func New() *Repository {
	return &Repository{
		User: users.New(),
	}
}
