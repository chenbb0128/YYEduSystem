package schedule

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu        sync.RWMutex
	nextID    uint64
	schedules []PickupSchedule
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) List(_ context.Context, orgID uint64) ([]PickupSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PickupSchedule, 0)
	for _, item := range s.schedules {
		if item.OrganizationID == orgID {
			out = append(out, cloneSchedule(item))
		}
	}
	slices.SortFunc(out, func(left, right PickupSchedule) int {
		if left.Enabled != right.Enabled {
			if left.Enabled {
				return -1
			}
			return 1
		}
		if left.Weekday != right.Weekday {
			if left.Weekday < right.Weekday {
				return -1
			}
			return 1
		}
		if left.ID > right.ID {
			return -1
		}
		if left.ID < right.ID {
			return 1
		}
		return 0
	})
	return out, nil
}

func (s *MemoryStore) Find(_ context.Context, orgID, id uint64) (PickupSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.schedules {
		if item.OrganizationID == orgID && item.ID == id {
			return cloneSchedule(item), nil
		}
	}
	return PickupSchedule{}, fmt.Errorf("%w: schedule %d", ErrNotFound, id)
}

func (s *MemoryStore) Create(_ context.Context, orgID uint64, params CreateParams) (PickupSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateParams(params); err != nil {
		return PickupSchedule{}, err
	}
	for _, item := range s.schedules {
		if item.OrganizationID == orgID && item.Enabled && overlaps(item, params) {
			return PickupSchedule{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := fromParams(s.nextID, orgID, params, now)
	s.nextID++
	s.schedules = append(s.schedules, item)
	return cloneSchedule(item), nil
}

func (s *MemoryStore) Update(_ context.Context, orgID uint64, params UpdateParams) (PickupSchedule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateParams(params.CreateParams); err != nil {
		return PickupSchedule{}, err
	}
	for index := range s.schedules {
		item := &s.schedules[index]
		if item.OrganizationID != orgID || item.ID != params.ID {
			continue
		}
		for _, other := range s.schedules {
			if other.ID != item.ID && other.OrganizationID == orgID && other.Enabled && params.Enabled && overlaps(other, params.CreateParams) {
				return PickupSchedule{}, ErrConflict
			}
		}
		createdAt := item.CreatedAt
		updated := fromParams(item.ID, orgID, params.CreateParams, item.UpdatedAt)
		updated.CreatedAt = createdAt
		updated.UpdatedAt = time.Now().UTC()
		*item = updated
		return cloneSchedule(*item), nil
	}
	return PickupSchedule{}, fmt.Errorf("%w: schedule %d", ErrNotFound, params.ID)
}

func validateParams(params CreateParams) error {
	if params.SchoolID == 0 || params.SchoolClassID == 0 || (params.Weekday != time.Sunday && (params.Weekday < time.Monday || params.Weekday > time.Saturday)) || params.EffectiveFrom.IsZero() {
		return ErrInvalid
	}
	if params.PickupMode != PickupModeSchool && params.PickupMode != PickupModeSelf {
		return ErrInvalid
	}
	if params.EffectiveTo != nil && params.EffectiveTo.Before(params.EffectiveFrom) {
		return ErrInvalid
	}
	if len([]rune(strings.TrimSpace(params.ExpectedPickupTime))) > 16 || len([]rune(strings.TrimSpace(params.Notes))) > 500 {
		return ErrInvalid
	}
	return nil
}

func overlaps(item PickupSchedule, params CreateParams) bool {
	if item.SchoolClassID != params.SchoolClassID || item.Weekday != params.Weekday {
		return false
	}
	if item.EffectiveTo != nil && item.EffectiveTo.Before(params.EffectiveFrom) {
		return false
	}
	if params.EffectiveTo != nil && params.EffectiveTo.Before(item.EffectiveFrom) {
		return false
	}
	return true
}

func fromParams(id, orgID uint64, params CreateParams, now time.Time) PickupSchedule {
	return PickupSchedule{ID: id, OrganizationID: orgID, SchoolID: params.SchoolID, SchoolClassID: params.SchoolClassID, CareClassID: cloneID(params.CareClassID), Weekday: params.Weekday, PickupMode: params.PickupMode, TeacherUserID: cloneID(params.TeacherUserID), TeacherName: strings.TrimSpace(params.TeacherName), ExpectedPickupTime: strings.TrimSpace(params.ExpectedPickupTime), EffectiveFrom: params.EffectiveFrom, EffectiveTo: cloneTime(params.EffectiveTo), Enabled: params.Enabled, Notes: strings.TrimSpace(params.Notes), CreatedAt: now, UpdatedAt: now}
}

func cloneSchedule(item PickupSchedule) PickupSchedule {
	item.CareClassID = cloneID(item.CareClassID)
	item.TeacherUserID = cloneID(item.TeacherUserID)
	item.EffectiveTo = cloneTime(item.EffectiveTo)
	return item
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
	copy := *value
	return &copy
}
