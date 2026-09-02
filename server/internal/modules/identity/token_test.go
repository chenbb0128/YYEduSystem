package identity

import (
	"errors"
	"testing"
	"time"
)

func TestTokenManagerIssuesAndValidatesPair(t *testing.T) {
	manager, err := NewTokenManager("01234567890123456789012345678901", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := manager.IssuePair(Principal{Kind: PrincipalKindUser, SubjectID: 7, OrganizationID: 1, Role: UserRoleTeacher})
	if err != nil {
		t.Fatal(err)
	}
	access, err := manager.ParseAccess(pair.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if access.SubjectID != 7 || access.Role != UserRoleTeacher {
		t.Fatalf("access principal = %+v", access)
	}
	refresh, err := manager.ParseRefresh(pair.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if refresh.Kind != PrincipalKindUser {
		t.Fatalf("refresh principal = %+v", refresh)
	}
	if _, err := manager.ParseAccess(pair.RefreshToken); !errors.Is(err, ErrWrongTokenType) {
		t.Fatalf("wrong token type error = %v", err)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	manager, err := NewTokenManager("01234567890123456789012345678901", time.Hour, 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	manager.now = func() time.Time { return now }
	pair, err := manager.IssuePair(Principal{Kind: PrincipalKindParent, SubjectID: 3, OrganizationID: 1})
	if err != nil {
		t.Fatal(err)
	}
	manager.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := manager.ParseAccess(pair.AccessToken); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired token error = %v", err)
	}
}
