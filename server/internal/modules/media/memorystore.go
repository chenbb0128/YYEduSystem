package media

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu     sync.RWMutex
	nextID uint64
	items  []Asset
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) Create(_ context.Context, orgID uint64, params CreateParams) (Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := strings.TrimSpace(params.ObjectKey)
	for _, item := range s.items {
		if item.OrganizationID == orgID && item.ObjectKey == key {
			return Asset{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := Asset{ID: s.nextID, OrganizationID: orgID, ObjectKey: key, ResourceType: strings.TrimSpace(params.ResourceType), ResourceID: cloneID(params.ResourceID), OwnerType: strings.TrimSpace(params.OwnerType), OwnerID: cloneID(params.OwnerID), ContentType: strings.TrimSpace(params.ContentType), SizeBytes: params.SizeBytes, SHA256: strings.TrimSpace(params.SHA256), Status: AssetStatusActive, RetentionUntil: cloneTime(params.RetentionUntil), CreatedByUserID: cloneID(params.CreatedByUserID), CreatedAt: now}
	s.nextID++
	s.items = append(s.items, item)
	return clone(item), nil
}

func (s *MemoryStore) FindByKey(_ context.Context, orgID uint64, key string) (Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.OrganizationID == orgID && item.ObjectKey == strings.TrimSpace(key) && item.Status == AssetStatusActive {
			return clone(item), nil
		}
	}
	return Asset{}, fmt.Errorf("%w: %s", ErrNotFound, key)
}

func (s *MemoryStore) List(_ context.Context, orgID uint64, resourceType string, limit int) ([]Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]Asset, 0, min(limit, len(s.items)))
	for index := len(s.items) - 1; index >= 0 && len(out) < limit; index-- {
		item := s.items[index]
		if item.OrganizationID == orgID && item.Status == AssetStatusActive && (strings.TrimSpace(resourceType) == "" || item.ResourceType == strings.TrimSpace(resourceType)) {
			out = append(out, clone(item))
		}
	}
	slices.SortFunc(out, func(a, b Asset) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return out, nil
}

func cloneID(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func clone(value Asset) Asset {
	value.ResourceID = cloneID(value.ResourceID)
	value.OwnerID = cloneID(value.OwnerID)
	value.RetentionUntil = cloneTime(value.RetentionUntil)
	value.CreatedByUserID = cloneID(value.CreatedByUserID)
	value.DeletedAt = cloneTime(value.DeletedAt)
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
