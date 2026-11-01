package usecases

import (
	"errors"
	"testing"

	"app-finance/internal/models"
	"app-finance/internal/repositories"
	userrepo "app-finance/internal/repositories/users"

	"github.com/google/uuid"
)

type fakeUsers struct {
	users      map[uuid.UUID]models.User
	getByIDErr error
}

func (f *fakeUsers) Add(u models.User) error {
	f.users[u.ID] = u
	return nil
}

func (f *fakeUsers) EmailExists(email string) (bool, error) {
	for _, u := range f.users {
		if u.Email == email {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeUsers) GetByEmail(email string) (models.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return models.User{}, userrepo.ErrNotFound
}

func (f *fakeUsers) GetByID(id uuid.UUID) (models.User, error) {
	if f.getByIDErr != nil {
		return models.User{}, f.getByIDErr
	}
	u, ok := f.users[id]
	if !ok {
		return models.User{}, userrepo.ErrNotFound
	}
	return u, nil
}

func (f *fakeUsers) Delete(id uuid.UUID) error {
	if _, ok := f.users[id]; !ok {
		return userrepo.ErrNotFound
	}
	delete(f.users, id)
	return nil
}

func (f *fakeUsers) Update(id uuid.UUID, c models.UpdateUser) error {
	u, ok := f.users[id]
	if !ok {
		return userrepo.ErrNotFound
	}
	if c.FirstName != nil {
		u.FirstName = *c.FirstName
	}
	if c.LastName != nil {
		u.LastName = *c.LastName
	}
	if c.Email != nil {
		u.Email = *c.Email
	}
	if c.Password != nil {
		u.Password = *c.Password
	}
	f.users[id] = u
	return nil
}

func newTestUseCase(t *testing.T) (*UseCase, *fakeUsers) {
	t.Helper()

	fake := &fakeUsers{users: make(map[uuid.UUID]models.User)}
	uc := New(&repositories.Repository{User: fake}, []byte("0123456789abcdef0123456789abcdef"))

	return uc, fake
}

func addUser(t *testing.T, uc *UseCase, email, password string) uuid.UUID {
	t.Helper()

	id, err := uc.Add(models.User{FirstName: "Ana", LastName: "Lima", Email: email, Password: password})
	if err != nil {
		t.Fatalf("add user: %v", err)
	}

	return id
}

func ptr(s string) *string { return &s }

func TestAddNormalizesAndValidates(t *testing.T) {
	uc, fake := newTestUseCase(t)

	id, err := uc.Add(models.User{FirstName: "  Ana ", LastName: " Lima", Email: "  Ana@Example.COM ", Password: "senha123"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	stored := fake.users[id]
	if stored.Email != "ana@example.com" || stored.FirstName != "Ana" || stored.LastName != "Lima" {
		t.Fatalf("stored = %+v, want normalized email and trimmed names", stored)
	}

	if _, err := uc.Add(models.User{FirstName: "B", LastName: "C", Email: "ANA@example.com", Password: "senha123"}); !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("case variant err = %v, want ErrEmailAlreadyExists", err)
	}
	if _, err := uc.Add(models.User{FirstName: "   ", LastName: "C", Email: "x@example.com", Password: "senha123"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank name err = %v, want ErrInvalidInput", err)
	}
}

func TestLoginHidesWhetherTheEmailExists(t *testing.T) {
	uc, _ := newTestUseCase(t)
	addUser(t, uc, "ana@example.com", "senha123")

	if _, err := uc.Login("ana@example.com", "errada"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := uc.Login("ninguem@example.com", "senha123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown email err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := uc.Login("ANA@example.com", "senha123"); err != nil {
		t.Fatalf("login with different case: %v", err)
	}
}

func TestUpdateRequiresCurrentPasswordForEmailAndPassword(t *testing.T) {
	uc, fake := newTestUseCase(t)
	id := addUser(t, uc, "ana@example.com", "senha123")
	user := fake.users[id]

	cases := map[string]models.UpdateUser{
		"missing current password": {Email: ptr("novo@example.com")},
		"wrong current password":   {Password: ptr("outrasenha9"), CurrentPassword: ptr("errada")},
	}
	for name, update := range cases {
		if err := uc.Update(user, update); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("%s: err = %v, want ErrInvalidCredentials", name, err)
		}
	}

	if err := uc.Update(user, models.UpdateUser{FirstName: ptr("Maria")}); err != nil {
		t.Fatalf("name-only update: %v", err)
	}

	if err := uc.Update(user, models.UpdateUser{Email: ptr("Novo@Example.com"), CurrentPassword: ptr("senha123")}); err != nil {
		t.Fatalf("email update: %v", err)
	}
	if got := fake.users[id].Email; got != "novo@example.com" {
		t.Fatalf("email = %q, want normalized new email", got)
	}
}

func TestAuthenticateRejectsTokenAfterPasswordChange(t *testing.T) {
	uc, _ := newTestUseCase(t)
	addUser(t, uc, "ana@example.com", "senha123")

	token, err := uc.Login("ana@example.com", "senha123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	user, err := uc.Authenticate(token)
	if err != nil {
		t.Fatalf("authenticate fresh token: %v", err)
	}

	if err := uc.Update(user, models.UpdateUser{Password: ptr("outrasenha9"), CurrentPassword: ptr("senha123")}); err != nil {
		t.Fatalf("change password: %v", err)
	}

	if _, err := uc.Authenticate(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old token err = %v, want ErrInvalidToken", err)
	}
}

func TestAuthenticateRejectsDeletedUser(t *testing.T) {
	uc, _ := newTestUseCase(t)
	addUser(t, uc, "ana@example.com", "senha123")

	token, _ := uc.Login("ana@example.com", "senha123")
	user, err := uc.Authenticate(token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	if err := uc.Delete(user, "senha123"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := uc.Authenticate(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token of deleted user err = %v, want ErrInvalidToken", err)
	}
}

func TestAuthenticateDatabaseFailureIsNotInvalidToken(t *testing.T) {
	uc, fake := newTestUseCase(t)
	addUser(t, uc, "ana@example.com", "senha123")
	token, _ := uc.Login("ana@example.com", "senha123")

	fake.getByIDErr = errors.New("connection refused")

	_, err := uc.Authenticate(token)
	if err == nil || errors.Is(err, ErrInvalidToken) {
		t.Fatalf("err = %v, want a non-ErrInvalidToken failure", err)
	}
}
