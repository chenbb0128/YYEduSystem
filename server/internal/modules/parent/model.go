package parent

import "time"

const (
	AccountStatusActive   = "active"
	AccountStatusDisabled = "disabled"

	BindingStatusActive   = "active"
	BindingStatusInactive = "inactive"

	LeaveSubmittedByParent  = "parent"
	LeaveSubmittedByTeacher = "teacher"

	LeaveStatusPending   = "pending"
	LeaveStatusApproved  = "approved"
	LeaveStatusRejected  = "rejected"
	LeaveStatusCancelled = "cancelled"

	ChildApplicationStatusPending   = "pending"
	ChildApplicationStatusNeedsInfo = "needs_info"
	ChildApplicationStatusApproved  = "approved"
	ChildApplicationStatusRejected  = "rejected"

	MessageKindPickup   = "pickup"
	MessageKindMeal     = "meal"
	MessageKindHomework = "homework"
	MessageKindLeave    = "leave"
	MessageKindSummary  = "summary"

	PrivacyPolicyCurrentVersion = "privacy-v3-20260901"
)

type Account struct {
	ID             uint64
	OrganizationID uint64
	OpenID         string
	Nickname       string
	Avatar         string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PrivacyConsent struct {
	ID              uint64
	OrganizationID  uint64
	ParentAccountID uint64
	PolicyVersion   string
	ConsentedAt     time.Time
	CreatedAt       time.Time
}

type MessageSubscription struct {
	OrganizationID  uint64
	ParentAccountID uint64
	Kind            string
	Status          string
	TemplateVersion string
	AuthorizedAt    *time.Time
	UpdatedAt       time.Time
}

type Binding struct {
	ID              uint64
	OrganizationID  uint64
	ParentAccountID uint64
	StudentID       uint64
	StudentName     string
	SchoolClassID   uint64
	CareClassID     *uint64
	SchoolName      string
	Grade           string
	ClassName       string
	CareClassName   string
	Relationship    string
	IsPrimary       bool
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ChildApplication struct {
	ID               uint64
	OrganizationID   uint64
	ParentAccountID  uint64
	StudentID        *uint64
	StudentName      string
	SchoolNameInput  string
	GradeInput       string
	ClassNameInput   string
	SchoolID         *uint64
	SchoolClassID    *uint64
	Grade            string
	ClassName        string
	GuardianName     string
	GuardianPhone    string
	Relationship     string
	Notes            string
	Status           string
	ReviewNote       string
	ReviewedByUserID *uint64
	ReviewedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type LeaveRequest struct {
	ID                uint64
	OrganizationID    uint64
	StudentID         uint64
	ParentAccountID   *uint64
	SubmittedByType   string
	SubmittedByUserID *uint64
	LeaveDate         time.Time
	Reason            string
	Status            string
	TeacherNote       string
	ReviewedByUserID  *uint64
	ReviewedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateAccountParams struct {
	OpenID   string
	Nickname string
	Avatar   string
}

type UpdateMessageSubscriptionParams struct {
	Kind            string
	Status          string
	TemplateVersion string
}

type RecordPrivacyConsentParams struct {
	PolicyVersion string
}

type BindStudentParams struct {
	ParentAccountID uint64
	StudentID       uint64
	Relationship    string
	IsPrimary       bool
}

type CreateChildApplicationParams struct {
	ParentAccountID uint64
	StudentName     string
	SchoolNameInput string
	GradeInput      string
	ClassNameInput  string
	SchoolID        *uint64
	SchoolClassID   *uint64
	Grade           string
	ClassName       string
	GuardianName    string
	GuardianPhone   string
	Relationship    string
	Notes           string
}

type UpdateChildApplicationParams struct {
	ID uint64
	CreateChildApplicationParams
}

type ReviewChildApplicationParams struct {
	ID               uint64
	Status           string
	StudentID        *uint64
	SchoolID         *uint64
	SchoolClassID    *uint64
	ReviewNote       string
	ReviewedByUserID uint64
}

type CreateLeaveRequestParams struct {
	StudentID       uint64
	ParentAccountID uint64
	LeaveDate       time.Time
	Reason          string
}

type CreateTeacherLeaveRequestParams struct {
	StudentID         uint64
	SubmittedByUserID uint64
	LeaveDate         time.Time
	Reason            string
}

type UpdateLeaveRequestParams struct {
	ID              uint64
	ParentAccountID uint64
	LeaveDate       time.Time
	Reason          string
}

type CancelLeaveRequestParams struct {
	ID              uint64
	ParentAccountID uint64
}

type ReviewLeaveRequestParams struct {
	ID               uint64
	Status           string
	TeacherNote      string
	ReviewedByUserID uint64
}
