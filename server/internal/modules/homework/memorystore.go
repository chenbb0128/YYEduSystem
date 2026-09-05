package homework

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// MemoryStore keeps local UI work usable without MySQL.
type MemoryStore struct {
	mu       sync.RWMutex
	nextID   uint64
	tasks    []Task
	students []TaskStudent
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) ListTasks(_ context.Context, orgID uint64) ([]Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Task, 0)
	for _, item := range s.tasks {
		if item.OrganizationID == orgID {
			out = append(out, cloneTask(item))
		}
	}
	slices.SortFunc(out, func(left, right Task) int {
		if left.HomeworkDate.Equal(right.HomeworkDate) {
			if left.ID > right.ID {
				return -1
			}
			if left.ID < right.ID {
				return 1
			}
			return 0
		}
		if left.HomeworkDate.After(right.HomeworkDate) {
			return -1
		}
		return 1
	})
	return out, nil
}

func (s *MemoryStore) FindTask(_ context.Context, orgID, id uint64) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.tasks {
		if item.OrganizationID == orgID && item.ID == id {
			return cloneTask(item), nil
		}
	}
	return Task{}, fmt.Errorf("%w: task %d", ErrNotFound, id)
}

func (s *MemoryStore) CreateTask(_ context.Context, orgID uint64, params CreateTaskParams, roster []StudentRef) (Task, error) {
	if len(roster) == 0 {
		return Task{}, fmt.Errorf("%w: roster is empty", ErrConflict)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	createdBy := cloneID(params.CreatedByUserID)
	task := Task{ID: s.nextID, OrganizationID: orgID, HomeworkDate: params.HomeworkDate, SchoolID: params.SchoolID, SchoolClassID: params.SchoolClassID, Subject: strings.TrimSpace(params.Subject), Content: strings.TrimSpace(params.Content), AttachmentURLs: append([]string(nil), params.AttachmentURLs...), CreatedByUserID: createdBy, CreatorName: strings.TrimSpace(params.CreatorName), Status: TaskStatusActive, CreatedAt: now, UpdatedAt: now}
	s.nextID++
	s.tasks = append(s.tasks, task)
	seen := make(map[uint64]struct{}, len(roster))
	for _, student := range roster {
		if student.ID == 0 {
			continue
		}
		if _, ok := seen[student.ID]; ok {
			continue
		}
		seen[student.ID] = struct{}{}
		s.students = append(s.students, TaskStudent{ID: s.nextID, OrganizationID: orgID, TaskID: task.ID, StudentID: student.ID, StudentName: student.Name, Status: StudentStatusPending, CreatedAt: now, UpdatedAt: now})
		s.nextID++
	}
	if len(seen) == 0 {
		return Task{}, fmt.Errorf("%w: roster is empty", ErrConflict)
	}
	return cloneTask(task), nil
}

func (s *MemoryStore) ListTaskStudents(_ context.Context, orgID, taskID uint64) ([]TaskStudent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TaskStudent, 0)
	for _, item := range s.students {
		if item.OrganizationID == orgID && item.TaskID == taskID {
			out = append(out, cloneTaskStudent(item))
		}
	}
	if len(out) == 0 && !s.taskExistsLocked(orgID, taskID) {
		return nil, fmt.Errorf("%w: task %d", ErrNotFound, taskID)
	}
	slices.SortFunc(out, func(left, right TaskStudent) int { return strings.Compare(left.StudentName, right.StudentName) })
	return out, nil
}

func (s *MemoryStore) ReviewStudent(_ context.Context, orgID uint64, params ReviewStudentParams) (TaskStudent, error) {
	if !validStudentStatus(params.Status) {
		return TaskStudent{}, ErrInvalidStatus
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.students {
		item := &s.students[index]
		if item.OrganizationID != orgID || item.TaskID != params.TaskID || item.StudentID != params.StudentID {
			continue
		}
		now := time.Now().UTC()
		item.Status = params.Status
		item.CorrectionNote = strings.TrimSpace(params.CorrectionNote)
		item.ReviewedByUserID = cloneID(params.ReviewedByUserID)
		item.ReviewedAt = &now
		item.UpdatedAt = now
		return cloneTaskStudent(*item), nil
	}
	return TaskStudent{}, fmt.Errorf("%w: student %d", ErrNotFound, params.StudentID)
}

func (s *MemoryStore) BulkReviewStudents(_ context.Context, orgID uint64, params BulkReviewStudentsParams) ([]TaskStudent, error) {
	if len(params.Items) == 0 {
		return nil, fmt.Errorf("%w: empty review batch", ErrConflict)
	}
	seen := make(map[uint64]struct{}, len(params.Items))
	for _, review := range params.Items {
		if review.StudentID == 0 {
			return nil, fmt.Errorf("%w: student is required", ErrNotFound)
		}
		if !validStudentStatus(review.Status) {
			return nil, ErrInvalidStatus
		}
		if _, exists := seen[review.StudentID]; exists {
			return nil, fmt.Errorf("%w: duplicate student %d", ErrConflict, review.StudentID)
		}
		seen[review.StudentID] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	positions := make(map[uint64]int, len(params.Items))
	for index := range s.students {
		item := s.students[index]
		if item.OrganizationID == orgID && item.TaskID == params.TaskID {
			positions[item.StudentID] = index
		}
	}
	for _, review := range params.Items {
		if _, exists := positions[review.StudentID]; !exists {
			return nil, fmt.Errorf("%w: student %d", ErrNotFound, review.StudentID)
		}
	}
	now := time.Now().UTC()
	out := make([]TaskStudent, 0, len(params.Items))
	for _, review := range params.Items {
		item := &s.students[positions[review.StudentID]]
		item.Status = review.Status
		item.CorrectionNote = strings.TrimSpace(review.CorrectionNote)
		item.ReviewedByUserID = cloneID(params.ReviewedByUserID)
		item.ReviewedAt = &now
		item.UpdatedAt = now
		out = append(out, cloneTaskStudent(*item))
	}
	return out, nil
}

func (s *MemoryStore) ListStudentHomework(_ context.Context, orgID, studentID uint64) ([]StudentHomework, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make(map[uint64]Task, len(s.tasks))
	for _, task := range s.tasks {
		if task.OrganizationID == orgID && task.Status == TaskStatusActive {
			tasks[task.ID] = task
		}
	}
	out := make([]StudentHomework, 0)
	for _, item := range s.students {
		if item.OrganizationID != orgID || item.StudentID != studentID {
			continue
		}
		task, ok := tasks[item.TaskID]
		if !ok {
			continue
		}
		out = append(out, StudentHomework{Task: cloneTask(task), TaskStudent: cloneTaskStudent(item)})
	}
	slices.SortFunc(out, func(left, right StudentHomework) int {
		if left.HomeworkDate.Equal(right.HomeworkDate) {
			if left.Task.ID > right.Task.ID {
				return -1
			}
			if left.Task.ID < right.Task.ID {
				return 1
			}
			return 0
		}
		if left.HomeworkDate.After(right.HomeworkDate) {
			return -1
		}
		return 1
	})
	return out, nil
}

func (s *MemoryStore) taskExistsLocked(orgID, taskID uint64) bool {
	for _, item := range s.tasks {
		if item.OrganizationID == orgID && item.ID == taskID {
			return true
		}
	}
	return false
}

func validStudentStatus(status string) bool {
	return status == StudentStatusCompleted || status == StudentStatusIncomplete || status == StudentStatusNotSubmitted
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

func cloneTask(item Task) Task {
	item.CreatedByUserID = cloneID(item.CreatedByUserID)
	item.AttachmentURLs = append([]string(nil), item.AttachmentURLs...)
	return item
}

func cloneTaskStudent(item TaskStudent) TaskStudent {
	item.ReviewedByUserID = cloneID(item.ReviewedByUserID)
	item.ReviewedAt = cloneTime(item.ReviewedAt)
	return item
}
