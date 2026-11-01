package repositories

import (
	"database/sql"

	"app-finance/internal/models"
	"app-finance/internal/repositories/users"

	"github.com/google/uuid"
)

type Repository struct {
	User interface {
		Add(newUser models.User) error
		EmailExists(email string) (bool, error)
		GetByEmail(email string) (models.User, error)
		GetByID(id uuid.UUID) (models.User, error)
		Delete(id uuid.UUID) error
		Update(id uuid.UUID, updatedUser models.UpdateUser) error
	}
}

func New(db *sql.DB) *Repository {
	return &Repository{
		User: users.New(db),
	}
}
