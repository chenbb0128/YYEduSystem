package homework

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("homework: resource not found")
	ErrConflict      = errors.New("homework: resource already exists")
	ErrInvalidStatus = errors.New("homework: invalid student status")
)

type Store interface {
	ListTasks(context.Context, uint64) ([]Task, error)
	FindTask(context.Context, uint64, uint64) (Task, error)
	CreateTask(context.Context, uint64, CreateTaskParams, []StudentRef) (Task, error)
	ListTaskStudents(context.Context, uint64, uint64) ([]TaskStudent, error)
	ReviewStudent(context.Context, uint64, ReviewStudentParams) (TaskStudent, error)
	BulkReviewStudents(context.Context, uint64, BulkReviewStudentsParams) ([]TaskStudent, error)
	ListStudentHomework(context.Context, uint64, uint64) ([]StudentHomework, error)
}
