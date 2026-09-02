package media

import "time"

const (
	AssetStatusActive  = "active"
	AssetStatusDeleted = "deleted"
)

type Asset struct {
	ID              uint64
	OrganizationID  uint64
	ObjectKey       string
	ResourceType    string
	ResourceID      *uint64
	OwnerType       string
	OwnerID         *uint64
	ContentType     string
	SizeBytes       int64
	SHA256          string
	Status          string
	RetentionUntil  *time.Time
	CreatedByUserID *uint64
	CreatedAt       time.Time
	DeletedAt       *time.Time
}

type CreateParams struct {
	ObjectKey       string
	ResourceType    string
	ResourceID      *uint64
	OwnerType       string
	OwnerID         *uint64
	ContentType     string
	SizeBytes       int64
	SHA256          string
	RetentionUntil  *time.Time
	CreatedByUserID *uint64
}
