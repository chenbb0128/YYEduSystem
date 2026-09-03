package platformadmin

import (
	"context"
	"time"
)

const (
	OrganizationStatusPending  = "pending"
	OrganizationStatusActive   = "active"
	OrganizationStatusDisabled = "disabled"

	InviteStatusActive    = "active"
	InviteStatusRevoked   = "revoked"
	InviteStatusExhausted = "exhausted"

	RegistrationStatusPending  = "pending"
	RegistrationStatusApproved = "approved"
	RegistrationStatusRejected = "rejected"
)

type Organization struct {
	ID              uint64
	Name            string
	Slug            string
	ContactName     string
	ContactPhone    string
	AuthorizedUntil *time.Time
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Invite struct {
	ID          uint64
	CodeHint    string
	MaxUses     uint32
	UsedCount   uint32
	ExpiresAt   *time.Time
	Status      string
	Note        string
	CreatedByID uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Registration struct {
	ID                uint64
	InviteID          uint64
	OrganizationID    *uint64
	OrganizationName  string
	Slug              string
	ContactName       string
	ContactPhone      string
	AdminUsername     string
	AdminPasswordHash string
	Status            string
	ReviewNote        string
	ReviewedByID      *uint64
	ReviewedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateInviteParams struct {
	MaxUses     uint32
	ExpiresAt   *time.Time
	Note        string
	CreatedByID uint64
}

type CreateOrganizationParams struct {
	Name            string
	Slug            string
	ContactName     string
	ContactPhone    string
	AuthorizedUntil *time.Time
	Status          string
}

type CreateRegistrationParams struct {
	InviteCode        string
	OrganizationName  string
	Slug              string
	ContactName       string
	ContactPhone      string
	AdminUsername     string
	AdminPasswordHash string
}

type SetRegistrationStatusParams struct {
	ID             uint64
	Status         string
	OrganizationID *uint64
	ReviewNote     string
	ReviewedByID   uint64
}

type Store interface {
	ListOrganizations(ctx context.Context) ([]Organization, error)
	CreateOrganization(ctx context.Context, params CreateOrganizationParams) (Organization, error)
	SetOrganizationStatus(ctx context.Context, id uint64, status string) error
	ListInvites(ctx context.Context, status string) ([]Invite, error)
	CreateInvite(ctx context.Context, params CreateInviteParams) (Invite, string, error)
	RevokeInvite(ctx context.Context, id uint64) error
	CreateRegistration(ctx context.Context, params CreateRegistrationParams) (Registration, error)
	GetRegistration(ctx context.Context, id uint64) (Registration, error)
	ListRegistrations(ctx context.Context, status string) ([]Registration, error)
	SetRegistrationStatus(ctx context.Context, params SetRegistrationStatusParams) error
}
