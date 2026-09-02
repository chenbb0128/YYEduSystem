package summary

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("summary: resource not found")
	ErrConflict     = errors.New("summary: resource already exists")
	ErrInvalidState = errors.New("summary: invalid state")
)

type Store interface {
	List(context.Context, uint64, *time.Time) ([]DailySummary, error)
	Find(context.Context, uint64, uint64) (DailySummary, error)
	ListVersions(context.Context, uint64, uint64) ([]Version, error)
	Generate(context.Context, uint64, GenerateParams) (DailySummary, error)
	Update(context.Context, uint64, UpdateParams) (DailySummary, error)
	SetStatus(context.Context, uint64, uint64, string) (DailySummary, error)
	Withdraw(context.Context, uint64, uint64, string) (DailySummary, error)
	Correct(context.Context, uint64, CorrectParams) (DailySummary, error)
	MarkRead(context.Context, uint64, uint64, uint64, uint32) error
	ReadAt(context.Context, uint64, uint64, uint64) (*time.Time, error)
}
