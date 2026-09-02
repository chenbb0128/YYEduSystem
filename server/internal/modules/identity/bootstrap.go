package identity

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// EnsureDefaultAdmin keeps backwards compatibility for local callers.
func EnsureDefaultAdmin(ctx context.Context, store UserStore) error {
	return EnsureConfiguredAdmin(ctx, store, "admin", "123456")
}

// EnsureConfiguredAdmin is only intended for local/bootstrap environments.
// Production deployments should create the first administrator through a
// controlled provisioning step and keep BootstrapAdminEnabled disabled.
func EnsureConfiguredAdmin(ctx context.Context, store UserStore, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "123456"
	}
	if _, err := store.FindUserByUsername(ctx, username); err == nil {
		return nil
	} else if !errors.Is(err, ErrUserNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = store.CreateUser(ctx, CreateUserParams{Username: username, PasswordHash: string(hash), Role: UserRoleAdmin, Nickname: "管理员", Status: UserStatusActive})
	return err
}
