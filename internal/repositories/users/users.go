package users

import (
	"database/sql"
	"errors"
	"fmt"

	"app-finance/internal/models"
)

// ErrNotFound isola o use case do sql.ErrNoRows, que é detalhe do banco.
var ErrNotFound = errors.New("user not found")

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
	if err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}

	return nil
}

func (u *Users) GetByEmail(email string) (models.User, error) {
	var user models.User
	// o hash da senha é lido para o campo Password; o json:"-" impede que vaze nas respostas
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
