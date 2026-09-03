package platformadmin

import (
	"context"
	"errors"
)

var (
	ErrNotFound        = errors.New("platformadmin: resource not found")
	ErrConflict        = errors.New("platformadmin: resource already exists")
	ErrInvalidInvite   = errors.New("platformadmin: invite code is invalid")
	ErrInviteExhausted = errors.New("platformadmin: invite code is exhausted")
	ErrInvalidStatus   = errors.New("platformadmin: invalid status")
)

type contextKey struct{}

func WithOrganizationID(ctx context.Context, organizationID uint64) context.Context {
	return context.WithValue(ctx, contextKey{}, organizationID)
}
