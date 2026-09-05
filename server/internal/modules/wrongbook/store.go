package wrongbook

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("wrongbook: resource not found")
	ErrConflict      = errors.New("wrongbook: resource already exists")
	ErrInvalidStatus = errors.New("wrongbook: invalid status")
	ErrInvalidState  = errors.New("wrongbook: invalid state")
)

type Store interface {
	ListQuestions(context.Context, uint64, ListQuestionsParams) ([]Question, error)
	FindQuestion(context.Context, uint64, uint64) (Question, error)
	CreateQuestion(context.Context, uint64, CreateQuestionParams) (Question, error)
	BulkCreateQuestions(context.Context, uint64, BulkCreateQuestionsParams) ([]Question, error)
	UpdateQuestion(context.Context, uint64, UpdateQuestionParams) (Question, error)
	ListPapers(context.Context, uint64, ListPapersParams) ([]Paper, error)
	FindPaper(context.Context, uint64, uint64) (Paper, error)
	CreatePaper(context.Context, uint64, CreatePaperParams) (Paper, error)
}
