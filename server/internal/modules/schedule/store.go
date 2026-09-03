package schedule

import (
	"context"
	"errors"
)

var (
	ErrNotFound     = errors.New("schedule: resource not found")
	ErrConflict     = errors.New("schedule: resource already exists")
	ErrInvalid      = errors.New("schedule: invalid schedule")
	ErrUnauthorized = errors.New("schedule: schedule is outside staff assignment")
)

type Store interface {
	List(context.Context, uint64) ([]PickupSchedule, error)
	Find(context.Context, uint64, uint64) (PickupSchedule, error)
	Create(context.Context, uint64, CreateParams) (PickupSchedule, error)
	Update(context.Context, uint64, UpdateParams) (PickupSchedule, error)
}
