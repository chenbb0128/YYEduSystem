package meal

import "time"

const (
	PlanStatusActive = "active"
	PlanStatusClosed = "closed"

	DietNoteChangeStatusPending  = "pending"
	DietNoteChangeStatusApproved = "approved"
	DietNoteChangeStatusRejected = "rejected"
)

type Plan struct {
	ID              uint64
	OrganizationID  uint64
	MealDate        time.Time
	MenuText        string
	PhotoURL        string
	AdjustmentNote  string
	CreatedByUserID *uint64
	CreatedByName   string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DietNote struct {
	ID              uint64
	OrganizationID  uint64
	StudentID       uint64
	Note            string
	UpdatedByUserID *uint64
	UpdatedByName   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// DietNoteChangeRequest records a parent-proposed change before it becomes
// part of the official student care profile. The current note is kept as a
// snapshot so staff can review what was in force when the request was made.
type DietNoteChangeRequest struct {
	ID               uint64
	OrganizationID   uint64
	StudentID        uint64
	ParentAccountID  uint64
	CurrentNote      string
	RequestedNote    string
	Status           string
	ReviewNote       string
	ReviewedByUserID *uint64
	ReviewedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type UpsertPlanParams struct {
	MealDate        time.Time
	MenuText        string
	PhotoURL        string
	AdjustmentNote  string
	CreatedByUserID *uint64
	CreatedByName   string
}

type CopyPlanParams struct {
	SourceDate      time.Time
	TargetDate      time.Time
	CreatedByUserID *uint64
	CreatedByName   string
}

type UpsertDietNoteParams struct {
	StudentID       uint64
	Note            string
	UpdatedByUserID *uint64
	UpdatedByName   string
}

type CreateDietNoteChangeRequestParams struct {
	StudentID       uint64
	ParentAccountID uint64
	RequestedNote   string
}

type ReviewDietNoteChangeRequestParams struct {
	ID               uint64
	Status           string
	ReviewNote       string
	ReviewedByUserID uint64
}
