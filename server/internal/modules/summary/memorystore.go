package summary

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu       sync.RWMutex
	nextID   uint64
	items    []DailySummary
	versions []Version
	reads    map[uint64]map[uint64]readRecord
}

type readRecord struct {
	version uint32
	at      time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{nextID: 1, reads: map[uint64]map[uint64]readRecord{}}
}
func (s *MemoryStore) List(_ context.Context, orgID uint64, date *time.Time) ([]DailySummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DailySummary, 0)
	for _, item := range s.items {
		if item.OrganizationID == orgID && (date == nil || sameDay(item.SummaryDate, *date)) {
			out = append(out, clone(item))
		}
	}
	slices.SortFunc(out, func(a, b DailySummary) int { return b.SummaryDate.Compare(a.SummaryDate) })
	return out, nil
}
func (s *MemoryStore) Find(_ context.Context, orgID, id uint64) (DailySummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.OrganizationID == orgID && item.ID == id {
			return clone(item), nil
		}
	}
	return DailySummary{}, fmt.Errorf("%w: %d", ErrNotFound, id)
}
func (s *MemoryStore) Generate(_ context.Context, orgID uint64, p GenerateParams) (DailySummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for i := range s.items {
		if s.items[i].OrganizationID == orgID && sameDay(s.items[i].SummaryDate, p.SummaryDate) && s.items[i].Status == StatusDraft {
			s.items[i].Version++
			s.items[i].Content = strings.TrimSpace(p.Content)
			s.items[i].ChildUpdates = cloneMap(p.ChildUpdates)
			s.items[i].CreatedByUserID = cloneID(p.CreatedByUserID)
			s.items[i].CreatedByName = strings.TrimSpace(p.CreatedByName)
			s.items[i].GeneratedAt = &now
			s.items[i].UpdatedAt = now
			s.appendVersion(s.items[i], "generated", "")
			return clone(s.items[i]), nil
		}
		if s.items[i].OrganizationID == orgID && sameDay(s.items[i].SummaryDate, p.SummaryDate) {
			return DailySummary{}, ErrInvalidState
		}
	}
	item := DailySummary{ID: s.nextID, OrganizationID: orgID, SummaryDate: dateOnly(p.SummaryDate), Content: strings.TrimSpace(p.Content), ChildUpdates: cloneMap(p.ChildUpdates), Status: StatusDraft, Version: 1, CreatedByUserID: cloneID(p.CreatedByUserID), CreatedByName: strings.TrimSpace(p.CreatedByName), GeneratedAt: &now, CreatedAt: now, UpdatedAt: now}
	s.nextID++
	s.items = append(s.items, item)
	s.appendVersion(item, "generated", "")
	return clone(item), nil
}
func (s *MemoryStore) Update(_ context.Context, orgID uint64, p UpdateParams) (DailySummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		item := &s.items[i]
		if item.OrganizationID != orgID || item.ID != p.ID {
			continue
		}
		if item.Status != StatusDraft {
			return DailySummary{}, ErrInvalidState
		}
		item.Version++
		item.Content = strings.TrimSpace(p.Content)
		item.ChildUpdates = cloneMap(p.ChildUpdates)
		item.UpdatedAt = time.Now().UTC()
		s.appendVersion(*item, "updated", "")
		return clone(*item), nil
	}
	return DailySummary{}, ErrNotFound
}
func (s *MemoryStore) SetStatus(_ context.Context, orgID, id uint64, status string) (DailySummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for i := range s.items {
		item := &s.items[i]
		if item.OrganizationID != orgID || item.ID != id {
			continue
		}
		if status == StatusPublished && item.Status == StatusDraft {
			item.Status = status
			item.PublishedAt = &now
			s.appendVersion(*item, "published", "")
		} else if status == StatusClosed && (item.Status == StatusPublished || item.Status == StatusDraft) {
			item.Status = status
			item.ClosedAt = &now
			s.appendVersion(*item, "closed", "")
		} else {
			return DailySummary{}, ErrInvalidState
		}
		item.UpdatedAt = now
		return clone(*item), nil
	}
	return DailySummary{}, ErrNotFound
}

func (s *MemoryStore) Withdraw(_ context.Context, orgID, id uint64, reason string) (DailySummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		item := &s.items[i]
		if item.OrganizationID != orgID || item.ID != id {
			continue
		}
		if item.Status != StatusPublished {
			return DailySummary{}, ErrInvalidState
		}
		now := time.Now().UTC()
		item.Status = StatusWithdrawn
		item.WithdrawnAt = &now
		item.WithdrawalReason = strings.TrimSpace(reason)
		item.UpdatedAt = now
		s.appendVersion(*item, "withdrawn", item.WithdrawalReason)
		return clone(*item), nil
	}
	return DailySummary{}, ErrNotFound
}

func (s *MemoryStore) Correct(_ context.Context, orgID uint64, p CorrectParams) (DailySummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		item := &s.items[i]
		if item.OrganizationID != orgID || item.ID != p.ID {
			continue
		}
		if item.Status != StatusPublished && item.Status != StatusClosed && item.Status != StatusWithdrawn {
			return DailySummary{}, ErrInvalidState
		}
		now := time.Now().UTC()
		item.Version++
		item.Content = strings.TrimSpace(p.Content)
		item.ChildUpdates = cloneMap(p.ChildUpdates)
		item.Status = StatusPublished
		item.ClosedAt = nil
		item.PublishedAt = &now
		item.WithdrawnAt = nil
		item.WithdrawalReason = ""
		item.CorrectionReason = strings.TrimSpace(p.Reason)
		item.CreatedByUserID = cloneID(p.CreatedByUserID)
		item.CreatedByName = strings.TrimSpace(p.CreatedByName)
		item.UpdatedAt = now
		s.appendVersion(*item, "corrected", item.CorrectionReason)
		return clone(*item), nil
	}
	return DailySummary{}, ErrNotFound
}

func (s *MemoryStore) ListVersions(_ context.Context, orgID, summaryID uint64) ([]Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Version, 0)
	for _, item := range s.versions {
		if item.SummaryID == summaryID {
			if current, err := s.findLocked(orgID, summaryID); err != nil || current.OrganizationID != orgID {
				continue
			}
			out = append(out, cloneVersion(item))
		}
	}
	if len(out) == 0 {
		if _, err := s.findLocked(orgID, summaryID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *MemoryStore) MarkRead(_ context.Context, orgID, summaryID, parentID uint64, version uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.findLocked(orgID, summaryID)
	if err != nil {
		return err
	}
	if item.Status != StatusPublished && item.Status != StatusClosed {
		return ErrInvalidState
	}
	if s.reads[summaryID] == nil {
		s.reads[summaryID] = map[uint64]readRecord{}
	}
	s.reads[summaryID][parentID] = readRecord{version: version, at: time.Now().UTC()}
	return nil
}

func (s *MemoryStore) ReadAt(_ context.Context, orgID, summaryID, parentID uint64) (*time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, err := s.findLocked(orgID, summaryID)
	if err != nil {
		return nil, err
	}
	value, ok := s.reads[summaryID][parentID]
	if !ok || value.version != item.Version {
		return nil, nil
	}
	return &value.at, nil
}

func (s *MemoryStore) findLocked(orgID, id uint64) (DailySummary, error) {
	for _, item := range s.items {
		if item.OrganizationID == orgID && item.ID == id {
			return item, nil
		}
	}
	return DailySummary{}, ErrNotFound
}

func (s *MemoryStore) appendVersion(item DailySummary, action, reason string) {
	s.versions = append(s.versions, Version{ID: uint64(len(s.versions) + 1), SummaryID: item.ID, Version: item.Version, Action: action, Content: item.Content, ChildUpdates: cloneMap(item.ChildUpdates), Reason: strings.TrimSpace(reason), CreatedByUserID: cloneID(item.CreatedByUserID), CreatedByName: item.CreatedByName, CreatedAt: item.UpdatedAt})
}

func cloneVersion(v Version) Version {
	v.ChildUpdates = cloneMap(v.ChildUpdates)
	v.CreatedByUserID = cloneID(v.CreatedByUserID)
	return v
}
func sameDay(a, b time.Time) bool {
	a, b = a.UTC(), b.UTC()
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}
func dateOnly(v time.Time) time.Time {
	v = v.UTC()
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
}
func cloneID(v *uint64) *uint64 {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}
func cloneMap(v map[uint64]string) map[uint64]string {
	if v == nil {
		return map[uint64]string{}
	}
	out := make(map[uint64]string, len(v))
	for k, x := range v {
		out[k] = x
	}
	return out
}
func clone(v DailySummary) DailySummary {
	v.CreatedByUserID = cloneID(v.CreatedByUserID)
	if v.GeneratedAt != nil {
		x := *v.GeneratedAt
		v.GeneratedAt = &x
	}
	if v.PublishedAt != nil {
		x := *v.PublishedAt
		v.PublishedAt = &x
	}
	if v.ClosedAt != nil {
		x := *v.ClosedAt
		v.ClosedAt = &x
	}
	if v.WithdrawnAt != nil {
		x := *v.WithdrawnAt
		v.WithdrawnAt = &x
	}
	v.ChildUpdates = cloneMap(v.ChildUpdates)
	return v
}
