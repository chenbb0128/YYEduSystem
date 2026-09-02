package summary

import "time"

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	StatusClosed    = "closed"
	StatusWithdrawn = "withdrawn"
)

type DailySummary struct {
	ID               uint64
	OrganizationID   uint64
	SummaryDate      time.Time
	Content          string
	ChildUpdates     map[uint64]string
	Status           string
	Version          uint32
	WithdrawnAt      *time.Time
	WithdrawalReason string
	CorrectionReason string
	CreatedByUserID  *uint64
	CreatedByName    string
	GeneratedAt      *time.Time
	PublishedAt      *time.Time
	ClosedAt         *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type GenerateParams struct {
	SummaryDate     time.Time
	Content         string
	ChildUpdates    map[uint64]string
	CreatedByUserID *uint64
	CreatedByName   string
}

type UpdateParams struct {
	ID           uint64
	Content      string
	ChildUpdates map[uint64]string
}

type CorrectParams struct {
	ID              uint64
	Content         string
	ChildUpdates    map[uint64]string
	Reason          string
	CreatedByUserID *uint64
	CreatedByName   string
}

type Version struct {
	ID              uint64
	SummaryID       uint64
	Version         uint32
	Action          string
	Content         string
	ChildUpdates    map[uint64]string
	Reason          string
	CreatedByUserID *uint64
	CreatedByName   string
	CreatedAt       time.Time
}
