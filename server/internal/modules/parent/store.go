package parent

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("parent: resource not found")
	ErrConflict      = errors.New("parent: resource already exists")
	ErrInvalidStatus = errors.New("parent: invalid status")
	ErrInvalidState  = errors.New("parent: invalid state transition")
)

type Store interface {
	CreateAccount(context.Context, uint64, CreateAccountParams) (Account, error)
	FindAccountByID(context.Context, uint64, uint64) (Account, error)
	FindAccountByOpenID(context.Context, uint64, string) (Account, error)
	GetLatestPrivacyConsent(context.Context, uint64, uint64) (PrivacyConsent, error)
	RecordPrivacyConsent(context.Context, uint64, uint64, RecordPrivacyConsentParams) (PrivacyConsent, error)
	ListAccountsForStudent(context.Context, uint64, uint64) ([]Account, error)
	ListMessageSubscriptions(context.Context, uint64, uint64) ([]MessageSubscription, error)
	UpdateMessageSubscriptions(context.Context, uint64, uint64, []UpdateMessageSubscriptionParams) error
	CreateBinding(context.Context, uint64, BindStudentParams) (Binding, error)
	ListBindings(context.Context, uint64, uint64) ([]Binding, error)
	CreateChildApplication(context.Context, uint64, CreateChildApplicationParams) (ChildApplication, error)
	UpdateChildApplication(context.Context, uint64, UpdateChildApplicationParams) (ChildApplication, error)
	GetChildApplication(context.Context, uint64, uint64) (ChildApplication, error)
	ListChildApplications(context.Context, uint64, *uint64) ([]ChildApplication, error)
	ReviewChildApplication(context.Context, uint64, ReviewChildApplicationParams) (ChildApplication, error)
	CreateLeaveRequest(context.Context, uint64, CreateLeaveRequestParams) (LeaveRequest, error)
	UpdateLeaveRequest(context.Context, uint64, UpdateLeaveRequestParams) (LeaveRequest, error)
	CancelLeaveRequest(context.Context, uint64, CancelLeaveRequestParams) (LeaveRequest, error)
	ListLeaveRequests(context.Context, uint64, *uint64) ([]LeaveRequest, error)
	ListApprovedLeaveStudentIDs(context.Context, uint64, time.Time) (map[uint64]struct{}, error)
	ReviewLeaveRequest(context.Context, uint64, ReviewLeaveRequestParams) (LeaveRequest, error)
}

type TeacherLeaveStore interface {
	CreateTeacherLeaveRequest(context.Context, uint64, CreateTeacherLeaveRequestParams) (LeaveRequest, error)
}
