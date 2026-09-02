package assignment

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("assignment: resource not found")
	ErrConflict      = errors.New("assignment: resource already exists")
	ErrInvalidStatus = errors.New("assignment: invalid status")
)

type Store interface {
	List(context.Context, uint64, uint64, uint64) ([]TeacherClassAssignment, error)
	Find(context.Context, uint64, uint64) (TeacherClassAssignment, error)
	FindByPair(context.Context, uint64, uint64, uint64) (TeacherClassAssignment, error)
	Create(context.Context, uint64, CreateParams) (TeacherClassAssignment, error)
	SetStatus(context.Context, uint64, SetStatusParams) (TeacherClassAssignment, error)
}
