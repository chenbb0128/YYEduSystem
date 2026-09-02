package identity

import (
	"context"
	"fmt"
)

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

func RequireUser(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok || principal.Kind != PrincipalKindUser {
		return Principal{}, fmt.Errorf("identity: user principal required")
	}
	return principal, nil
}

func RequireParent(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok || principal.Kind != PrincipalKindParent {
		return Principal{}, fmt.Errorf("identity: parent principal required")
	}
	return principal, nil
}
