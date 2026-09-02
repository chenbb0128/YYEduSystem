package pickup

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound              = errors.New("pickup: resource not found")
	ErrConflict              = errors.New("pickup: resource already exists")
	ErrInvalidState          = errors.New("pickup: invalid state transition")
	ErrInvalidStatus         = errors.New("pickup: invalid student status")
	ErrUnauthorizedOperation = errors.New("pickup: operation is outside staff assignment")
)

const (
	ChangeRequestStatusPending  = "pending"
	ChangeRequestStatusApproved = "approved"
	ChangeRequestStatusRejected = "rejected"
)

func IsValidMemberStatus(status string) bool {
	switch status {
	case MemberStatusPickedUp, MemberStatusSelfArrived, MemberStatusParentPickedUp, MemberStatusLeave, MemberStatusAbsent, MemberStatusArrived, MemberStatusNotArrived, MemberStatusLeft, MemberStatusMidwayLeft, MemberStatusAbnormal:
		return true
	default:
		return false
	}
}

func IsValidMemberTransition(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case MemberStatusPlanned:
		return to == MemberStatusPickedUp || to == MemberStatusSelfArrived || to == MemberStatusParentPickedUp || to == MemberStatusLeave || to == MemberStatusAbsent
	case MemberStatusPickedUp:
		return to == MemberStatusArrived || to == MemberStatusNotArrived || to == MemberStatusAbnormal
	case MemberStatusSelfArrived:
		return to == MemberStatusArrived || to == MemberStatusLeft || to == MemberStatusMidwayLeft || to == MemberStatusAbnormal
	case MemberStatusNotArrived, MemberStatusAbnormal:
		return to == MemberStatusArrived || to == MemberStatusAbnormal
	case MemberStatusArrived:
		return to == MemberStatusLeft || to == MemberStatusMidwayLeft || to == MemberStatusAbnormal
	default:
		return false
	}
}

func IsReadyToFinish(status string) bool {
	switch status {
	case MemberStatusSelfArrived, MemberStatusParentPickedUp, MemberStatusLeave, MemberStatusAbsent, MemberStatusArrived, MemberStatusLeft, MemberStatusMidwayLeft:
		return true
	default:
		return false
	}
}

type Store interface {
	ListOperations(context.Context, uint64) ([]Operation, error)
	FindOperation(context.Context, uint64, uint64) (Operation, error)
	CreateOperation(context.Context, uint64, CreateOperationParams, []StudentRef) (Operation, error)
	ConfirmOperation(context.Context, uint64, ConfirmOperationParams) (Operation, error)
	HandoffOperation(context.Context, uint64, HandoffOperationParams) (Operation, error)
	ListHandoffs(context.Context, uint64, uint64) ([]Handoff, error)
	SetOperationStatus(context.Context, uint64, SetOperationStatusParams) (Operation, error)
	ListOperationStudents(context.Context, uint64, uint64) ([]OperationStudent, error)
	AddOperationStudent(context.Context, uint64, AddOperationStudentParams) (OperationStudent, error)
	CompleteOperationStudentProfile(context.Context, uint64, uint64, uint64) error
	MarkOperationStudent(context.Context, uint64, MarkStudentParams) (OperationStudent, error)
	CorrectOperationEvent(context.Context, uint64, CorrectEventParams) (OperationStudent, error)
	ListEvents(context.Context, uint64, uint64) ([]Event, error)
	CreateNotification(context.Context, uint64, CreateNotificationParams) (Notification, error)
	FindNotification(context.Context, uint64, uint64) (Notification, error)
	ListNotifications(context.Context, uint64) ([]Notification, error)
	SetNotificationStatus(context.Context, uint64, SetNotificationStatusParams) error
	MarkNotificationRead(context.Context, uint64, uint64) error
	ListNotificationOutbox(context.Context, time.Time, time.Time, int) ([]NotificationOutbox, error)
	ClaimNotificationOutbox(context.Context, uint64, uint64, time.Time) (bool, error)
	CompleteNotificationOutbox(context.Context, uint64, uint64, string, *time.Time, string) error
	CreateNotificationDeliveryLog(context.Context, uint64, CreateDeliveryLogParams) (NotificationDeliveryLog, error)
	SetNotificationDeliveryLogStatus(context.Context, uint64, SetDeliveryLogStatusParams) error
	ListNotificationDeliveryLogs(context.Context, uint64, *uint64, string) ([]NotificationDeliveryLog, error)
	RetryNotification(context.Context, uint64, uint64) error
	CreatePickupChangeRequest(context.Context, uint64, CreatePickupChangeRequestParams) (PickupChangeRequest, error)
	ListPickupChangeRequests(context.Context, uint64, *time.Time, string) ([]PickupChangeRequest, error)
	ReviewPickupChangeRequest(context.Context, uint64, ReviewPickupChangeRequestParams) (PickupChangeRequest, error)
}

type NotificationWriter interface {
	CreateNotification(context.Context, uint64, CreateNotificationParams) (Notification, error)
}

type NotificationHookStore interface {
	SetNotificationHook(NotificationHook)
}
