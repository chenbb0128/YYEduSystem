package meal

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("meal: resource not found")
	ErrConflict     = errors.New("meal: resource already exists")
	ErrInvalidState = errors.New("meal: invalid state")
)

type Store interface {
	ListPlans(context.Context, uint64, *time.Time, *time.Time) ([]Plan, error)
	FindPlan(context.Context, uint64, uint64) (Plan, error)
	UpsertPlan(context.Context, uint64, UpsertPlanParams) (Plan, error)
	CopyPlan(context.Context, uint64, CopyPlanParams) (Plan, error)
	ListDietNotes(context.Context, uint64, *uint64) ([]DietNote, error)
	UpsertDietNote(context.Context, uint64, UpsertDietNoteParams) (DietNote, error)
	ListDietNoteChangeRequests(context.Context, uint64, *uint64, *string) ([]DietNoteChangeRequest, error)
	CreateDietNoteChangeRequest(context.Context, uint64, CreateDietNoteChangeRequestParams) (DietNoteChangeRequest, error)
	ReviewDietNoteChangeRequest(context.Context, uint64, ReviewDietNoteChangeRequestParams) (DietNoteChangeRequest, error)
}
