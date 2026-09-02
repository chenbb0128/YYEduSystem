package assignment

import "time"

const (
	AssignmentStatusActive   = "active"
	AssignmentStatusDisabled = "disabled"
)

type TeacherClassAssignment struct {
	ID             uint64
	OrganizationID uint64
	TeacherUserID  uint64
	SchoolClassID  uint64
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateParams struct {
	TeacherUserID uint64
	SchoolClassID uint64
}

type SetStatusParams struct {
	ID     uint64
	Status string
}
