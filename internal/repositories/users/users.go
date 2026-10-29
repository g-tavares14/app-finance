package users

import "app-finance/internal/models"

type Users struct {
	users []models.User
}

func New() *Users {
	return &Users{
		users: make([]models.User, 0),
	}
}

func (u *Users) GetAll() ([]models.User, error) {
	return u.users, nil
}

func (u *Users) EmailExists(email string) bool {
	for _, user := range u.users {
		if user.Email == email {
			return true
		}
	}
	return false
}

func (u *Users) Add(newUser models.User) error {
	u.users = append(u.users, newUser)

	return nil
}
