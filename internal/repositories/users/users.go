package users

import (
	"database/sql"
	"fmt"

	"app-finance/internal/models"
)

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
		"INSERT INTO users (id, first_name, last_name, email, password) VALUES ($1, $2, $3, $4, $5)",
		newUser.ID, newUser.FirstName, newUser.LastName, newUser.Email, newUser.Password,
	)
	if err != nil {
		return fmt.Errorf("inserting user: %w", err)
	}

	return nil
}
