package homework

import "time"

const (
	TaskStatusActive    = "active"
	TaskStatusCancelled = "cancelled"

	StudentStatusPending      = "pending"
	StudentStatusCompleted    = "completed"
	StudentStatusIncomplete   = "incomplete"
	StudentStatusNotSubmitted = "not_submitted"
)

type Task struct {
	ID              uint64
	OrganizationID  uint64
	HomeworkDate    time.Time
	SchoolID        uint64
	SchoolClassID   uint64
	Subject         string
	Content         string
	AttachmentURLs  []string
	CreatedByUserID *uint64
	CreatorName     string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type TaskStudent struct {
	ID               uint64
	OrganizationID   uint64
	TaskID           uint64
	StudentID        uint64
	StudentName      string
	Status           string
	CorrectionNote   string
	ReviewedByUserID *uint64
	ReviewedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type StudentHomework struct {
	Task
	TaskStudent
}

type StudentRef struct {
	ID   uint64
	Name string
}

type CreateTaskParams struct {
	HomeworkDate    time.Time
	SchoolID        uint64
	SchoolClassID   uint64
	Subject         string
	Content         string
	AttachmentURLs  []string
	CreatedByUserID *uint64
	CreatorName     string
}

type ReviewStudentParams struct {
	TaskID           uint64
	StudentID        uint64
	Status           string
	CorrectionNote   string
	ReviewedByUserID *uint64
}

type BulkReviewItem struct {
	StudentID      uint64
	Status         string
	CorrectionNote string
}

type BulkReviewStudentsParams struct {
	TaskID           uint64
	Items            []BulkReviewItem
	ReviewedByUserID *uint64
}
