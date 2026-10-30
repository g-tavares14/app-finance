package repositories

import (
	"database/sql"

	"app-finance/internal/models"
	"app-finance/internal/repositories/users"
)

type Repository struct {
	User interface {
		GetAll() ([]models.User, error)
		Add(newUser models.User) error
		EmailExists(email string) (bool, error)
	}
}

func New(db *sql.DB) *Repository {
	return &Repository{
		User: users.New(db),
	}
}
