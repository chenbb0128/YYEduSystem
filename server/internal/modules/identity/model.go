package identity

import "time"

type UserStatus string

type UserRole string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"

	UserRolePlatformAdmin UserRole = "platform_admin"
	UserRoleAdmin         UserRole = "admin"
	UserRoleTeacher       UserRole = "teacher"
	UserRoleEditor        UserRole = "editor"
	UserRoleViewer        UserRole = "viewer"
)

type User struct {
	ID                 uint64
	OrganizationID     uint64
	OrganizationStatus string
	Username           string
	PasswordHash       string
	Role               UserRole
	Nickname           string
	Avatar             string
	Status             UserStatus
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateUserParams struct {
	OrganizationID uint64
	Username       string
	PasswordHash   string
	Role           UserRole
	Nickname       string
	Avatar         string
	Status         UserStatus
}

type UpdateUserParams struct {
	ID           uint64
	PasswordHash string
	Role         UserRole
	Nickname     string
	Avatar       string
	Status       UserStatus
}

type UpdateProfileParams struct {
	ID       uint64
	Nickname string
	Avatar   string
}

type SetUserStatusParams struct {
	ID     uint64
	Status UserStatus
}

func IsPlatformAdmin(role UserRole) bool {
	return role == UserRolePlatformAdmin
}

func IsOrganizationManager(role UserRole) bool {
	return role == UserRoleAdmin || role == UserRoleEditor
}
