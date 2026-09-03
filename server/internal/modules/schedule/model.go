package schedule

import "time"

const (
	PickupModeSchool = "school_pickup"
	PickupModeSelf   = "self_arrival"
)

type PickupSchedule struct {
	ID                 uint64
	OrganizationID     uint64
	SchoolID           uint64
	SchoolClassID      uint64
	CareClassID        *uint64
	Weekday            time.Weekday
	PickupMode         string
	TeacherUserID      *uint64
	TeacherName        string
	ExpectedPickupTime string
	EffectiveFrom      time.Time
	EffectiveTo        *time.Time
	Enabled            bool
	Notes              string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateParams struct {
	SchoolID           uint64
	SchoolClassID      uint64
	CareClassID        *uint64
	Weekday            time.Weekday
	PickupMode         string
	TeacherUserID      *uint64
	TeacherName        string
	ExpectedPickupTime string
	EffectiveFrom      time.Time
	EffectiveTo        *time.Time
	Enabled            bool
	Notes              string
}

type UpdateParams struct {
	ID uint64
	CreateParams
}

type GenerationResult struct {
	Date                time.Time         `json:"date"`
	CreatedOperationIDs []uint64          `json:"created_operation_ids"`
	SkippedScheduleIDs  []uint64          `json:"skipped_schedule_ids"`
	SkippedReasons      map[uint64]string `json:"skipped_reasons"`
}
