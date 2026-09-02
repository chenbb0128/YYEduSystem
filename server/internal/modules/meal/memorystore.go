package meal

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu                     sync.RWMutex
	nextID                 uint64
	plans                  []Plan
	notes                  []DietNote
	dietNoteChangeRequests []DietNoteChangeRequest
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) newID() uint64 { id := s.nextID; s.nextID++; return id }

func (s *MemoryStore) ListPlans(_ context.Context, orgID uint64, from, to *time.Time) ([]Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Plan, 0, len(s.plans))
	for _, item := range s.plans {
		if item.OrganizationID != orgID || (from != nil && item.MealDate.Before(*from)) || (to != nil && item.MealDate.After(*to)) {
			continue
		}
		out = append(out, clonePlan(item))
	}
	slices.SortFunc(out, func(a, b Plan) int { return b.MealDate.Compare(a.MealDate) })
	return out, nil
}

func (s *MemoryStore) FindPlan(_ context.Context, orgID, id uint64) (Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.plans {
		if item.OrganizationID == orgID && item.ID == id {
			return clonePlan(item), nil
		}
	}
	return Plan{}, fmt.Errorf("%w: plan %d", ErrNotFound, id)
}

func (s *MemoryStore) UpsertPlan(_ context.Context, orgID uint64, params UpsertPlanParams) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for index := range s.plans {
		item := &s.plans[index]
		if item.OrganizationID == orgID && sameDay(item.MealDate, params.MealDate) {
			item.MenuText, item.PhotoURL, item.AdjustmentNote = strings.TrimSpace(params.MenuText), strings.TrimSpace(params.PhotoURL), strings.TrimSpace(params.AdjustmentNote)
			item.CreatedByUserID, item.CreatedByName, item.Status, item.UpdatedAt = cloneID(params.CreatedByUserID), strings.TrimSpace(params.CreatedByName), PlanStatusActive, now
			return clonePlan(*item), nil
		}
	}
	item := Plan{ID: s.newID(), OrganizationID: orgID, MealDate: dateOnly(params.MealDate), MenuText: strings.TrimSpace(params.MenuText), PhotoURL: strings.TrimSpace(params.PhotoURL), AdjustmentNote: strings.TrimSpace(params.AdjustmentNote), CreatedByUserID: cloneID(params.CreatedByUserID), CreatedByName: strings.TrimSpace(params.CreatedByName), Status: PlanStatusActive, CreatedAt: now, UpdatedAt: now}
	s.plans = append(s.plans, item)
	return clonePlan(item), nil
}

func (s *MemoryStore) CopyPlan(ctx context.Context, orgID uint64, params CopyPlanParams) (Plan, error) {
	s.mu.RLock()
	var source *Plan
	for _, item := range s.plans {
		if item.OrganizationID == orgID && sameDay(item.MealDate, params.SourceDate) {
			copy := clonePlan(item)
			source = &copy
			break
		}
	}
	s.mu.RUnlock()
	if source == nil {
		return Plan{}, fmt.Errorf("%w: source plan", ErrNotFound)
	}
	return s.UpsertPlan(ctx, orgID, UpsertPlanParams{MealDate: params.TargetDate, MenuText: source.MenuText, PhotoURL: "", AdjustmentNote: source.AdjustmentNote, CreatedByUserID: params.CreatedByUserID, CreatedByName: params.CreatedByName})
}

func (s *MemoryStore) ListDietNotes(_ context.Context, orgID uint64, studentID *uint64) ([]DietNote, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DietNote, 0, len(s.notes))
	for _, item := range s.notes {
		if item.OrganizationID == orgID && (studentID == nil || item.StudentID == *studentID) {
			out = append(out, cloneDietNote(item))
		}
	}
	return out, nil
}

func (s *MemoryStore) UpsertDietNote(_ context.Context, orgID uint64, params UpsertDietNoteParams) (DietNote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for index := range s.notes {
		item := &s.notes[index]
		if item.OrganizationID == orgID && item.StudentID == params.StudentID {
			item.Note, item.UpdatedByUserID, item.UpdatedByName, item.UpdatedAt = strings.TrimSpace(params.Note), cloneID(params.UpdatedByUserID), strings.TrimSpace(params.UpdatedByName), now
			return cloneDietNote(*item), nil
		}
	}
	item := DietNote{ID: s.newID(), OrganizationID: orgID, StudentID: params.StudentID, Note: strings.TrimSpace(params.Note), UpdatedByUserID: cloneID(params.UpdatedByUserID), UpdatedByName: strings.TrimSpace(params.UpdatedByName), CreatedAt: now, UpdatedAt: now}
	s.notes = append(s.notes, item)
	return cloneDietNote(item), nil
}

func (s *MemoryStore) ListDietNoteChangeRequests(_ context.Context, orgID uint64, studentID *uint64, status *string) ([]DietNoteChangeRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DietNoteChangeRequest, 0)
	for _, item := range s.dietNoteChangeRequests {
		if item.OrganizationID != orgID || (studentID != nil && item.StudentID != *studentID) || (status != nil && item.Status != *status) {
			continue
		}
		out = append(out, cloneDietNoteChangeRequest(item))
	}
	slices.SortFunc(out, func(left, right DietNoteChangeRequest) int {
		if left.Status == DietNoteChangeStatusPending && right.Status != DietNoteChangeStatusPending {
			return -1
		}
		if right.Status == DietNoteChangeStatusPending && left.Status != DietNoteChangeStatusPending {
			return 1
		}
		if left.CreatedAt.After(right.CreatedAt) {
			return -1
		}
		if left.CreatedAt.Before(right.CreatedAt) {
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

func (s *MemoryStore) CreateDietNoteChangeRequest(_ context.Context, orgID uint64, params CreateDietNoteChangeRequestParams) (DietNoteChangeRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.dietNoteChangeRequests {
		if item.OrganizationID == orgID && item.StudentID == params.StudentID && item.Status == DietNoteChangeStatusPending {
			return DietNoteChangeRequest{}, ErrConflict
		}
	}
	currentNote := ""
	for _, item := range s.notes {
		if item.OrganizationID == orgID && item.StudentID == params.StudentID {
			currentNote = item.Note
			break
		}
	}
	now := time.Now().UTC()
	item := DietNoteChangeRequest{
		ID: s.newID(), OrganizationID: orgID, StudentID: params.StudentID, ParentAccountID: params.ParentAccountID,
		CurrentNote: currentNote, RequestedNote: strings.TrimSpace(params.RequestedNote), Status: DietNoteChangeStatusPending,
		CreatedAt: now, UpdatedAt: now,
	}
	s.dietNoteChangeRequests = append(s.dietNoteChangeRequests, item)
	return cloneDietNoteChangeRequest(item), nil
}

func (s *MemoryStore) ReviewDietNoteChangeRequest(_ context.Context, orgID uint64, params ReviewDietNoteChangeRequestParams) (DietNoteChangeRequest, error) {
	if params.Status != DietNoteChangeStatusApproved && params.Status != DietNoteChangeStatusRejected {
		return DietNoteChangeRequest{}, ErrInvalidState
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.dietNoteChangeRequests {
		item := &s.dietNoteChangeRequests[index]
		if item.OrganizationID != orgID || item.ID != params.ID {
			continue
		}
		if item.Status != DietNoteChangeStatusPending {
			return DietNoteChangeRequest{}, ErrInvalidState
		}
		now := time.Now().UTC()
		if params.Status == DietNoteChangeStatusApproved {
			updated := false
			for noteIndex := range s.notes {
				note := &s.notes[noteIndex]
				if note.OrganizationID == orgID && note.StudentID == item.StudentID {
					note.Note = item.RequestedNote
					note.UpdatedByUserID = cloneID(uint64Ptr(params.ReviewedByUserID))
					note.UpdatedByName = "审核人"
					note.UpdatedAt = now
					updated = true
					break
				}
			}
			if !updated {
				s.notes = append(s.notes, DietNote{ID: s.newID(), OrganizationID: orgID, StudentID: item.StudentID, Note: item.RequestedNote, UpdatedByUserID: cloneID(uint64Ptr(params.ReviewedByUserID)), UpdatedByName: "审核人", CreatedAt: now, UpdatedAt: now})
			}
		}
		item.Status = params.Status
		item.ReviewNote = strings.TrimSpace(params.ReviewNote)
		item.ReviewedByUserID = cloneID(uint64Ptr(params.ReviewedByUserID))
		item.ReviewedAt = &now
		item.UpdatedAt = now
		return cloneDietNoteChangeRequest(*item), nil
	}
	return DietNoteChangeRequest{}, fmt.Errorf("%w: diet note change request %d", ErrNotFound, params.ID)
}

func sameDay(a, b time.Time) bool {
	a, b = a.UTC(), b.UTC()
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}
func dateOnly(value time.Time) time.Time {
	v := value.UTC()
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
}
func cloneID(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
func clonePlan(value Plan) Plan { value.CreatedByUserID = cloneID(value.CreatedByUserID); return value }
func cloneDietNote(value DietNote) DietNote {
	value.UpdatedByUserID = cloneID(value.UpdatedByUserID)
	return value
}

func cloneDietNoteChangeRequest(value DietNoteChangeRequest) DietNoteChangeRequest {
	value.ReviewedByUserID = cloneID(value.ReviewedByUserID)
	if value.ReviewedAt != nil {
		copy := *value.ReviewedAt
		value.ReviewedAt = &copy
	}
	return value
}

func uint64Ptr(value uint64) *uint64 { return &value }
