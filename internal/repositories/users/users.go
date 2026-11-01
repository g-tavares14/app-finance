package users

import (
	"database/sql"
	"errors"
	"fmt"

	"app-finance/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrNotFound = errors.New("user not found")

var ErrEmailTaken = errors.New("email already taken")

const uniqueViolationCode = "23505"

type Users struct {
	db *sql.DB
}

func New(db *sql.DB) *Users {
	return &Users{
		db: db,
	}
}

func (u *Users) GetAll() ([]models.User, error) {
	rows, err := u.db.Query("SELECT id, first_name, last_name, email FROM users")
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating users: %w", err)
	}

	return users, nil
}

func (u *Users) EmailExists(email string) (bool, error) {
	var exists bool
	err := u.db.QueryRow("SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking if email exists: %w", err)
	}

	return exists, nil
}

func (u *Users) Add(newUser models.User) error {
	_, err := u.db.Exec(
		"INSERT INTO users (id, first_name, last_name, email, password_hash) VALUES ($1, $2, $3, $4, $5)",
		newUser.ID, newUser.FirstName, newUser.LastName, newUser.Email, newUser.Password,
	)
	if isUniqueViolation(err) {
		return ErrEmailTaken
	}
	if err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}

func (u *Users) GetByEmail(email string) (models.User, error) {
	var user models.User
	err := u.db.QueryRow(
		"SELECT id, first_name, last_name, email, password_hash FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email, &user.Password)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("getting user by email: %w", err)
	}

	return user, nil
}

func (u *Users) GetByID(id uuid.UUID) (models.User, error) {
	var user models.User
	err := u.db.QueryRow(
		"SELECT id, first_name, last_name, email, password_hash FROM users WHERE id = $1",
		id,
	).Scan(&user.ID, &user.FirstName, &user.LastName, &user.Email, &user.Password)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("getting user by id: %w", err)
	}

	return user, nil
}

func (u *Users) Delete(id uuid.UUID) error {
	result, err := u.db.Exec("DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (u *Users) Update(id uuid.UUID, updateData models.UpdateUser) error {
	query := "UPDATE users SET first_name = COALESCE($1, first_name), last_name = COALESCE($2, last_name), email = COALESCE($3, email), password_hash = COALESCE($4, password_hash) WHERE id = $5"
	result, err := u.db.Exec(query, updateData.FirstName, updateData.LastName, updateData.Email, updateData.Password, id)

	if isUniqueViolation(err) {
		return ErrEmailTaken
	}
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking updated rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
