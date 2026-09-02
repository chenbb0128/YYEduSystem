package assignment

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// MemoryStore keeps local development usable when MySQL is disabled.
type MemoryStore struct {
	mu          sync.RWMutex
	nextID      uint64
	assignments []TeacherClassAssignment
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) List(_ context.Context, orgID, teacherUserID, schoolClassID uint64) ([]TeacherClassAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TeacherClassAssignment, 0)
	for _, item := range s.assignments {
		if item.OrganizationID != orgID || (teacherUserID != 0 && item.TeacherUserID != teacherUserID) || (schoolClassID != 0 && item.SchoolClassID != schoolClassID) {
			continue
		}
		out = append(out, item)
	}
	slices.SortFunc(out, func(left, right TeacherClassAssignment) int {
		if left.Status != right.Status {
			if left.Status == AssignmentStatusActive {
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

func (s *MemoryStore) Find(_ context.Context, orgID, id uint64) (TeacherClassAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.assignments {
		if item.OrganizationID == orgID && item.ID == id {
			return item, nil
		}
	}
	return TeacherClassAssignment{}, fmt.Errorf("%w: assignment %d", ErrNotFound, id)
}

func (s *MemoryStore) FindByPair(_ context.Context, orgID, teacherUserID, schoolClassID uint64) (TeacherClassAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.assignments {
		if item.OrganizationID == orgID && item.TeacherUserID == teacherUserID && item.SchoolClassID == schoolClassID {
			return item, nil
		}
	}
	return TeacherClassAssignment{}, fmt.Errorf("%w: teacher %d class %d", ErrNotFound, teacherUserID, schoolClassID)
}

func (s *MemoryStore) Create(_ context.Context, orgID uint64, params CreateParams) (TeacherClassAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.assignments {
		item := &s.assignments[index]
		if item.OrganizationID != orgID || item.TeacherUserID != params.TeacherUserID || item.SchoolClassID != params.SchoolClassID {
			continue
		}
		if item.Status == AssignmentStatusDisabled {
			item.Status = AssignmentStatusActive
			item.UpdatedAt = time.Now().UTC()
			return *item, nil
		}
		return TeacherClassAssignment{}, ErrConflict
	}
	now := time.Now().UTC()
	item := TeacherClassAssignment{ID: s.nextID, OrganizationID: orgID, TeacherUserID: params.TeacherUserID, SchoolClassID: params.SchoolClassID, Status: AssignmentStatusActive, CreatedAt: now, UpdatedAt: now}
	s.nextID++
	s.assignments = append(s.assignments, item)
	return item, nil
}

func (s *MemoryStore) SetStatus(_ context.Context, orgID uint64, params SetStatusParams) (TeacherClassAssignment, error) {
	if params.Status != AssignmentStatusActive && params.Status != AssignmentStatusDisabled {
		return TeacherClassAssignment{}, ErrInvalidStatus
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.assignments {
		item := &s.assignments[index]
		if item.OrganizationID == orgID && item.ID == params.ID {
			item.Status = strings.TrimSpace(params.Status)
			item.UpdatedAt = time.Now().UTC()
			return *item, nil
		}
	}
	return TeacherClassAssignment{}, fmt.Errorf("%w: assignment %d", ErrNotFound, params.ID)
}
