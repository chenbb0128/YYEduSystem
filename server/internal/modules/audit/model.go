package audit

import "context"

type Entry struct {
	ID             uint64
	OrganizationID uint64
	ActorType      string
	ActorID        *uint64
	Action         string
	ResourceType   string
	ResourceID     *uint64
	MetadataJSON   string
	RequestID      string
	CreatedAt      string
}

type RecordParams struct {
	OrganizationID uint64
	ActorType      string
	ActorID        *uint64
	Action         string
	ResourceType   string
	ResourceID     *uint64
	MetadataJSON   string
	RequestID      string
}

type ListFilter struct {
	Action       string
	ResourceType string
	Limit        int
}

type Writer interface {
	Record(context.Context, RecordParams) error
}

type Store interface {
	Writer
	List(context.Context, uint64, ListFilter) ([]Entry, error)
}
