package parent

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// MemoryStore keeps the local development workflow usable without MySQL.
type MemoryStore struct {
	mu              sync.RWMutex
	nextID          uint64
	accounts        []Account
	privacyConsents []PrivacyConsent
	subscriptions   []MessageSubscription
	bindings        []Binding
	applications    []ChildApplication
	leaves          []LeaveRequest
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) newID() uint64 {
	id := s.nextID
	s.nextID++
	return id
}

func (s *MemoryStore) CreateAccount(_ context.Context, orgID uint64, params CreateAccountParams) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.accounts {
		if item.OrganizationID == orgID && item.OpenID == params.OpenID {
			return Account{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := Account{ID: s.newID(), OrganizationID: orgID, OpenID: strings.TrimSpace(params.OpenID), Nickname: strings.TrimSpace(params.Nickname), Avatar: strings.TrimSpace(params.Avatar), Status: AccountStatusActive, CreatedAt: now, UpdatedAt: now}
	s.accounts = append(s.accounts, item)
	return item, nil
}

func (s *MemoryStore) FindAccountByID(_ context.Context, orgID, id uint64) (Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.accounts {
		if item.OrganizationID == orgID && item.ID == id {
			return item, nil
		}
	}
	return Account{}, fmt.Errorf("%w: account %d", ErrNotFound, id)
}

func (s *MemoryStore) FindAccountByOpenID(_ context.Context, orgID uint64, openID string) (Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.accounts {
		if item.OrganizationID == orgID && item.OpenID == strings.TrimSpace(openID) {
			return item, nil
		}
	}
	return Account{}, fmt.Errorf("%w: account %s", ErrNotFound, openID)
}

func (s *MemoryStore) GetLatestPrivacyConsent(_ context.Context, orgID, parentID uint64) (PrivacyConsent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *PrivacyConsent
	for _, item := range s.privacyConsents {
		if item.OrganizationID != orgID || item.ParentAccountID != parentID {
			continue
		}
		if latest == nil || item.ConsentedAt.After(latest.ConsentedAt) || (item.ConsentedAt.Equal(latest.ConsentedAt) && item.ID > latest.ID) {
			copy := item
			latest = &copy
		}
	}
	if latest == nil {
		return PrivacyConsent{}, ErrNotFound
	}
	return *latest, nil
}

func (s *MemoryStore) RecordPrivacyConsent(_ context.Context, orgID, parentID uint64, params RecordPrivacyConsentParams) (PrivacyConsent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	version := strings.TrimSpace(params.PolicyVersion)
	for _, item := range s.privacyConsents {
		if item.OrganizationID == orgID && item.ParentAccountID == parentID && item.PolicyVersion == version {
			return item, nil
		}
	}
	now := time.Now().UTC()
	item := PrivacyConsent{ID: s.newID(), OrganizationID: orgID, ParentAccountID: parentID, PolicyVersion: version, ConsentedAt: now, CreatedAt: now}
	s.privacyConsents = append(s.privacyConsents, item)
	return item, nil
}

func (s *MemoryStore) ListAccountsForStudent(_ context.Context, orgID, studentID uint64) ([]Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	accountIDs := make(map[uint64]struct{})
	for _, binding := range s.bindings {
		if binding.OrganizationID == orgID && binding.StudentID == studentID && binding.Status == BindingStatusActive {
			accountIDs[binding.ParentAccountID] = struct{}{}
		}
	}
	out := make([]Account, 0, len(accountIDs))
	for _, account := range s.accounts {
		if account.OrganizationID == orgID && account.Status == AccountStatusActive {
			if _, ok := accountIDs[account.ID]; ok {
				out = append(out, account)
			}
		}
	}
	return out, nil
}

func (s *MemoryStore) ListMessageSubscriptions(_ context.Context, orgID, parentID uint64) ([]MessageSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MessageSubscription, 0)
	for _, item := range s.subscriptions {
		if item.ParentAccountID == parentID && item.OrganizationID == orgID {
			out = append(out, item)
		}
	}
	slices.SortFunc(out, func(left, right MessageSubscription) int {
		return strings.Compare(left.Kind, right.Kind)
	})
	return out, nil
}

func (s *MemoryStore) UpdateMessageSubscriptions(_ context.Context, orgID, parentID uint64, params []UpdateMessageSubscriptionParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, param := range params {
		kind, status := strings.TrimSpace(param.Kind), strings.TrimSpace(param.Status)
		if kind == "" || status == "" {
			return ErrInvalidStatus
		}
		found := false
		for index := range s.subscriptions {
			item := &s.subscriptions[index]
			if item.OrganizationID == orgID && item.ParentAccountID == parentID && item.Kind == kind {
				item.Status, item.UpdatedAt = status, now
				found = true
				break
			}
		}
		if !found {
			var authorizedAt *time.Time
			if status == "accept" {
				authorizedAt = &now
			}
			s.subscriptions = append(s.subscriptions, MessageSubscription{OrganizationID: orgID, ParentAccountID: parentID, Kind: kind, Status: status, TemplateVersion: strings.TrimSpace(param.TemplateVersion), AuthorizedAt: authorizedAt, UpdatedAt: now})
			continue
		}
		for index := range s.subscriptions {
			item := &s.subscriptions[index]
			if item.OrganizationID != orgID || item.ParentAccountID != parentID || item.Kind != kind {
				continue
			}
			item.TemplateVersion = strings.TrimSpace(param.TemplateVersion)
			if status == "accept" {
				item.AuthorizedAt = &now
			}
			break
		}
	}
	return nil
}

func (s *MemoryStore) CreateBinding(_ context.Context, orgID uint64, params BindStudentParams) (Binding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.bindings {
		if item.OrganizationID == orgID && item.ParentAccountID == params.ParentAccountID && item.StudentID == params.StudentID {
			return Binding{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := Binding{ID: s.newID(), OrganizationID: orgID, ParentAccountID: params.ParentAccountID, StudentID: params.StudentID, Relationship: strings.TrimSpace(params.Relationship), IsPrimary: params.IsPrimary, Status: BindingStatusActive, CreatedAt: now, UpdatedAt: now}
	s.bindings = append(s.bindings, item)
	return item, nil
}

func (s *MemoryStore) ListBindings(_ context.Context, orgID, parentID uint64) ([]Binding, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Binding, 0)
	for _, item := range s.bindings {
		if item.OrganizationID == orgID && item.ParentAccountID == parentID && item.Status == BindingStatusActive {
			out = append(out, item)
		}
	}
	slices.SortFunc(out, func(left, right Binding) int { return strings.Compare(left.StudentName, right.StudentName) })
	return out, nil
}

func (s *MemoryStore) CreateChildApplication(_ context.Context, orgID uint64, params CreateChildApplicationParams) (ChildApplication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.applications {
		if item.OrganizationID == orgID && item.ParentAccountID == params.ParentAccountID && item.StudentName == strings.TrimSpace(params.StudentName) && (item.Status == ChildApplicationStatusPending || item.Status == ChildApplicationStatusNeedsInfo || item.Status == ChildApplicationStatusApproved) && sameOptionalID(item.SchoolClassID, params.SchoolClassID) {
			return ChildApplication{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := ChildApplication{
		ID:              s.newID(),
		OrganizationID:  orgID,
		ParentAccountID: params.ParentAccountID,
		StudentName:     strings.TrimSpace(params.StudentName),
		SchoolNameInput: strings.TrimSpace(params.SchoolNameInput),
		GradeInput:      strings.TrimSpace(params.GradeInput),
		ClassNameInput:  strings.TrimSpace(params.ClassNameInput),
		SchoolID:        cloneID(params.SchoolID),
		SchoolClassID:   cloneID(params.SchoolClassID),
		Grade:           strings.TrimSpace(params.Grade),
		ClassName:       strings.TrimSpace(params.ClassName),
		GuardianName:    strings.TrimSpace(params.GuardianName),
		GuardianPhone:   strings.TrimSpace(params.GuardianPhone),
		Relationship:    strings.TrimSpace(params.Relationship),
		Notes:           strings.TrimSpace(params.Notes),
		Status:          ChildApplicationStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.applications = append(s.applications, item)
	return cloneApplication(item), nil
}

func (s *MemoryStore) UpdateChildApplication(_ context.Context, orgID uint64, params UpdateChildApplicationParams) (ChildApplication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.applications {
		if existing.OrganizationID == orgID && existing.ID != params.ID && existing.ParentAccountID == params.ParentAccountID && normalizeName(existing.StudentName) == normalizeName(params.StudentName) && (existing.Status == ChildApplicationStatusPending || existing.Status == ChildApplicationStatusNeedsInfo || existing.Status == ChildApplicationStatusApproved) && sameOptionalID(existing.SchoolClassID, params.SchoolClassID) {
			return ChildApplication{}, ErrConflict
		}
	}
	for index := range s.applications {
		item := &s.applications[index]
		if item.OrganizationID != orgID || item.ID != params.ID || item.ParentAccountID != params.ParentAccountID {
			continue
		}
		if item.Status != ChildApplicationStatusNeedsInfo {
			return ChildApplication{}, ErrInvalidState
		}
		now := time.Now().UTC()
		item.StudentName = strings.TrimSpace(params.StudentName)
		item.SchoolNameInput = strings.TrimSpace(params.SchoolNameInput)
		item.GradeInput = strings.TrimSpace(params.GradeInput)
		item.ClassNameInput = strings.TrimSpace(params.ClassNameInput)
		item.SchoolID = cloneID(params.SchoolID)
		item.SchoolClassID = cloneID(params.SchoolClassID)
		item.Grade = strings.TrimSpace(params.Grade)
		item.ClassName = strings.TrimSpace(params.ClassName)
		item.GuardianName = strings.TrimSpace(params.GuardianName)
		item.GuardianPhone = strings.TrimSpace(params.GuardianPhone)
		item.Relationship = strings.TrimSpace(params.Relationship)
		item.Notes = strings.TrimSpace(params.Notes)
		item.Status = ChildApplicationStatusPending
		item.ReviewNote = ""
		item.ReviewedByUserID = nil
		item.ReviewedAt = nil
		item.UpdatedAt = now
		return cloneApplication(*item), nil
	}
	return ChildApplication{}, fmt.Errorf("%w: child application %d", ErrNotFound, params.ID)
}

func (s *MemoryStore) GetChildApplication(_ context.Context, orgID, id uint64) (ChildApplication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.applications {
		if item.OrganizationID == orgID && item.ID == id {
			return cloneApplication(item), nil
		}
	}
	return ChildApplication{}, fmt.Errorf("%w: child application %d", ErrNotFound, id)
}

func (s *MemoryStore) ListChildApplications(_ context.Context, orgID uint64, parentID *uint64) ([]ChildApplication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ChildApplication, 0)
	for _, item := range s.applications {
		if item.OrganizationID != orgID || (parentID != nil && item.ParentAccountID != *parentID) {
			continue
		}
		out = append(out, cloneApplication(item))
	}
	slices.SortFunc(out, func(left, right ChildApplication) int {
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

func (s *MemoryStore) ReviewChildApplication(_ context.Context, orgID uint64, params ReviewChildApplicationParams) (ChildApplication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if params.Status != ChildApplicationStatusApproved && params.Status != ChildApplicationStatusRejected && params.Status != ChildApplicationStatusNeedsInfo {
		return ChildApplication{}, ErrInvalidStatus
	}
	for index := range s.applications {
		item := &s.applications[index]
		if item.OrganizationID != orgID || item.ID != params.ID {
			continue
		}
		if item.Status != ChildApplicationStatusPending && item.Status != ChildApplicationStatusNeedsInfo {
			return ChildApplication{}, ErrInvalidState
		}
		now := time.Now().UTC()
		item.Status = params.Status
		item.StudentID = cloneID(params.StudentID)
		item.SchoolID = cloneID(params.SchoolID)
		item.SchoolClassID = cloneID(params.SchoolClassID)
		item.ReviewNote = strings.TrimSpace(params.ReviewNote)
		item.ReviewedByUserID = cloneID(uint64Ptr(params.ReviewedByUserID))
		item.ReviewedAt = &now
		item.UpdatedAt = now
		return cloneApplication(*item), nil
	}
	return ChildApplication{}, fmt.Errorf("%w: child application %d", ErrNotFound, params.ID)
}

func (s *MemoryStore) CreateLeaveRequest(_ context.Context, orgID uint64, params CreateLeaveRequestParams) (LeaveRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	parentID := params.ParentAccountID
	item := LeaveRequest{ID: s.newID(), OrganizationID: orgID, StudentID: params.StudentID, ParentAccountID: &parentID, SubmittedByType: LeaveSubmittedByParent, LeaveDate: params.LeaveDate, Reason: strings.TrimSpace(params.Reason), Status: LeaveStatusPending, CreatedAt: now, UpdatedAt: now}
	s.leaves = append(s.leaves, item)
	return cloneLeave(item), nil
}

func (s *MemoryStore) CreateTeacherLeaveRequest(_ context.Context, orgID uint64, params CreateTeacherLeaveRequestParams) (LeaveRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.leaves {
		if existing.OrganizationID == orgID && existing.StudentID == params.StudentID && sameDay(existing.LeaveDate, params.LeaveDate) && existing.Status != LeaveStatusRejected && existing.Status != LeaveStatusCancelled {
			return LeaveRequest{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	submitter := params.SubmittedByUserID
	item := LeaveRequest{ID: s.newID(), OrganizationID: orgID, StudentID: params.StudentID, SubmittedByType: LeaveSubmittedByTeacher, SubmittedByUserID: &submitter, LeaveDate: params.LeaveDate, Reason: strings.TrimSpace(params.Reason), Status: LeaveStatusApproved, TeacherNote: "老师口头代记", ReviewedByUserID: &submitter, ReviewedAt: &now, CreatedAt: now, UpdatedAt: now}
	s.leaves = append(s.leaves, item)
	return cloneLeave(item), nil
}

func (s *MemoryStore) UpdateLeaveRequest(_ context.Context, orgID uint64, params UpdateLeaveRequestParams) (LeaveRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.leaves {
		item := &s.leaves[index]
		if item.OrganizationID != orgID || item.ID != params.ID || item.ParentAccountID == nil || *item.ParentAccountID != params.ParentAccountID {
			continue
		}
		if item.Status != LeaveStatusPending {
			return LeaveRequest{}, ErrInvalidState
		}
		item.LeaveDate, item.Reason, item.UpdatedAt = params.LeaveDate, strings.TrimSpace(params.Reason), time.Now().UTC()
		return cloneLeave(*item), nil
	}
	return LeaveRequest{}, fmt.Errorf("%w: leave request %d", ErrNotFound, params.ID)
}

func (s *MemoryStore) CancelLeaveRequest(_ context.Context, orgID uint64, params CancelLeaveRequestParams) (LeaveRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.leaves {
		item := &s.leaves[index]
		if item.OrganizationID != orgID || item.ID != params.ID || item.ParentAccountID == nil || *item.ParentAccountID != params.ParentAccountID {
			continue
		}
		if item.Status != LeaveStatusPending {
			return LeaveRequest{}, ErrInvalidState
		}
		item.Status, item.UpdatedAt = LeaveStatusCancelled, time.Now().UTC()
		return cloneLeave(*item), nil
	}
	return LeaveRequest{}, fmt.Errorf("%w: leave request %d", ErrNotFound, params.ID)
}

func (s *MemoryStore) ListLeaveRequests(_ context.Context, orgID uint64, parentID *uint64) ([]LeaveRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]LeaveRequest, 0)
	for _, item := range s.leaves {
		if item.OrganizationID != orgID || (parentID != nil && (item.ParentAccountID == nil || *item.ParentAccountID != *parentID)) {
			continue
		}
		out = append(out, cloneLeave(item))
	}
	slices.SortFunc(out, func(left, right LeaveRequest) int {
		if left.LeaveDate.Equal(right.LeaveDate) {
			if left.ID > right.ID {
				return -1
			}
			if left.ID < right.ID {
				return 1
			}
			return 0
		}
		if left.LeaveDate.After(right.LeaveDate) {
			return -1
		}
		return 1
	})
	return out, nil
}

func (s *MemoryStore) ListApprovedLeaveStudentIDs(_ context.Context, orgID uint64, leaveDate time.Time) (map[uint64]struct{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[uint64]struct{})
	for _, item := range s.leaves {
		if item.OrganizationID == orgID && item.Status == LeaveStatusApproved && sameDay(item.LeaveDate, leaveDate) {
			out[item.StudentID] = struct{}{}
		}
	}
	return out, nil
}

func (s *MemoryStore) ReviewLeaveRequest(_ context.Context, orgID uint64, params ReviewLeaveRequestParams) (LeaveRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if params.Status != LeaveStatusApproved && params.Status != LeaveStatusRejected {
		return LeaveRequest{}, ErrInvalidStatus
	}
	for index := range s.leaves {
		item := &s.leaves[index]
		if item.OrganizationID != orgID || item.ID != params.ID {
			continue
		}
		if item.Status != LeaveStatusPending {
			return LeaveRequest{}, ErrInvalidState
		}
		now := time.Now().UTC()
		reviewer := params.ReviewedByUserID
		item.Status, item.TeacherNote, item.ReviewedByUserID, item.ReviewedAt, item.UpdatedAt = params.Status, strings.TrimSpace(params.TeacherNote), &reviewer, &now, now
		return cloneLeave(*item), nil
	}
	return LeaveRequest{}, fmt.Errorf("%w: leave request %d", ErrNotFound, params.ID)
}

func cloneLeave(item LeaveRequest) LeaveRequest {
	if item.ParentAccountID != nil {
		value := *item.ParentAccountID
		item.ParentAccountID = &value
	}
	if item.SubmittedByUserID != nil {
		value := *item.SubmittedByUserID
		item.SubmittedByUserID = &value
	}
	if item.ReviewedByUserID != nil {
		value := *item.ReviewedByUserID
		item.ReviewedByUserID = &value
	}
	if item.ReviewedAt != nil {
		value := *item.ReviewedAt
		item.ReviewedAt = &value
	}
	return item
}

func cloneApplication(item ChildApplication) ChildApplication {
	item.StudentID = cloneID(item.StudentID)
	item.SchoolID = cloneID(item.SchoolID)
	item.SchoolClassID = cloneID(item.SchoolClassID)
	item.ReviewedByUserID = cloneID(item.ReviewedByUserID)
	if item.ReviewedAt != nil {
		value := *item.ReviewedAt
		item.ReviewedAt = &value
	}
	return item
}

func cloneID(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func uint64Ptr(value uint64) *uint64 { return &value }

func sameOptionalID(left, right *uint64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
