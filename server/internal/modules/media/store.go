package media

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("media: asset not found")
	ErrConflict = errors.New("media: asset already exists")
)

type Store interface {
	Create(context.Context, uint64, CreateParams) (Asset, error)
	FindByKey(context.Context, uint64, string) (Asset, error)
	List(context.Context, uint64, string, int) ([]Asset, error)
}
