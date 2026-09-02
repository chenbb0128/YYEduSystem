package audit

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.RWMutex
	nextID  uint64
	entries []Entry
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) Record(_ context.Context, params RecordParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	metadata := strings.TrimSpace(params.MetadataJSON)
	if metadata == "" {
		metadata = "{}"
	}
	if !json.Valid([]byte(metadata)) {
		metadata = `{"raw":"invalid metadata"}`
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	s.entries = append(s.entries, Entry{ID: s.nextID, OrganizationID: params.OrganizationID, ActorType: defaultActor(params.ActorType), ActorID: cloneID(params.ActorID), Action: strings.TrimSpace(params.Action), ResourceType: strings.TrimSpace(params.ResourceType), ResourceID: cloneID(params.ResourceID), MetadataJSON: metadata, RequestID: strings.TrimSpace(params.RequestID), CreatedAt: now})
	s.nextID++
	return nil
}

func (s *MemoryStore) List(_ context.Context, orgID uint64, filter ListFilter) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	out := make([]Entry, 0, limit)
	for index := len(s.entries) - 1; index >= 0 && len(out) < limit; index-- {
		item := s.entries[index]
		if item.OrganizationID != orgID || (strings.TrimSpace(filter.Action) != "" && item.Action != strings.TrimSpace(filter.Action)) || (strings.TrimSpace(filter.ResourceType) != "" && item.ResourceType != strings.TrimSpace(filter.ResourceType)) {
			continue
		}
		out = append(out, cloneEntry(item))
	}
	return out, nil
}

func defaultActor(value string) string {
	switch strings.TrimSpace(value) {
	case "staff", "parent", "system", "anonymous":
		return strings.TrimSpace(value)
	default:
		return "anonymous"
	}
}

func cloneID(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	x := *value
	return &x
}

func cloneEntry(value Entry) Entry {
	value.ActorID = cloneID(value.ActorID)
	value.ResourceID = cloneID(value.ResourceID)
	return value
}
