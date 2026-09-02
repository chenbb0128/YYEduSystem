package identity

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestEnsureConfiguredAdminUsesConfiguredCredentials(t *testing.T) {
	store := NewMemoryStore()
	if err := EnsureConfiguredAdmin(context.Background(), store, "operator", "strong-local-password"); err != nil {
		t.Fatalf("EnsureConfiguredAdmin() error = %v", err)
	}

	user, err := store.FindUserByUsername(context.Background(), "operator")
	if err != nil {
		t.Fatalf("FindUserByUsername() error = %v", err)
	}
	if user.Role != UserRoleAdmin {
		t.Fatalf("role = %q, want admin", user.Role)
	}
	if user.PasswordHash == "" {
		t.Fatal("configured admin password hash is empty")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("strong-local-password")); err != nil {
		t.Fatalf("configured password does not match: %v", err)
	}
}
