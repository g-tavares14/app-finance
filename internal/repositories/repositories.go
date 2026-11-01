package repositories

import (
	"context"
	"database/sql"

	"app-finance/internal/models"
	"app-finance/internal/repositories/users"

	"github.com/google/uuid"
)

type Repository struct {
	User interface {
		Add(ctx context.Context, newUser models.User) error
		EmailExists(ctx context.Context, email string) (bool, error)
		GetByEmail(ctx context.Context, email string) (models.User, error)
		GetByID(ctx context.Context, id uuid.UUID) (models.User, error)
		Delete(ctx context.Context, id uuid.UUID) error
		Update(ctx context.Context, id uuid.UUID, updatedUser models.UpdateUser) error
	}
}

func New(db *sql.DB) *Repository {
	return &Repository{
		User: users.New(db),
	}
}
