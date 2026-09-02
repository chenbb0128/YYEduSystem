package pickup

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

// MemoryStore is used when MySQL is disabled, so local UI work can proceed without infrastructure.
type MemoryStore struct {
	mu               sync.RWMutex
	nextID           uint64
	operations       []Operation
	members          []OperationStudent
	events           []Event
	notifications    []Notification
	outbox           []NotificationOutbox
	deliveryLogs     []NotificationDeliveryLog
	changeRequests   []PickupChangeRequest
	handoffs         []Handoff
	notificationHook NotificationHook
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{nextID: 1} }

func (s *MemoryStore) newID() uint64 {
	id := s.nextID
	s.nextID++
	return id
}

func (s *MemoryStore) ListOperations(_ context.Context, orgID uint64) ([]Operation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Operation, 0, len(s.operations))
	for _, item := range s.operations {
		if item.OrganizationID == orgID {
			out = append(out, cloneOperation(item))
		}
	}
	return out, nil
}

func (s *MemoryStore) FindOperation(_ context.Context, orgID, id uint64) (Operation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.operations {
		if item.OrganizationID == orgID && item.ID == id {
			return cloneOperation(item), nil
		}
	}
	return Operation{}, fmt.Errorf("%w: operation %d", ErrNotFound, id)
}

func (s *MemoryStore) CreateOperation(_ context.Context, orgID uint64, params CreateOperationParams, roster []StudentRef) (Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.operations {
		if item.OrganizationID == orgID && sameDay(item.OperationDate, params.OperationDate) && item.SchoolClassID == params.SchoolClassID && item.Status != OperationStatusCancelled {
			return Operation{}, ErrConflict
		}
	}
	if len(roster) == 0 {
		return Operation{}, fmt.Errorf("%w: roster is empty", ErrInvalidState)
	}
	now := time.Now().UTC()
	operation := Operation{ID: s.newID(), OrganizationID: orgID, OperationDate: params.OperationDate, PickupMode: params.PickupMode, SchoolID: params.SchoolID, SchoolClassID: params.SchoolClassID, CareClassID: cloneID(params.CareClassID), TeacherUserID: cloneID(params.TeacherUserID), TeacherName: strings.TrimSpace(params.TeacherName), Status: OperationStatusDraft, Notes: strings.TrimSpace(params.Notes), CreatedAt: now, UpdatedAt: now}
	s.operations = append(s.operations, operation)
	seen := make(map[uint64]struct{}, len(roster))
	for _, student := range roster {
		if student.ID == 0 {
			continue
		}
		if _, exists := seen[student.ID]; exists {
			continue
		}
		seen[student.ID] = struct{}{}
		s.members = append(s.members, OperationStudent{ID: s.newID(), OrganizationID: orgID, OperationID: operation.ID, StudentID: student.ID, StudentName: student.Name, Status: MemberStatusPlanned, CreatedAt: now, UpdatedAt: now})
	}
	if len(seen) == 0 {
		return Operation{}, fmt.Errorf("%w: roster is empty", ErrInvalidState)
	}
	return cloneOperation(operation), nil
}

func (s *MemoryStore) SetOperationStatus(_ context.Context, orgID uint64, params SetOperationStatusParams) (Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.operations {
		item := &s.operations[index]
		if item.OrganizationID != orgID || item.ID != params.ID {
			continue
		}
		now := time.Now().UTC()
		switch {
		case (item.Status == OperationStatusDraft || item.Status == OperationStatusConfirmed) && params.Status == OperationStatusStarted:
			item.Status, item.StartedAt = params.Status, &now
		case item.Status == OperationStatusStarted && params.Status == OperationStatusFinished:
			for _, member := range s.members {
				if member.OperationID == item.ID && !IsReadyToFinish(member.Status) {
					return Operation{}, fmt.Errorf("%w: student %s is not ready to finish", ErrInvalidState, member.StudentName)
				}
			}
			item.Status, item.FinishedAt = params.Status, &now
		default:
			return Operation{}, fmt.Errorf("%w: %s -> %s", ErrInvalidState, item.Status, params.Status)
		}
		item.UpdatedAt = now
		return cloneOperation(*item), nil
	}
	return Operation{}, fmt.Errorf("%w: operation %d", ErrNotFound, params.ID)
}

func (s *MemoryStore) ConfirmOperation(_ context.Context, orgID uint64, params ConfirmOperationParams) (Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.operationLocked(orgID, params.ID)
	if !ok {
		return Operation{}, fmt.Errorf("%w: operation %d", ErrNotFound, params.ID)
	}
	if item.Status != OperationStatusDraft {
		return Operation{}, fmt.Errorf("%w: operation is %s", ErrInvalidState, item.Status)
	}
	now := time.Now().UTC()
	item.Status = OperationStatusConfirmed
	item.ConfirmedAt = &now
	item.ConfirmedByUserID = cloneID(params.ConfirmedByUserID)
	item.ConfirmedByName = strings.TrimSpace(params.ConfirmedByName)
	item.ExecutingTeacherUserID = cloneID(params.ExecutingTeacherUserID)
	item.ExecutingTeacherName = strings.TrimSpace(params.ExecutingTeacherName)
	item.TeacherRole = defaultRole(params.TeacherRole)
	item.ExpectedPickupTime = strings.TrimSpace(params.ExpectedPickupTime)
	if strings.TrimSpace(params.Notes) != "" {
		item.Notes = strings.TrimSpace(params.Notes)
	}
	item.UpdatedAt = now
	return cloneOperation(*item), nil
}

func (s *MemoryStore) HandoffOperation(_ context.Context, orgID uint64, params HandoffOperationParams) (Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.operationLocked(orgID, params.ID)
	if !ok {
		return Operation{}, fmt.Errorf("%w: operation %d", ErrNotFound, params.ID)
	}
	if item.Status != OperationStatusStarted {
		return Operation{}, fmt.Errorf("%w: operation is %s", ErrInvalidState, item.Status)
	}
	if params.ToTeacherUserID == 0 || strings.TrimSpace(params.ToTeacherName) == "" {
		return Operation{}, fmt.Errorf("%w: handoff teacher is incomplete", ErrInvalidState)
	}
	now := time.Now().UTC()
	fromID := cloneID(item.ExecutingTeacherUserID)
	fromName := strings.TrimSpace(item.ExecutingTeacherName)
	item.ExecutingTeacherUserID = cloneID(&params.ToTeacherUserID)
	item.ExecutingTeacherName = strings.TrimSpace(params.ToTeacherName)
	item.TeacherRole = defaultRole(params.TeacherRole)
	item.UpdatedAt = now
	s.handoffs = append(s.handoffs, Handoff{ID: s.newID(), OrganizationID: orgID, OperationID: item.ID, FromTeacherUserID: fromID, FromTeacherName: fromName, ToTeacherUserID: cloneID(&params.ToTeacherUserID), ToTeacherName: strings.TrimSpace(params.ToTeacherName), TeacherRole: item.TeacherRole, Note: strings.TrimSpace(params.Note), HandoffAt: now, CreatedByUserID: cloneID(params.CreatedByUserID), CreatedByName: strings.TrimSpace(params.CreatedByName)})
	return cloneOperation(*item), nil
}

func (s *MemoryStore) ListHandoffs(_ context.Context, orgID, operationID uint64) ([]Handoff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.operationExistsLocked(orgID, operationID) {
		return nil, fmt.Errorf("%w: operation %d", ErrNotFound, operationID)
	}
	out := make([]Handoff, 0)
	for index := len(s.handoffs) - 1; index >= 0; index-- {
		item := s.handoffs[index]
		if item.OrganizationID == orgID && item.OperationID == operationID {
			out = append(out, cloneHandoff(item))
		}
	}
	return out, nil
}

func (s *MemoryStore) ListOperationStudents(_ context.Context, orgID, operationID uint64) ([]OperationStudent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]OperationStudent, 0)
	for _, item := range s.members {
		if item.OrganizationID == orgID && item.OperationID == operationID {
			out = append(out, cloneOperationStudent(item))
		}
	}
	if len(out) == 0 {
		if !s.operationExistsLocked(orgID, operationID) {
			return nil, fmt.Errorf("%w: operation %d", ErrNotFound, operationID)
		}
	}
	return out, nil
}

func (s *MemoryStore) AddOperationStudent(_ context.Context, orgID uint64, params AddOperationStudentParams) (OperationStudent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation, ok := s.operationLocked(orgID, params.OperationID)
	if !ok {
		return OperationStudent{}, fmt.Errorf("%w: operation %d", ErrNotFound, params.OperationID)
	}
	if operation.Status == OperationStatusFinished || operation.Status == OperationStatusCancelled {
		return OperationStudent{}, fmt.Errorf("%w: operation is %s", ErrInvalidState, operation.Status)
	}
	if params.StudentID == 0 || strings.TrimSpace(params.StudentName) == "" {
		return OperationStudent{}, fmt.Errorf("%w: temporary student is incomplete", ErrInvalidState)
	}
	for _, item := range s.members {
		if item.OrganizationID == orgID && item.OperationID == params.OperationID && item.StudentID == params.StudentID {
			return OperationStudent{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := OperationStudent{ID: s.newID(), OrganizationID: orgID, OperationID: params.OperationID, StudentID: params.StudentID, StudentName: strings.TrimSpace(params.StudentName), Status: MemberStatusPlanned, Note: strings.TrimSpace(params.Note), IsTemporary: params.IsTemporary, ProfilePending: params.ProfilePending, PickupMode: defaultPickupMode(params.PickupMode, operation.PickupMode), CreatedAt: now, UpdatedAt: now}
	s.members = append(s.members, item)
	return cloneOperationStudent(item), nil
}

func (s *MemoryStore) CompleteOperationStudentProfile(_ context.Context, orgID, operationID, studentID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.members {
		item := &s.members[index]
		if item.OrganizationID != orgID || item.OperationID != operationID || item.StudentID != studentID {
			continue
		}
		if !item.IsTemporary {
			return nil
		}
		item.ProfilePending = false
		item.UpdatedAt = time.Now().UTC()
		return nil
	}
	return fmt.Errorf("%w: student %d", ErrNotFound, studentID)
}

func (s *MemoryStore) MarkOperationStudent(ctx context.Context, orgID uint64, params MarkStudentParams) (OperationStudent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation, ok := s.operationLocked(orgID, params.OperationID)
	if !ok {
		return OperationStudent{}, fmt.Errorf("%w: operation %d", ErrNotFound, params.OperationID)
	}
	if operation.Status != OperationStatusStarted {
		return OperationStudent{}, fmt.Errorf("%w: operation is %s", ErrInvalidState, operation.Status)
	}
	if !IsValidMemberStatus(params.Status) {
		return OperationStudent{}, fmt.Errorf("%w: %s", ErrInvalidStatus, params.Status)
	}
	for index := range s.members {
		item := &s.members[index]
		if item.OrganizationID != orgID || item.OperationID != params.OperationID || item.StudentID != params.StudentID {
			continue
		}
		if !IsValidMemberTransition(item.Status, params.Status) {
			return OperationStudent{}, fmt.Errorf("%w: %s -> %s", ErrInvalidState, item.Status, params.Status)
		}
		if item.Status == params.Status {
			photoURL := strings.TrimSpace(params.PhotoURL)
			if photoURL != "" && photoURL != item.PhotoURL {
				now := time.Now().UTC()
				item.PhotoURL = photoURL
				item.UpdatedAt = now
				s.events = append(s.events, Event{ID: s.newID(), OrganizationID: orgID, OperationID: item.OperationID, OperationStudentID: item.ID, StudentID: item.StudentID, EventType: params.Status, EventAt: now, OperatorName: strings.TrimSpace(params.OperatorName), PhotoURL: photoURL, Note: "补传照片"})
			}
			return cloneOperationStudent(*item), nil
		}
		now := time.Now().UTC()
		item.Status, item.CheckedAt, item.Note, item.UpdatedAt = params.Status, &now, strings.TrimSpace(params.Note), now
		if strings.TrimSpace(params.PhotoURL) != "" {
			item.PhotoURL = strings.TrimSpace(params.PhotoURL)
		}
		event := Event{ID: s.newID(), OrganizationID: orgID, OperationID: item.OperationID, OperationStudentID: item.ID, StudentID: item.StudentID, EventType: params.Status, EventAt: now, OperatorName: strings.TrimSpace(params.OperatorName), PhotoURL: item.PhotoURL, Note: item.Note}
		s.events = append(s.events, event)
		operationID := item.OperationID
		notification := Notification{ID: s.newID(), OrganizationID: orgID, StudentID: item.StudentID, OperationID: &operationID, EventID: &event.ID, RecipientType: "parent", Kind: "pickup_status", Title: pickupNotificationTitle(params.Status), Content: fmt.Sprintf("%s：%s", item.StudentName, pickupNotificationContent(params.Status)), Status: "pending", CreatedAt: now}
		s.notifications = append(s.notifications, notification)
		s.appendNotificationOutboxLocked(notification, now)
		if s.notificationHook != nil {
			s.notificationHook(context.WithoutCancel(ctx), cloneNotification(notification))
		}
		return cloneOperationStudent(*item), nil
	}
	return OperationStudent{}, fmt.Errorf("%w: student %d", ErrNotFound, params.StudentID)
}

func (s *MemoryStore) CorrectOperationEvent(ctx context.Context, orgID uint64, params CorrectEventParams) (OperationStudent, error) {
	if params.Status == MemberStatusPlanned || !IsValidMemberStatus(params.Status) || strings.TrimSpace(params.Reason) == "" {
		return OperationStudent{}, fmt.Errorf("%w: correction is invalid", ErrInvalidState)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	operation, ok := s.operationLocked(orgID, params.OperationID)
	if !ok {
		return OperationStudent{}, fmt.Errorf("%w: operation %d", ErrNotFound, params.OperationID)
	}
	if operation.Status == OperationStatusCancelled {
		return OperationStudent{}, fmt.Errorf("%w: operation is cancelled", ErrInvalidState)
	}
	var original *Event
	for index := range s.events {
		item := &s.events[index]
		if item.OrganizationID == orgID && item.OperationID == params.OperationID && item.ID == params.EventID {
			original = item
			break
		}
	}
	if original == nil {
		return OperationStudent{}, fmt.Errorf("%w: event %d", ErrNotFound, params.EventID)
	}
	for index := range s.members {
		member := &s.members[index]
		if member.OrganizationID != orgID || member.OperationID != params.OperationID || member.StudentID != original.StudentID {
			continue
		}
		now := time.Now().UTC()
		member.Status, member.CheckedAt, member.Note, member.UpdatedAt = params.Status, &now, strings.TrimSpace(params.Reason), now
		correction := Event{ID: s.newID(), OrganizationID: orgID, OperationID: params.OperationID, OperationStudentID: member.ID, StudentID: member.StudentID, EventType: "correction", EventAt: now, OperatorName: strings.TrimSpace(params.OperatorName), PhotoURL: member.PhotoURL, Note: fmt.Sprintf("更正事件 #%d 为 %s：%s", original.ID, params.Status, strings.TrimSpace(params.Reason))}
		s.events = append(s.events, correction)
		operationID := params.OperationID
		notification := Notification{ID: s.newID(), OrganizationID: orgID, StudentID: member.StudentID, OperationID: &operationID, EventID: &correction.ID, RecipientType: "parent", Kind: "pickup_status", Title: "接送记录已更正", Content: fmt.Sprintf("%s：接送状态已更正为 %s；原因：%s", member.StudentName, params.Status, strings.TrimSpace(params.Reason)), Status: "pending", CreatedAt: now}
		s.notifications = append(s.notifications, notification)
		s.appendNotificationOutboxLocked(notification, now)
		if s.notificationHook != nil {
			s.notificationHook(context.WithoutCancel(ctx), cloneNotification(notification))
		}
		return cloneOperationStudent(*member), nil
	}
	return OperationStudent{}, fmt.Errorf("%w: student %d", ErrNotFound, original.StudentID)
}

func (s *MemoryStore) ListEvents(_ context.Context, orgID, operationID uint64) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, 0)
	for _, item := range s.events {
		if item.OrganizationID == orgID && item.OperationID == operationID {
			out = append(out, item)
		}
	}
	slices.Reverse(out)
	return out, nil
}

func (s *MemoryStore) ListNotifications(_ context.Context, orgID uint64) ([]Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Notification, 0)
	for _, item := range s.notifications {
		if item.OrganizationID == orgID {
			out = append(out, cloneNotification(item))
		}
	}
	slices.Reverse(out)
	return out, nil
}

func (s *MemoryStore) FindNotification(_ context.Context, orgID, id uint64) (Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.notifications {
		if item.OrganizationID == orgID && item.ID == id {
			return cloneNotification(item), nil
		}
	}
	return Notification{}, fmt.Errorf("%w: notification %d", ErrNotFound, id)
}

func (s *MemoryStore) CreateNotification(ctx context.Context, orgID uint64, params CreateNotificationParams) (Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	item := Notification{ID: s.newID(), OrganizationID: orgID, StudentID: params.StudentID, OperationID: cloneID(params.OperationID), EventID: cloneID(params.EventID), RecipientType: "parent", Kind: strings.TrimSpace(params.Kind), Title: strings.TrimSpace(params.Title), Content: strings.TrimSpace(params.Content), Status: "pending", CreatedAt: now}
	s.notifications = append(s.notifications, item)
	s.appendNotificationOutboxLocked(item, now)
	if s.notificationHook != nil {
		s.notificationHook(context.WithoutCancel(ctx), cloneNotification(item))
	}
	return cloneNotification(item), nil
}

func (s *MemoryStore) ListNotificationOutbox(_ context.Context, now, staleBefore time.Time, limit int) ([]NotificationOutbox, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	out := make([]NotificationOutbox, 0, limit)
	for _, item := range s.outbox {
		if item.Status != "pending" && item.Status != "failed" {
			continue
		}
		if item.AvailableAt.After(now) || (item.LockedAt != nil && item.LockedAt.After(staleBefore)) {
			continue
		}
		out = append(out, cloneOutbox(item))
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *MemoryStore) ClaimNotificationOutbox(_ context.Context, orgID, id uint64, claimedAt time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.outbox {
		item := &s.outbox[index]
		if item.OrganizationID != orgID || item.ID != id || (item.Status != "pending" && item.Status != "failed") {
			continue
		}
		item.Status = "processing"
		item.Attempts++
		item.LockedAt = cloneTime(&claimedAt)
		item.UpdatedAt = claimedAt
		return true, nil
	}
	return false, nil
}

func (s *MemoryStore) CompleteNotificationOutbox(_ context.Context, orgID, id uint64, status string, availableAt *time.Time, lastError string) error {
	if status != "pending" && status != "processed" && status != "failed" {
		return ErrInvalidState
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.outbox {
		item := &s.outbox[index]
		if item.OrganizationID != orgID || item.ID != id {
			continue
		}
		now := time.Now().UTC()
		item.Status, item.LockedAt, item.LastError, item.UpdatedAt = status, nil, strings.TrimSpace(lastError), now
		item.AvailableAt = now
		if availableAt != nil {
			item.AvailableAt = *availableAt
		}
		if status == "processed" {
			item.ProcessedAt = &now
		}
		return nil
	}
	return fmt.Errorf("%w: outbox %d", ErrNotFound, id)
}

func (s *MemoryStore) CreateNotificationDeliveryLog(_ context.Context, orgID uint64, params CreateDeliveryLogParams) (NotificationDeliveryLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.deliveryLogs {
		if item.OrganizationID == orgID && item.NotificationID == params.NotificationID && item.ParentAccountID == params.ParentAccountID && item.MessageKind == params.MessageKind && item.TemplateID == params.TemplateID {
			return cloneDeliveryLog(item), nil
		}
	}
	now := time.Now().UTC()
	item := NotificationDeliveryLog{ID: s.newID(), OrganizationID: orgID, NotificationID: params.NotificationID, ParentAccountID: params.ParentAccountID, MessageKind: strings.TrimSpace(params.MessageKind), TemplateID: strings.TrimSpace(params.TemplateID), Status: "pending", CreatedAt: now, UpdatedAt: now}
	s.deliveryLogs = append(s.deliveryLogs, item)
	return cloneDeliveryLog(item), nil
}

func (s *MemoryStore) SetNotificationDeliveryLogStatus(_ context.Context, orgID uint64, params SetDeliveryLogStatusParams) error {
	if params.Status != "pending" && params.Status != "sent" && params.Status != "failed" && params.Status != "skipped" {
		return ErrInvalidState
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.deliveryLogs {
		item := &s.deliveryLogs[index]
		if item.OrganizationID != orgID || item.ID != params.ID {
			continue
		}
		item.Status, item.Attempts, item.LastAttemptAt, item.SentAt, item.NextRetryAt, item.DeliveryError, item.UpdatedAt = params.Status, params.Attempts, cloneTime(params.LastAttemptAt), cloneTime(params.SentAt), cloneTime(params.NextRetryAt), strings.TrimSpace(params.DeliveryError), time.Now().UTC()
		return nil
	}
	return fmt.Errorf("%w: delivery log %d", ErrNotFound, params.ID)
}

func (s *MemoryStore) ListNotificationDeliveryLogs(_ context.Context, orgID uint64, notificationID *uint64, status string) ([]NotificationDeliveryLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]NotificationDeliveryLog, 0)
	for _, item := range s.deliveryLogs {
		if item.OrganizationID != orgID || (notificationID != nil && item.NotificationID != *notificationID) || (strings.TrimSpace(status) != "" && item.Status != strings.TrimSpace(status)) {
			continue
		}
		out = append(out, cloneDeliveryLog(item))
	}
	slices.Reverse(out)
	return out, nil
}

func (s *MemoryStore) RetryNotification(_ context.Context, orgID, notificationID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var notification *Notification
	for index := range s.notifications {
		if s.notifications[index].OrganizationID == orgID && s.notifications[index].ID == notificationID {
			notification = &s.notifications[index]
			break
		}
	}
	if notification == nil {
		return fmt.Errorf("%w: notification %d", ErrNotFound, notificationID)
	}
	for index := range s.outbox {
		item := &s.outbox[index]
		if item.OrganizationID != orgID || item.NotificationID != notificationID {
			continue
		}
		if item.Status != "failed" {
			return fmt.Errorf("%w: notification is not failed", ErrInvalidState)
		}
		now := time.Now().UTC()
		item.Status, item.Attempts, item.AvailableAt, item.LockedAt, item.ProcessedAt, item.LastError, item.UpdatedAt = "pending", 0, now, nil, nil, "", now
		notification.Status, notification.DeliveryAttempts, notification.LastAttemptAt, notification.NextRetryAt, notification.DeliveryError, notification.SentAt = "pending", 0, nil, nil, "", nil
		return nil
	}
	return fmt.Errorf("%w: notification outbox %d", ErrNotFound, notificationID)
}

func (s *MemoryStore) SetNotificationStatus(_ context.Context, orgID uint64, params SetNotificationStatusParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if params.Status != "pending" && params.Status != "sent" && params.Status != "failed" {
		return ErrInvalidState
	}
	for index := range s.notifications {
		item := &s.notifications[index]
		if item.OrganizationID != orgID || item.ID != params.ID {
			continue
		}
		item.Status = params.Status
		item.SentAt = cloneTime(params.SentAt)
		item.DeliveryAttempts = params.DeliveryAttempts
		item.LastAttemptAt = cloneTime(params.LastAttemptAt)
		item.DeliveryError = params.DeliveryError
		item.NextRetryAt = cloneTime(params.NextRetryAt)
		return nil
	}
	return fmt.Errorf("%w: notification %d", ErrNotFound, params.ID)
}

func (s *MemoryStore) MarkNotificationRead(_ context.Context, orgID, id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.notifications {
		item := &s.notifications[index]
		if item.OrganizationID != orgID || item.ID != id {
			continue
		}
		now := time.Now().UTC()
		item.ReadAt = &now
		return nil
	}
	return fmt.Errorf("%w: notification %d", ErrNotFound, id)
}

func (s *MemoryStore) CreatePickupChangeRequest(_ context.Context, orgID uint64, params CreatePickupChangeRequestParams) (PickupChangeRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if params.StudentID == 0 || strings.TrimSpace(params.RequestedStatus) == "" {
		return PickupChangeRequest{}, fmt.Errorf("%w: invalid change request", ErrInvalidState)
	}
	for _, item := range s.changeRequests {
		if item.OrganizationID == orgID && item.StudentID == params.StudentID && sameDay(item.ChangeDate, params.ChangeDate) && item.Status == ChangeRequestStatusPending {
			return PickupChangeRequest{}, ErrConflict
		}
	}
	now := time.Now().UTC()
	item := PickupChangeRequest{ID: s.newID(), OrganizationID: orgID, StudentID: params.StudentID, OperationID: cloneID(params.OperationID), ChangeDate: params.ChangeDate, RequestedStatus: strings.TrimSpace(params.RequestedStatus), Note: strings.TrimSpace(params.Note), SubmittedBy: strings.TrimSpace(params.SubmittedBy), Status: ChangeRequestStatusPending, CreatedAt: now, UpdatedAt: now}
	s.changeRequests = append(s.changeRequests, item)
	return cloneChangeRequest(item), nil
}

func (s *MemoryStore) ListPickupChangeRequests(_ context.Context, orgID uint64, date *time.Time, status string) ([]PickupChangeRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PickupChangeRequest, 0)
	for _, item := range s.changeRequests {
		if item.OrganizationID != orgID || (date != nil && !sameDay(item.ChangeDate, *date)) || (strings.TrimSpace(status) != "" && item.Status != status) {
			continue
		}
		out = append(out, cloneChangeRequest(item))
	}
	slices.Reverse(out)
	return out, nil
}

func (s *MemoryStore) ReviewPickupChangeRequest(_ context.Context, orgID uint64, params ReviewPickupChangeRequestParams) (PickupChangeRequest, error) {
	if params.Status != ChangeRequestStatusApproved && params.Status != ChangeRequestStatusRejected {
		return PickupChangeRequest{}, ErrInvalidStatus
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.changeRequests {
		item := &s.changeRequests[index]
		if item.OrganizationID != orgID || item.ID != params.ID {
			continue
		}
		if item.Status != ChangeRequestStatusPending {
			return PickupChangeRequest{}, fmt.Errorf("%w: request is %s", ErrInvalidState, item.Status)
		}
		now := time.Now().UTC()
		item.Status, item.ReviewedByUserID, item.ReviewedAt, item.ReviewNote, item.UpdatedAt = params.Status, cloneID(params.ReviewedByUserID), &now, strings.TrimSpace(params.ReviewNote), now
		return cloneChangeRequest(*item), nil
	}
	return PickupChangeRequest{}, fmt.Errorf("%w: change request %d", ErrNotFound, params.ID)
}

func (s *MemoryStore) SetNotificationHook(hook NotificationHook) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notificationHook = hook
}

func (s *MemoryStore) appendNotificationOutboxLocked(notification Notification, now time.Time) {
	s.outbox = append(s.outbox, NotificationOutbox{ID: s.newID(), OrganizationID: notification.OrganizationID, EventType: "notification.created", AggregateType: "notification", AggregateID: notification.ID, NotificationID: notification.ID, Status: "pending", AvailableAt: now, CreatedAt: now, UpdatedAt: now})
}

func (s *MemoryStore) operationLocked(orgID, id uint64) (*Operation, bool) {
	for index := range s.operations {
		if s.operations[index].OrganizationID == orgID && s.operations[index].ID == id {
			return &s.operations[index], true
		}
	}
	return nil, false
}

func (s *MemoryStore) operationExistsLocked(orgID, id uint64) bool {
	_, ok := s.operationLocked(orgID, id)
	return ok
}

func sameDay(left, right time.Time) bool {
	left, right = left.UTC(), right.UTC()
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}

func pickupNotificationTitle(status string) string {
	switch status {
	case MemberStatusPickedUp:
		return "孩子已在校门口接到"
	case MemberStatusSelfArrived:
		return "孩子已到托管班"
	case MemberStatusParentPickedUp:
		return "孩子已由家长接走"
	case MemberStatusLeave:
		return "孩子今日请假"
	default:
		return "孩子今日未到托管班"
	}
}

func pickupNotificationContent(status string) string {
	switch status {
	case MemberStatusPickedUp:
		return "老师已在学校门口接到孩子，照片已记录。"
	case MemberStatusSelfArrived:
		return "孩子已自行到达托管班。"
	case MemberStatusParentPickedUp:
		return "孩子已登记为家长临时接走。"
	case MemberStatusLeave:
		return "孩子已登记今日请假。"
	default:
		return "孩子已登记为今日未到托管班。"
	}
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
func cloneOperation(item Operation) Operation {
	item.CareClassID, item.TeacherUserID, item.StartedAt, item.FinishedAt = cloneID(item.CareClassID), cloneID(item.TeacherUserID), cloneTime(item.StartedAt), cloneTime(item.FinishedAt)
	item.ConfirmedAt, item.ConfirmedByUserID, item.ExecutingTeacherUserID = cloneTime(item.ConfirmedAt), cloneID(item.ConfirmedByUserID), cloneID(item.ExecutingTeacherUserID)
	return item
}
func cloneOperationStudent(item OperationStudent) OperationStudent {
	item.CheckedAt = cloneTime(item.CheckedAt)
	return item
}

func cloneHandoff(item Handoff) Handoff {
	item.FromTeacherUserID = cloneID(item.FromTeacherUserID)
	item.ToTeacherUserID = cloneID(item.ToTeacherUserID)
	item.CreatedByUserID = cloneID(item.CreatedByUserID)
	return item
}
func cloneNotification(item Notification) Notification {
	item.OperationID, item.EventID, item.SentAt, item.ReadAt = cloneID(item.OperationID), cloneID(item.EventID), cloneTime(item.SentAt), cloneTime(item.ReadAt)
	return item
}

func cloneOutbox(item NotificationOutbox) NotificationOutbox {
	item.LockedAt, item.ProcessedAt = cloneTime(item.LockedAt), cloneTime(item.ProcessedAt)
	return item
}

func cloneDeliveryLog(item NotificationDeliveryLog) NotificationDeliveryLog {
	item.LastAttemptAt, item.SentAt, item.NextRetryAt = cloneTime(item.LastAttemptAt), cloneTime(item.SentAt), cloneTime(item.NextRetryAt)
	return item
}

func cloneChangeRequest(item PickupChangeRequest) PickupChangeRequest {
	item.OperationID, item.ReviewedByUserID, item.ReviewedAt = cloneID(item.OperationID), cloneID(item.ReviewedByUserID), cloneTime(item.ReviewedAt)
	return item
}

func defaultRole(value string) string {
	switch strings.TrimSpace(value) {
	case "lead", "collaborator", "substitute":
		return strings.TrimSpace(value)
	default:
		return "lead"
	}
}

func defaultPickupMode(value, fallback string) string {
	if strings.TrimSpace(value) == "school_pickup" || strings.TrimSpace(value) == "self_arrival" || strings.TrimSpace(value) == "parent_picked_up" {
		return strings.TrimSpace(value)
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "school_pickup"
}
