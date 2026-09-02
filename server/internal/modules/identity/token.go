package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type PrincipalKind string

const (
	PrincipalKindUser   PrincipalKind = "user"
	PrincipalKindParent PrincipalKind = "parent"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type Principal struct {
	Kind           PrincipalKind
	SubjectID      uint64
	OrganizationID uint64
	Role           UserRole
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

type tokenClaims struct {
	SubjectID      uint64        `json:"sub"`
	OrganizationID uint64        `json:"org"`
	Role           UserRole      `json:"role"`
	Kind           PrincipalKind `json:"kind"`
	Type           TokenType     `json:"type"`
	IssuedAt       int64         `json:"iat"`
	ExpiresAt      int64         `json:"exp"`
	ID             string        `json:"jti"`
}

type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) (*TokenManager, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return nil, errors.New("identity: auth secret must contain at least 32 characters")
	}
	if accessTTL <= 0 || refreshTTL <= accessTTL {
		return nil, errors.New("identity: invalid token ttl")
	}
	return &TokenManager{secret: []byte(secret), accessTTL: accessTTL, refreshTTL: refreshTTL, now: time.Now}, nil
}

func (m *TokenManager) IssuePair(principal Principal) (TokenPair, error) {
	issuedAt := m.now().UTC()
	access, err := m.issue(principal, TokenTypeAccess, issuedAt, m.accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := m.issue(principal, TokenTypeRefresh, issuedAt, m.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresIn: int64(m.accessTTL / time.Second)}, nil
}

func (m *TokenManager) ParseAccess(token string) (Principal, error) {
	claims, err := m.parse(token, TokenTypeAccess)
	if err != nil {
		return Principal{}, err
	}
	return principalFromClaims(claims), nil
}

func (m *TokenManager) ParseRefresh(token string) (Principal, error) {
	claims, err := m.parse(token, TokenTypeRefresh)
	if err != nil {
		return Principal{}, err
	}
	return principalFromClaims(claims), nil
}

func (m *TokenManager) issue(principal Principal, tokenType TokenType, issuedAt time.Time, ttl time.Duration) (string, error) {
	if principal.SubjectID == 0 || principal.OrganizationID == 0 || principal.Kind == "" {
		return "", ErrInvalidToken
	}
	id, err := randomID()
	if err != nil {
		return "", err
	}
	claims := tokenClaims{
		SubjectID: principal.SubjectID, OrganizationID: principal.OrganizationID, Role: principal.Role,
		Kind: principal.Kind, Type: tokenType, IssuedAt: issuedAt.Unix(), ExpiresAt: issuedAt.Add(ttl).Unix(), ID: id,
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := m.sign(encodedPayload)
	return encodedPayload + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (m *TokenManager) parse(token string, expectedType TokenType) (tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return tokenClaims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenClaims{}, ErrInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(signature, m.sign(parts[0])) {
		return tokenClaims{}, ErrInvalidToken
	}
	var claims tokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return tokenClaims{}, ErrInvalidToken
	}
	if claims.Type != expectedType {
		return tokenClaims{}, ErrWrongTokenType
	}
	if claims.SubjectID == 0 || claims.OrganizationID == 0 || claims.Kind == "" || claims.ExpiresAt <= 0 {
		return tokenClaims{}, ErrInvalidToken
	}
	if m.now().UTC().Unix() >= claims.ExpiresAt {
		return tokenClaims{}, ErrTokenExpired
	}
	return claims, nil
}

func (m *TokenManager) sign(payload string) []byte {
	hash := hmac.New(sha256.New, m.secret)
	_, _ = hash.Write([]byte(payload))
	return hash.Sum(nil)
}

func principalFromClaims(claims tokenClaims) Principal {
	return Principal{Kind: claims.Kind, SubjectID: claims.SubjectID, OrganizationID: claims.OrganizationID, Role: claims.Role}
}

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
