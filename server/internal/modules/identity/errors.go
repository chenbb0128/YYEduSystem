package identity

import "errors"

var (
	ErrUserNotFound       = errors.New("identity: user not found")
	ErrUsernameTaken      = errors.New("identity: username taken")
	ErrInvalidCredentials = errors.New("identity: invalid credentials")
	ErrUserDisabled       = errors.New("identity: user disabled")
	ErrInvalidRole        = errors.New("identity: invalid role")
	ErrInvalidToken       = errors.New("identity: invalid token")
	ErrTokenExpired       = errors.New("identity: token expired")
	ErrWrongTokenType     = errors.New("identity: wrong token type")
)
