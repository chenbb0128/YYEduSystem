package wrongbook

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
	questions []Question
	papers    []Paper
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) ListQuestions(_ context.Context, orgID uint64, params ListQuestionsParams) ([]Question, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keyword := strings.ToLower(strings.TrimSpace(params.Keyword))
	out := make([]Question, 0)
	for _, item := range s.questions {
		if item.OrganizationID != orgID {
			continue
		}
		if params.StudentID != 0 && item.StudentID != params.StudentID {
			continue
		}
		if params.Subject != "" && item.Subject != params.Subject {
			continue
		}
		if params.Status != "" && item.Status != params.Status {
			continue
		}
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{item.QuestionText, item.AnswerText, item.KnowledgePoint, item.TeacherNote}, "\n"))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		out = append(out, cloneQuestion(item))
	}
	sortQuestions(out)
	return out, nil
}

func (s *MemoryStore) FindQuestion(_ context.Context, orgID, id uint64) (Question, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.questions {
		if item.OrganizationID == orgID && item.ID == id {
			return cloneQuestion(item), nil
		}
	}
	return Question{}, fmt.Errorf("%w: question %d", ErrNotFound, id)
}

func (s *MemoryStore) CreateQuestion(_ context.Context, orgID uint64, params CreateQuestionParams) (Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.newQuestionLocked(orgID, params)
	s.questions = append(s.questions, item)
	return cloneQuestion(item), nil
}

func (s *MemoryStore) BulkCreateQuestions(_ context.Context, orgID uint64, params BulkCreateQuestionsParams) ([]Question, error) {
	if len(params.Items) == 0 {
		return nil, fmt.Errorf("%w: empty questions", ErrInvalidState)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Question, 0, len(params.Items))
	for _, item := range params.Items {
		created := s.newQuestionLocked(orgID, item)
		s.questions = append(s.questions, created)
		out = append(out, cloneQuestion(created))
	}
	return out, nil
}

func (s *MemoryStore) UpdateQuestion(_ context.Context, orgID uint64, params UpdateQuestionParams) (Question, error) {
	if !validQuestionStatus(params.Status) {
		return Question{}, ErrInvalidStatus
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.questions {
		item := &s.questions[index]
		if item.OrganizationID != orgID || item.ID != params.ID {
			continue
		}
		now := time.Now().UTC()
		item.Subject = strings.TrimSpace(params.Subject)
		item.QuestionText = strings.TrimSpace(params.QuestionText)
		item.AnswerText = strings.TrimSpace(params.AnswerText)
		item.Explanation = strings.TrimSpace(params.Explanation)
		item.KnowledgePoint = strings.TrimSpace(params.KnowledgePoint)
		item.TeacherNote = strings.TrimSpace(params.TeacherNote)
		item.Status = params.Status
		if params.Status == QuestionStatusMastered {
			item.LastReviewedAt = &now
		}
		item.UpdatedAt = now
		return cloneQuestion(*item), nil
	}
	return Question{}, fmt.Errorf("%w: question %d", ErrNotFound, params.ID)
}

func (s *MemoryStore) ListPapers(_ context.Context, orgID uint64, params ListPapersParams) ([]Paper, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Paper, 0)
	for _, item := range s.papers {
		if item.OrganizationID != orgID {
			continue
		}
		if params.StudentID != 0 && item.StudentID != params.StudentID {
			continue
		}
		out = append(out, clonePaper(item))
	}
	slices.SortFunc(out, func(left, right Paper) int {
		if left.CreatedAt.Equal(right.CreatedAt) {
			return compareDesc(left.ID, right.ID)
		}
		if left.CreatedAt.After(right.CreatedAt) {
			return -1
		}
		return 1
	})
	return out, nil
}

func (s *MemoryStore) FindPaper(_ context.Context, orgID, id uint64) (Paper, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.papers {
		if item.OrganizationID == orgID && item.ID == id {
			return clonePaper(item), nil
		}
	}
	return Paper{}, fmt.Errorf("%w: paper %d", ErrNotFound, id)
}

func (s *MemoryStore) CreatePaper(_ context.Context, orgID uint64, params CreatePaperParams) (Paper, error) {
	if len(params.QuestionIDs) == 0 {
		return Paper{}, fmt.Errorf("%w: no questions selected", ErrInvalidState)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	selected := make([]Question, 0, len(params.QuestionIDs))
	seen := make(map[uint64]struct{}, len(params.QuestionIDs))
	for _, id := range params.QuestionIDs {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		var found Question
		for _, question := range s.questions {
			if question.OrganizationID == orgID && question.ID == id && question.StudentID == params.StudentID {
				found = question
				break
			}
		}
		if found.ID == 0 {
			return Paper{}, fmt.Errorf("%w: question %d", ErrNotFound, id)
		}
		selected = append(selected, cloneQuestion(found))
	}
	if len(selected) == 0 {
		return Paper{}, fmt.Errorf("%w: no questions selected", ErrInvalidState)
	}
	now := time.Now().UTC()
	paper := Paper{
		ID:                s.nextID,
		OrganizationID:    orgID,
		StudentID:         params.StudentID,
		StudentName:       selected[0].StudentName,
		Title:             strings.TrimSpace(params.Title),
		Source:            defaultPaperSource(params.Source),
		Status:            PaperStatusGenerated,
		GeneratedByType:   defaultGeneratedBy(params.GeneratedByType),
		GeneratedByUserID: cloneID(params.GeneratedByUserID),
		QuestionCount:     len(selected),
		Questions:         selected,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if paper.Title == "" {
		paper.Title = defaultPaperTitle(now)
	}
	s.nextID++
	s.papers = append(s.papers, paper)
	return clonePaper(paper), nil
}

func (s *MemoryStore) newQuestionLocked(orgID uint64, params CreateQuestionParams) Question {
	now := time.Now().UTC()
	item := Question{
		ID:                   s.nextID,
		OrganizationID:       orgID,
		StudentID:            params.StudentID,
		Subject:              strings.TrimSpace(params.Subject),
		QuestionText:         strings.TrimSpace(params.QuestionText),
		AnswerText:           strings.TrimSpace(params.AnswerText),
		Explanation:          strings.TrimSpace(params.Explanation),
		KnowledgePoint:       strings.TrimSpace(params.KnowledgePoint),
		SourceImageURL:       strings.TrimSpace(params.SourceImageURL),
		SourceHomeworkTaskID: cloneID(params.SourceHomeworkTaskID),
		TeacherNote:          strings.TrimSpace(params.TeacherNote),
		Status:               QuestionStatusActive,
		CreatedByUserID:      cloneID(params.CreatedByUserID),
		CreatedByName:        strings.TrimSpace(params.CreatedByName),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	s.nextID++
	return item
}

func sortQuestions(items []Question) {
	slices.SortFunc(items, func(left, right Question) int {
		if left.CreatedAt.Equal(right.CreatedAt) {
			return compareDesc(left.ID, right.ID)
		}
		if left.CreatedAt.After(right.CreatedAt) {
			return -1
		}
		return 1
	})
}

func compareDesc(left, right uint64) int {
	if left > right {
		return -1
	}
	if left < right {
		return 1
	}
	return 0
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

func cloneQuestion(item Question) Question {
	item.SourceHomeworkTaskID = cloneID(item.SourceHomeworkTaskID)
	item.CreatedByUserID = cloneID(item.CreatedByUserID)
	item.LastReviewedAt = cloneTime(item.LastReviewedAt)
	return item
}

func clonePaper(item Paper) Paper {
	item.GeneratedByUserID = cloneID(item.GeneratedByUserID)
	item.Questions = append([]Question(nil), item.Questions...)
	for index := range item.Questions {
		item.Questions[index] = cloneQuestion(item.Questions[index])
	}
	return item
}
