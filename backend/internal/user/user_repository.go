package user

import (
	"context"
	"errors"

	"github.com/sanbei101/go-chat/internal/store"
)

// User repository injected with database connection object
// Takes a User struct and updates the database

type repository struct {
	queries *store.Queries
}

func NewRepository(queries *store.Queries) Repository {
	return &repository{queries: queries}
}

func (r *repository) CreateUser(ctx context.Context, user *User) (*User, error) {
	exists, err := r.queries.UserExistsByEmail(ctx, user.Email)
	if err != nil {
		return &User{}, err
	}
	if exists {
		return &User{}, errors.New("User already exists!")
	}

	createdUser, err := r.queries.CreateUser(ctx, store.CreateUserParams{
		Username: user.Username,
		Email:    user.Email,
		Password: user.Password,
	})
	if err != nil {
		return &User{}, err
	}

	return &User{
		ID:       createdUser.ID,
		Username: createdUser.Username,
		Email:    createdUser.Email,
		Password: createdUser.Password,
	}, nil
}

func (r *repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return &User{}, err
	}

	return &User{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
		Password: u.Password,
	}, nil
}
