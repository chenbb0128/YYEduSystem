package identity

import (
	"context"
	"fmt"
)

type principalContextKey struct{}

const defaultOrganizationID uint64 = 1

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

// OrganizationIDFromContext returns the tenant carried by the authenticated
// principal. The fallback keeps public/local handler tests backwards
// compatible, while authenticated staff and parent requests always use the
// organization encoded in their token.
func OrganizationIDFromContext(ctx context.Context, fallback uint64) uint64 {
	if principal, ok := PrincipalFromContext(ctx); ok && principal.OrganizationID != 0 {
		return principal.OrganizationID
	}
	return fallback
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
