package pickup

import (
	"context"
	"time"
)

const (
	OperationStatusDraft     = "draft"
	OperationStatusConfirmed = "confirmed"
	OperationStatusStarted   = "started"
	OperationStatusFinished  = "finished"
	OperationStatusCancelled = "cancelled"

	MemberStatusPlanned        = "planned"
	MemberStatusPickedUp       = "picked_up"
	MemberStatusSelfArrived    = "self_arrived"
	MemberStatusParentPickedUp = "parent_picked_up"
	MemberStatusLeave          = "leave"
	MemberStatusAbsent         = "absent"
	MemberStatusArrived        = "arrived"
	MemberStatusNotArrived     = "not_arrived"
	MemberStatusLeft           = "left"
	MemberStatusMidwayLeft     = "midway_left"
	MemberStatusAbnormal       = "abnormal"
)

type Operation struct {
	ID                     uint64
	OrganizationID         uint64
	OperationDate          time.Time
	PickupMode             string
	SchoolID               uint64
	SchoolClassID          uint64
	CareClassID            *uint64
	TeacherUserID          *uint64
	TeacherName            string
	Status                 string
	StartedAt              *time.Time
	FinishedAt             *time.Time
	ConfirmedAt            *time.Time
	ConfirmedByUserID      *uint64
	ConfirmedByName        string
	ExecutingTeacherUserID *uint64
	ExecutingTeacherName   string
	TeacherRole            string
	ExpectedPickupTime     string
	Notes                  string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type OperationStudent struct {
	ID             uint64
	OrganizationID uint64
	OperationID    uint64
	StudentID      uint64
	StudentName    string
	Status         string
	PhotoURL       string
	CheckedAt      *time.Time
	Note           string
	IsTemporary    bool
	ProfilePending bool
	PickupMode     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Handoff struct {
	ID                uint64
	OrganizationID    uint64
	OperationID       uint64
	FromTeacherUserID *uint64
	FromTeacherName   string
	ToTeacherUserID   *uint64
	ToTeacherName     string
	TeacherRole       string
	Note              string
	HandoffAt         time.Time
	CreatedByUserID   *uint64
	CreatedByName     string
}

type Event struct {
	ID                 uint64
	OrganizationID     uint64
	OperationID        uint64
	OperationStudentID uint64
	StudentID          uint64
	EventType          string
	EventAt            time.Time
	OperatorName       string
	PhotoURL           string
	Note               string
}

type Notification struct {
	ID               uint64
	OrganizationID   uint64
	StudentID        uint64
	OperationID      *uint64
	EventID          *uint64
	RecipientType    string
	Kind             string
	Title            string
	Content          string
	Status           string
	SentAt           *time.Time
	DeliveryAttempts int
	LastAttemptAt    *time.Time
	DeliveryError    string
	NextRetryAt      *time.Time
	ReadAt           *time.Time
	CreatedAt        time.Time
}

// NotificationOutbox is the durable hand-off from a committed notification
// to the asynchronous delivery worker. It deliberately contains no provider
// credentials or permanent media URLs.
type NotificationOutbox struct {
	ID             uint64
	OrganizationID uint64
	EventType      string
	AggregateType  string
	AggregateID    uint64
	NotificationID uint64
	Status         string
	Attempts       int
	AvailableAt    time.Time
	LockedAt       *time.Time
	ProcessedAt    *time.Time
	LastError      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type NotificationDeliveryLog struct {
	ID              uint64
	OrganizationID  uint64
	NotificationID  uint64
	ParentAccountID uint64
	MessageKind     string
	TemplateID      string
	Status          string
	Attempts        int
	LastAttemptAt   *time.Time
	SentAt          *time.Time
	NextRetryAt     *time.Time
	DeliveryError   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateDeliveryLogParams struct {
	NotificationID  uint64
	ParentAccountID uint64
	MessageKind     string
	TemplateID      string
}

type SetDeliveryLogStatusParams struct {
	ID            uint64
	Status        string
	Attempts      int
	LastAttemptAt *time.Time
	SentAt        *time.Time
	NextRetryAt   *time.Time
	DeliveryError string
}

type PickupChangeRequest struct {
	ID               uint64
	OrganizationID   uint64
	StudentID        uint64
	StudentName      string
	OperationID      *uint64
	ChangeDate       time.Time
	RequestedStatus  string
	Note             string
	SubmittedBy      string
	Status           string
	ReviewedByUserID *uint64
	ReviewedAt       *time.Time
	ReviewNote       string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type StudentRef struct {
	ID   uint64
	Name string
}

type CreateOperationParams struct {
	OperationDate time.Time
	PickupMode    string
	SchoolID      uint64
	SchoolClassID uint64
	CareClassID   *uint64
	TeacherUserID *uint64
	TeacherName   string
	Notes         string
}

type SetOperationStatusParams struct {
	ID     uint64
	Status string
}

type ConfirmOperationParams struct {
	ID                     uint64
	ExecutingTeacherUserID *uint64
	ExecutingTeacherName   string
	TeacherRole            string
	ExpectedPickupTime     string
	ConfirmedByUserID      *uint64
	ConfirmedByName        string
	Notes                  string
}

type HandoffOperationParams struct {
	ID              uint64
	ToTeacherUserID uint64
	ToTeacherName   string
	TeacherRole     string
	Note            string
	CreatedByUserID *uint64
	CreatedByName   string
}

type AddOperationStudentParams struct {
	OperationID    uint64
	StudentID      uint64
	StudentName    string
	IsTemporary    bool
	ProfilePending bool
	PickupMode     string
	Note           string
}

type MarkStudentParams struct {
	OperationID  uint64
	StudentID    uint64
	Status       string
	PhotoURL     string
	OperatorName string
	Note         string
}

type CorrectEventParams struct {
	OperationID  uint64
	EventID      uint64
	Status       string
	OperatorName string
	Reason       string
}

type CreateNotificationParams struct {
	StudentID   uint64
	OperationID *uint64
	EventID     *uint64
	Kind        string
	Title       string
	Content     string
}

type SetNotificationStatusParams struct {
	ID               uint64
	Status           string
	SentAt           *time.Time
	DeliveryAttempts int
	LastAttemptAt    *time.Time
	DeliveryError    string
	NextRetryAt      *time.Time
}

type CreatePickupChangeRequestParams struct {
	StudentID       uint64
	OperationID     *uint64
	ChangeDate      time.Time
	RequestedStatus string
	Note            string
	SubmittedBy     string
}

type ReviewPickupChangeRequestParams struct {
	ID               uint64
	Status           string
	ReviewedByUserID *uint64
	ReviewNote       string
}

type NotificationHook func(context.Context, Notification)
