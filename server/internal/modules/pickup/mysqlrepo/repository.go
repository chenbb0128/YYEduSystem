package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database/sqlc"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct {
	queries          *sqlc.Queries
	exec             database.DBTX
	notificationHook pickup.NotificationHook
}

func (r *Repository) SetNotificationHook(hook pickup.NotificationHook) {
	r.notificationHook = hook
}

func New(exec database.DBTX) *Repository { return &Repository{queries: sqlc.New(exec), exec: exec} }

func (r *Repository) ListOperations(ctx context.Context, orgID uint64) ([]pickup.Operation, error) {
	items, err := r.queries.ListPickupOperations(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]pickup.Operation, 0, len(items))
	for _, item := range items {
		out = append(out, mapOperation(item))
	}
	return out, nil
}

func (r *Repository) FindOperation(ctx context.Context, orgID, id uint64) (pickup.Operation, error) {
	item, err := r.queries.GetPickupOperationByID(ctx, sqlc.GetPickupOperationByIDParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return pickup.Operation{}, translateError(err)
	}
	return mapOperation(item), nil
}

func (r *Repository) CreateOperation(ctx context.Context, orgID uint64, params pickup.CreateOperationParams, roster []pickup.StudentRef) (pickup.Operation, error) {
	if len(roster) == 0 {
		return pickup.Operation{}, pickup.ErrInvalidState
	}
	var operationID uint64
	err := r.withTransaction(ctx, func(q *sqlc.Queries) error {
		result, err := q.CreatePickupOperation(ctx, sqlc.CreatePickupOperationParams{OrganizationID: orgID, OperationDate: params.OperationDate, PickupMode: params.PickupMode, SchoolID: params.SchoolID, SchoolClassID: params.SchoolClassID, CareClassID: nullID(params.CareClassID), TeacherUserID: nullID(params.TeacherUserID), TeacherName: params.TeacherName, ExpectedPickupTime: strings.TrimSpace(params.ExpectedPickupTime), Notes: params.Notes})
		if err != nil {
			return translateError(err)
		}
		operationID, err = insertedID(result)
		if err != nil {
			return err
		}
		seen := make(map[uint64]struct{}, len(roster))
		for _, student := range roster {
			if student.ID == 0 {
				continue
			}
			if _, exists := seen[student.ID]; exists {
				continue
			}
			seen[student.ID] = struct{}{}
			if _, err := q.CreatePickupOperationStudent(ctx, sqlc.CreatePickupOperationStudentParams{OrganizationID: orgID, OperationID: operationID, StudentID: student.ID}); err != nil {
				return translateError(err)
			}
		}
		if len(seen) == 0 {
			return pickup.ErrInvalidState
		}
		return nil
	})
	if err != nil {
		return pickup.Operation{}, err
	}
	return r.FindOperation(ctx, orgID, operationID)
}

func (r *Repository) ConfirmOperation(ctx context.Context, orgID uint64, params pickup.ConfirmOperationParams) (pickup.Operation, error) {
	now := time.Now().UTC()
	result, err := r.queries.ConfirmPickupOperation(ctx, sqlc.ConfirmPickupOperationParams{
		ConfirmedAt:            sql.NullTime{Time: now, Valid: true},
		ConfirmedByUserID:      nullID(params.ConfirmedByUserID),
		ConfirmedByName:        strings.TrimSpace(params.ConfirmedByName),
		ExecutingTeacherUserID: nullID(params.ExecutingTeacherUserID),
		ExecutingTeacherName:   strings.TrimSpace(params.ExecutingTeacherName),
		TeacherRole:            defaultRole(params.TeacherRole),
		ExpectedPickupTime:     strings.TrimSpace(params.ExpectedPickupTime),
		Notes:                  strings.TrimSpace(params.Notes),
		ID:                     params.ID,
		OrganizationID:         orgID,
	})
	if err != nil {
		return pickup.Operation{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return pickup.Operation{}, err
	}
	return r.FindOperation(ctx, orgID, params.ID)
}

func (r *Repository) HandoffOperation(ctx context.Context, orgID uint64, params pickup.HandoffOperationParams) (pickup.Operation, error) {
	var now = time.Now().UTC()
	err := r.withTransaction(ctx, func(q *sqlc.Queries) error {
		current, err := q.GetPickupOperationByID(ctx, sqlc.GetPickupOperationByIDParams{ID: params.ID, OrganizationID: orgID})
		if err != nil {
			return translateError(err)
		}
		if current.Status != pickup.OperationStatusStarted {
			return fmt.Errorf("%w: operation is %s", pickup.ErrInvalidState, current.Status)
		}
		result, err := q.HandoffPickupOperation(ctx, sqlc.HandoffPickupOperationParams{ExecutingTeacherUserID: sql.NullInt64{Int64: int64(params.ToTeacherUserID), Valid: true}, ExecutingTeacherName: strings.TrimSpace(params.ToTeacherName), TeacherRole: defaultRole(params.TeacherRole), ID: params.ID, OrganizationID: orgID})
		if err != nil {
			return translateError(err)
		}
		if err := ensureAffected(result); err != nil {
			return err
		}
		_, err = q.CreatePickupHandoff(ctx, sqlc.CreatePickupHandoffParams{OrganizationID: orgID, OperationID: params.ID, FromTeacherUserID: current.ExecutingTeacherUserID, FromTeacherName: current.ExecutingTeacherName, ToTeacherUserID: params.ToTeacherUserID, ToTeacherName: strings.TrimSpace(params.ToTeacherName), TeacherRole: defaultRole(params.TeacherRole), Note: strings.TrimSpace(params.Note), HandoffAt: now, CreatedByUserID: nullID(params.CreatedByUserID), CreatedByName: strings.TrimSpace(params.CreatedByName)})
		return translateError(err)
	})
	if err != nil {
		return pickup.Operation{}, err
	}
	return r.FindOperation(ctx, orgID, params.ID)
}

func (r *Repository) ListHandoffs(ctx context.Context, orgID, operationID uint64) ([]pickup.Handoff, error) {
	items, err := r.queries.ListPickupHandoffs(ctx, sqlc.ListPickupHandoffsParams{OrganizationID: orgID, OperationID: operationID})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]pickup.Handoff, 0, len(items))
	for _, item := range items {
		out = append(out, pickup.Handoff{ID: item.ID, OrganizationID: item.OrganizationID, OperationID: item.OperationID, FromTeacherUserID: idPtr(item.FromTeacherUserID), FromTeacherName: item.FromTeacherName, ToTeacherUserID: idPtr(sql.NullInt64{Int64: int64(item.ToTeacherUserID), Valid: item.ToTeacherUserID > 0}), ToTeacherName: item.ToTeacherName, TeacherRole: item.TeacherRole, Note: item.Note, HandoffAt: item.HandoffAt, CreatedByUserID: idPtr(item.CreatedByUserID), CreatedByName: item.CreatedByName})
	}
	return out, nil
}

func (r *Repository) SetOperationStatus(ctx context.Context, orgID uint64, params pickup.SetOperationStatusParams) (pickup.Operation, error) {
	current, err := r.FindOperation(ctx, orgID, params.ID)
	if err != nil {
		return pickup.Operation{}, err
	}
	if !validOperationTransition(current.Status, params.Status) {
		return pickup.Operation{}, fmt.Errorf("%w: %s -> %s", pickup.ErrInvalidState, current.Status, params.Status)
	}
	if params.Status == pickup.OperationStatusFinished {
		members, err := r.queries.ListPickupOperationStudents(ctx, sqlc.ListPickupOperationStudentsParams{OperationID: params.ID, OrganizationID: orgID})
		if err != nil {
			return pickup.Operation{}, translateError(err)
		}
		for _, member := range members {
			if !pickup.IsReadyToFinish(member.Status) {
				return pickup.Operation{}, fmt.Errorf("%w: student %s is not ready to finish", pickup.ErrInvalidState, member.StudentName)
			}
		}
	}
	var startedAt, finishedAt sql.NullTime
	if params.Status == pickup.OperationStatusStarted {
		startedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}
	if params.Status == pickup.OperationStatusFinished {
		finishedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}
	result, err := r.queries.UpdatePickupOperationStatus(ctx, sqlc.UpdatePickupOperationStatusParams{Status: params.Status, StartedAt: startedAt, FinishedAt: finishedAt, ID: params.ID, OrganizationID: orgID})
	if err != nil {
		return pickup.Operation{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return pickup.Operation{}, err
	}
	return r.FindOperation(ctx, orgID, params.ID)
}

func (r *Repository) ListOperationStudents(ctx context.Context, orgID, operationID uint64) ([]pickup.OperationStudent, error) {
	items, err := r.queries.ListPickupOperationStudents(ctx, sqlc.ListPickupOperationStudentsParams{OperationID: operationID, OrganizationID: orgID})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]pickup.OperationStudent, 0, len(items))
	for _, item := range items {
		out = append(out, mapOperationStudent(item))
	}
	return out, nil
}

func (r *Repository) AddOperationStudent(ctx context.Context, orgID uint64, params pickup.AddOperationStudentParams) (pickup.OperationStudent, error) {
	operation, err := r.FindOperation(ctx, orgID, params.OperationID)
	if err != nil {
		return pickup.OperationStudent{}, err
	}
	if operation.Status == pickup.OperationStatusFinished || operation.Status == pickup.OperationStatusCancelled {
		return pickup.OperationStudent{}, fmt.Errorf("%w: operation is %s", pickup.ErrInvalidState, operation.Status)
	}
	result, err := r.queries.AddPickupOperationStudent(ctx, sqlc.AddPickupOperationStudentParams{
		OrganizationID: orgID, OperationID: params.OperationID, StudentID: params.StudentID,
		Note: strings.TrimSpace(params.Note), IsTemporary: params.IsTemporary,
		ProfilePending: params.ProfilePending, PickupMode: strings.TrimSpace(params.PickupMode),
	})
	if err != nil {
		return pickup.OperationStudent{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return pickup.OperationStudent{}, err
	}
	item, err := r.queries.GetPickupOperationStudent(ctx, sqlc.GetPickupOperationStudentParams{OperationID: params.OperationID, StudentID: params.StudentID, OrganizationID: orgID})
	if err != nil {
		return pickup.OperationStudent{}, translateError(err)
	}
	return mapOperationStudentByID(item), nil
}

func (r *Repository) CompleteOperationStudentProfile(ctx context.Context, orgID, operationID, studentID uint64) error {
	result, err := r.queries.CompletePickupOperationStudentProfile(ctx, sqlc.CompletePickupOperationStudentProfileParams{
		OperationID: operationID, StudentID: studentID, OrganizationID: orgID,
	})
	if err != nil {
		return translateError(err)
	}
	return ensureAffected(result)
}

func (r *Repository) MarkOperationStudent(ctx context.Context, orgID uint64, params pickup.MarkStudentParams) (pickup.OperationStudent, error) {
	if !pickup.IsValidMemberStatus(params.Status) {
		return pickup.OperationStudent{}, fmt.Errorf("%w: %s", pickup.ErrInvalidStatus, params.Status)
	}
	var eventID uint64
	var notification pickup.Notification
	err := r.withTransaction(ctx, func(q *sqlc.Queries) error {
		operation, err := q.GetPickupOperationByID(ctx, sqlc.GetPickupOperationByIDParams{ID: params.OperationID, OrganizationID: orgID})
		if err != nil {
			return translateError(err)
		}
		if operation.Status != pickup.OperationStatusStarted {
			return fmt.Errorf("%w: operation is %s", pickup.ErrInvalidState, operation.Status)
		}
		member, err := q.GetPickupOperationStudent(ctx, sqlc.GetPickupOperationStudentParams{OperationID: params.OperationID, StudentID: params.StudentID, OrganizationID: orgID})
		if err != nil {
			return translateError(err)
		}
		if !pickup.IsValidMemberTransition(member.Status, params.Status) {
			return fmt.Errorf("%w: %s -> %s", pickup.ErrInvalidState, member.Status, params.Status)
		}
		if member.Status == params.Status {
			photoURL := strings.TrimSpace(params.PhotoURL)
			if photoURL == "" || photoURL == member.PhotoUrl {
				return nil
			}
			result, err := q.UpdatePickupOperationStudent(ctx, sqlc.UpdatePickupOperationStudentParams{Status: member.Status, PhotoUrl: photoURL, CheckedAt: member.CheckedAt, Note: member.Note, OperationID: params.OperationID, StudentID: params.StudentID, OrganizationID: orgID})
			if err != nil {
				return translateError(err)
			}
			if err := ensureAffected(result); err != nil {
				return err
			}
			_, err = q.CreatePickupEvent(ctx, sqlc.CreatePickupEventParams{OrganizationID: orgID, OperationID: params.OperationID, OperationStudentID: member.OperationStudentID, StudentID: params.StudentID, EventType: params.Status, EventAt: time.Now().UTC(), OperatorName: strings.TrimSpace(params.OperatorName), PhotoUrl: photoURL, Note: "补传照片"})
			if err != nil {
				return translateError(err)
			}
			return nil
		}
		now := time.Now().UTC()
		photoURL := strings.TrimSpace(params.PhotoURL)
		if photoURL == "" {
			photoURL = member.PhotoUrl
		}
		result, err := q.UpdatePickupOperationStudent(ctx, sqlc.UpdatePickupOperationStudentParams{Status: params.Status, PhotoUrl: photoURL, CheckedAt: sql.NullTime{Time: now, Valid: true}, Note: strings.TrimSpace(params.Note), OperationID: params.OperationID, StudentID: params.StudentID, OrganizationID: orgID})
		if err != nil {
			return translateError(err)
		}
		if err := ensureAffected(result); err != nil {
			return err
		}
		eventResult, err := q.CreatePickupEvent(ctx, sqlc.CreatePickupEventParams{OrganizationID: orgID, OperationID: params.OperationID, OperationStudentID: member.OperationStudentID, StudentID: params.StudentID, EventType: params.Status, EventAt: now, OperatorName: strings.TrimSpace(params.OperatorName), PhotoUrl: photoURL, Note: strings.TrimSpace(params.Note)})
		if err != nil {
			return translateError(err)
		}
		eventID, err = insertedID(eventResult)
		if err != nil {
			return err
		}
		operationID := params.OperationID
		title := pickupNotificationTitle(params.Status)
		content := fmt.Sprintf("%s：%s", member.StudentName, pickupNotificationContent(params.Status))
		notificationResult, err := q.CreateNotification(ctx, sqlc.CreateNotificationParams{OrganizationID: orgID, StudentID: params.StudentID, OperationID: sql.NullInt64{Int64: int64(operationID), Valid: true}, EventID: sql.NullInt64{Int64: int64(eventID), Valid: true}, Kind: "pickup_status", Title: title, Content: content})
		if err != nil {
			return translateError(err)
		}
		notificationID, err := insertedID(notificationResult)
		if err != nil {
			return err
		}
		if _, err := q.CreateNotificationOutbox(ctx, sqlc.CreateNotificationOutboxParams{OrganizationID: orgID, AggregateID: notificationID, NotificationID: notificationID, JSONOBJECT: notificationID}); err != nil {
			return translateError(err)
		}
		notification = pickup.Notification{ID: notificationID, OrganizationID: orgID, StudentID: params.StudentID, OperationID: &operationID, EventID: &eventID, RecipientType: "parent", Kind: "pickup_status", Title: title, Content: content, Status: "pending", CreatedAt: now}
		return nil
	})
	if err != nil {
		return pickup.OperationStudent{}, err
	}
	updated, err := r.queries.GetPickupOperationStudent(ctx, sqlc.GetPickupOperationStudentParams{OperationID: params.OperationID, StudentID: params.StudentID, OrganizationID: orgID})
	if err != nil {
		return pickup.OperationStudent{}, translateError(err)
	}
	if r.notificationHook != nil && eventID != 0 {
		r.notificationHook(context.WithoutCancel(ctx), notification)
	}
	return mapOperationStudentByID(updated), nil
}

func (r *Repository) CorrectOperationEvent(ctx context.Context, orgID uint64, params pickup.CorrectEventParams) (pickup.OperationStudent, error) {
	if params.Status == pickup.MemberStatusPlanned || !pickup.IsValidMemberStatus(params.Status) || strings.TrimSpace(params.Reason) == "" {
		return pickup.OperationStudent{}, fmt.Errorf("%w: correction is invalid", pickup.ErrInvalidState)
	}
	reason := strings.TrimSpace(params.Reason)
	var studentID uint64
	var correctionID uint64
	var notification pickup.Notification
	err := r.withTransaction(ctx, func(q *sqlc.Queries) error {
		operation, err := q.GetPickupOperationByID(ctx, sqlc.GetPickupOperationByIDParams{ID: params.OperationID, OrganizationID: orgID})
		if err != nil {
			return translateError(err)
		}
		if operation.Status == pickup.OperationStatusCancelled {
			return fmt.Errorf("%w: operation is cancelled", pickup.ErrInvalidState)
		}
		original, err := q.GetPickupEventByID(ctx, sqlc.GetPickupEventByIDParams{ID: params.EventID, OperationID: params.OperationID, OrganizationID: orgID})
		if err != nil {
			return translateError(err)
		}
		studentID = original.StudentID
		member, err := q.GetPickupOperationStudent(ctx, sqlc.GetPickupOperationStudentParams{OperationID: params.OperationID, StudentID: studentID, OrganizationID: orgID})
		if err != nil {
			return translateError(err)
		}
		now := time.Now().UTC()
		if _, err := q.UpdatePickupOperationStudent(ctx, sqlc.UpdatePickupOperationStudentParams{Status: params.Status, PhotoUrl: member.PhotoUrl, CheckedAt: sql.NullTime{Time: now, Valid: true}, Note: reason, OperationID: params.OperationID, StudentID: studentID, OrganizationID: orgID}); err != nil {
			return translateError(err)
		}
		note := fmt.Sprintf("更正事件 #%d 为 %s：%s", original.ID, params.Status, reason)
		correctionResult, err := q.CreatePickupEvent(ctx, sqlc.CreatePickupEventParams{OrganizationID: orgID, OperationID: params.OperationID, OperationStudentID: member.OperationStudentID, StudentID: studentID, EventType: "correction", EventAt: now, OperatorName: strings.TrimSpace(params.OperatorName), PhotoUrl: member.PhotoUrl, Note: note})
		if err != nil {
			return translateError(err)
		}
		correctionID, err = insertedID(correctionResult)
		if err != nil {
			return err
		}
		operationID := params.OperationID
		content := fmt.Sprintf("%s：接送状态已更正为 %s；原因：%s", member.StudentName, params.Status, reason)
		notificationResult, err := q.CreateNotification(ctx, sqlc.CreateNotificationParams{OrganizationID: orgID, StudentID: studentID, OperationID: sql.NullInt64{Int64: int64(operationID), Valid: true}, EventID: sql.NullInt64{Int64: int64(correctionID), Valid: true}, Kind: "pickup_status", Title: "接送记录已更正", Content: content})
		if err != nil {
			return translateError(err)
		}
		notificationID, err := insertedID(notificationResult)
		if err != nil {
			return err
		}
		if _, err := q.CreateNotificationOutbox(ctx, sqlc.CreateNotificationOutboxParams{OrganizationID: orgID, AggregateID: notificationID, NotificationID: notificationID, JSONOBJECT: notificationID}); err != nil {
			return translateError(err)
		}
		notification = pickup.Notification{ID: notificationID, OrganizationID: orgID, StudentID: studentID, OperationID: &operationID, EventID: &correctionID, RecipientType: "parent", Kind: "pickup_status", Title: "接送记录已更正", Content: content, Status: "pending", CreatedAt: now}
		return nil
	})
	if err != nil {
		return pickup.OperationStudent{}, err
	}
	updated, err := r.queries.GetPickupOperationStudent(ctx, sqlc.GetPickupOperationStudentParams{OperationID: params.OperationID, StudentID: studentID, OrganizationID: orgID})
	if err != nil {
		return pickup.OperationStudent{}, translateError(err)
	}
	if r.notificationHook != nil {
		r.notificationHook(context.WithoutCancel(ctx), notification)
	}
	return mapOperationStudentByID(updated), nil
}

func (r *Repository) ListEvents(ctx context.Context, orgID, operationID uint64) ([]pickup.Event, error) {
	items, err := r.queries.ListPickupEvents(ctx, sqlc.ListPickupEventsParams{OperationID: operationID, OrganizationID: orgID})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]pickup.Event, 0, len(items))
	for _, item := range items {
		out = append(out, pickup.Event{ID: item.ID, OrganizationID: item.OrganizationID, OperationID: item.OperationID, OperationStudentID: item.OperationStudentID, StudentID: item.StudentID, EventType: item.EventType, EventAt: item.EventAt, OperatorName: item.OperatorName, PhotoURL: item.PhotoUrl, Note: item.Note})
	}
	return out, nil
}

func (r *Repository) ListNotifications(ctx context.Context, orgID uint64) ([]pickup.Notification, error) {
	items, err := r.queries.ListNotifications(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]pickup.Notification, 0, len(items))
	for _, item := range items {
		out = append(out, mapNotification(item))
	}
	return out, nil
}

func (r *Repository) FindNotification(ctx context.Context, orgID, id uint64) (pickup.Notification, error) {
	item, err := r.queries.GetNotificationByID(ctx, sqlc.GetNotificationByIDParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return pickup.Notification{}, translateError(err)
	}
	return mapNotificationValues(item.ID, item.OrganizationID, item.StudentID, idPtr(item.OperationID), idPtr(item.EventID), item.RecipientType, item.Kind, item.Title, item.Content, item.Status, timePtr(item.SentAt), int(item.DeliveryAttempts), timePtr(item.LastAttemptAt), item.DeliveryError, timePtr(item.NextRetryAt), timePtr(item.ReadAt), item.CreatedAt), nil
}

func (r *Repository) CreateNotification(ctx context.Context, orgID uint64, params pickup.CreateNotificationParams) (pickup.Notification, error) {
	var id uint64
	err := r.withTransaction(ctx, func(q *sqlc.Queries) error {
		result, err := q.CreateNotification(ctx, sqlc.CreateNotificationParams{OrganizationID: orgID, StudentID: params.StudentID, OperationID: nullID(params.OperationID), EventID: nullID(params.EventID), Kind: strings.TrimSpace(params.Kind), Title: strings.TrimSpace(params.Title), Content: strings.TrimSpace(params.Content)})
		if err != nil {
			return translateError(err)
		}
		id, err = insertedID(result)
		if err != nil {
			return err
		}
		if _, err := q.CreateNotificationOutbox(ctx, sqlc.CreateNotificationOutboxParams{OrganizationID: orgID, AggregateID: id, NotificationID: id, JSONOBJECT: id}); err != nil {
			return translateError(err)
		}
		return nil
	})
	if err != nil {
		return pickup.Notification{}, err
	}
	item, err := r.FindNotification(ctx, orgID, id)
	if err != nil {
		return pickup.Notification{}, err
	}
	if r.notificationHook != nil {
		r.notificationHook(context.WithoutCancel(ctx), item)
	}
	return item, nil
}

func (r *Repository) ListNotificationOutbox(ctx context.Context, now, staleBefore time.Time, limit int) ([]pickup.NotificationOutbox, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	items, err := r.queries.ListNotificationOutbox(ctx, sqlc.ListNotificationOutboxParams{AvailableAt: now, LockedAt: sql.NullTime{Time: staleBefore, Valid: true}, Limit: int32(limit)})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]pickup.NotificationOutbox, 0, len(items))
	for _, item := range items {
		out = append(out, pickup.NotificationOutbox{ID: item.ID, OrganizationID: item.OrganizationID, EventType: item.EventType, AggregateType: item.AggregateType, AggregateID: item.AggregateID, NotificationID: item.NotificationID, Status: item.Status, Attempts: int(item.Attempts), AvailableAt: item.AvailableAt, LockedAt: timePtr(item.LockedAt), ProcessedAt: timePtr(item.ProcessedAt), LastError: item.LastError, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt})
	}
	return out, nil
}

func (r *Repository) ClaimNotificationOutbox(ctx context.Context, orgID, id uint64, claimedAt time.Time) (bool, error) {
	result, err := r.queries.ClaimNotificationOutbox(ctx, sqlc.ClaimNotificationOutboxParams{LockedAt: sql.NullTime{Time: claimedAt, Valid: true}, ID: id, OrganizationID: orgID, LockedAt_2: sql.NullTime{Time: claimedAt, Valid: true}})
	if err != nil {
		return false, translateError(err)
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (r *Repository) CompleteNotificationOutbox(ctx context.Context, orgID, id uint64, status string, availableAt *time.Time, lastError string) error {
	if status != "pending" && status != "processed" && status != "failed" {
		return pickup.ErrInvalidState
	}
	when := time.Now().UTC()
	if availableAt != nil {
		when = availableAt.UTC()
	}
	result, err := r.queries.CompleteNotificationOutbox(ctx, sqlc.CompleteNotificationOutboxParams{Status: status, AvailableAt: when, Column3: status, LastError: strings.TrimSpace(lastError), ID: id, OrganizationID: orgID})
	if err != nil {
		return translateError(err)
	}
	return ensureAffected(result)
}

func (r *Repository) CreateNotificationDeliveryLog(ctx context.Context, orgID uint64, params pickup.CreateDeliveryLogParams) (pickup.NotificationDeliveryLog, error) {
	result, err := r.queries.CreateNotificationDeliveryLog(ctx, sqlc.CreateNotificationDeliveryLogParams{OrganizationID: orgID, NotificationID: params.NotificationID, ParentAccountID: params.ParentAccountID, MessageKind: strings.TrimSpace(params.MessageKind), TemplateID: strings.TrimSpace(params.TemplateID)})
	if err != nil {
		return pickup.NotificationDeliveryLog{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return pickup.NotificationDeliveryLog{}, err
	}
	item, err := r.queries.GetNotificationDeliveryLogByID(ctx, sqlc.GetNotificationDeliveryLogByIDParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return pickup.NotificationDeliveryLog{}, translateError(err)
	}
	return mapDeliveryLog(item), nil
}

func (r *Repository) SetNotificationDeliveryLogStatus(ctx context.Context, orgID uint64, params pickup.SetDeliveryLogStatusParams) error {
	if params.Status != "pending" && params.Status != "sent" && params.Status != "failed" && params.Status != "skipped" {
		return pickup.ErrInvalidState
	}
	result, err := r.queries.UpdateNotificationDeliveryLogStatus(ctx, sqlc.UpdateNotificationDeliveryLogStatusParams{Status: params.Status, Attempts: uint32(maxInt(params.Attempts, 0)), LastAttemptAt: nullTime(params.LastAttemptAt), SentAt: nullTime(params.SentAt), NextRetryAt: nullTime(params.NextRetryAt), DeliveryError: strings.TrimSpace(params.DeliveryError), ID: params.ID, OrganizationID: orgID})
	if err != nil {
		return translateError(err)
	}
	return ensureAffected(result)
}

func (r *Repository) ListNotificationDeliveryLogs(ctx context.Context, orgID uint64, notificationID *uint64, status string) ([]pickup.NotificationDeliveryLog, error) {
	items, err := r.queries.ListNotificationDeliveryLogs(ctx, sqlc.ListNotificationDeliveryLogsParams{OrganizationID: orgID, NotificationIDFilter: nullID(notificationID), StatusFilter: nullString(status)})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]pickup.NotificationDeliveryLog, 0, len(items))
	for _, item := range items {
		out = append(out, mapDeliveryLog(item))
	}
	return out, nil
}

func (r *Repository) RetryNotification(ctx context.Context, orgID, notificationID uint64) error {
	return r.withTransaction(ctx, func(q *sqlc.Queries) error {
		result, err := q.RetryNotificationOutbox(ctx, sqlc.RetryNotificationOutboxParams{NotificationID: notificationID, OrganizationID: orgID})
		if err != nil {
			return translateError(err)
		}
		if err := ensureAffected(result); err != nil {
			return fmt.Errorf("%w: notification is not failed", pickup.ErrInvalidState)
		}
		if _, err := q.UpdateNotificationStatus(ctx, sqlc.UpdateNotificationStatusParams{Status: "pending", SentAt: sql.NullTime{}, DeliveryAttempts: 0, LastAttemptAt: sql.NullTime{}, DeliveryError: "", NextRetryAt: sql.NullTime{}, ID: notificationID, OrganizationID: orgID}); err != nil {
			return translateError(err)
		}
		return nil
	})
}

func (r *Repository) SetNotificationStatus(ctx context.Context, orgID uint64, params pickup.SetNotificationStatusParams) error {
	if params.Status != "pending" && params.Status != "sent" && params.Status != "failed" {
		return pickup.ErrInvalidState
	}
	result, err := r.queries.UpdateNotificationStatus(ctx, sqlc.UpdateNotificationStatusParams{Status: params.Status, SentAt: nullTime(params.SentAt), DeliveryAttempts: uint32(maxInt(params.DeliveryAttempts, 0)), LastAttemptAt: nullTime(params.LastAttemptAt), DeliveryError: strings.TrimSpace(params.DeliveryError), NextRetryAt: nullTime(params.NextRetryAt), ID: params.ID, OrganizationID: orgID})
	if err != nil {
		return translateError(err)
	}
	return ensureAffected(result)
}

func maxInt(value, floor int) int {
	if value < floor {
		return floor
	}
	return value
}

func nullString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func (r *Repository) MarkNotificationRead(ctx context.Context, orgID, id uint64) error {
	result, err := r.queries.MarkNotificationRead(ctx, sqlc.MarkNotificationReadParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return translateError(err)
	}
	return ensureAffected(result)
}

func (r *Repository) CreatePickupChangeRequest(ctx context.Context, orgID uint64, params pickup.CreatePickupChangeRequestParams) (pickup.PickupChangeRequest, error) {
	result, err := r.queries.CreatePickupChangeRequest(ctx, sqlc.CreatePickupChangeRequestParams{
		OrganizationID: orgID, StudentID: params.StudentID, OperationID: nullID(params.OperationID),
		ChangeDate: params.ChangeDate, RequestedStatus: strings.TrimSpace(params.RequestedStatus),
		Note: strings.TrimSpace(params.Note), SubmittedBy: nonEmpty(params.SubmittedBy, "parent"),
	})
	if err != nil {
		return pickup.PickupChangeRequest{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return pickup.PickupChangeRequest{}, err
	}
	item, err := r.queries.GetPickupChangeRequest(ctx, sqlc.GetPickupChangeRequestParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return pickup.PickupChangeRequest{}, translateError(err)
	}
	return mapChangeRequest(item), nil
}

func (r *Repository) ListPickupChangeRequests(ctx context.Context, orgID uint64, date *time.Time, status string) ([]pickup.PickupChangeRequest, error) {
	items, err := r.queries.ListPickupChangeRequests(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]pickup.PickupChangeRequest, 0, len(items))
	for _, item := range items {
		if date != nil && !sameDay(item.ChangeDate, *date) {
			continue
		}
		if strings.TrimSpace(status) != "" && item.Status != status {
			continue
		}
		out = append(out, mapChangeRequestValues(item.ID, item.OrganizationID, item.StudentID, item.StudentName, idPtr(item.OperationID), item.ChangeDate, item.RequestedStatus, item.Note, item.SubmittedBy, item.Status, idPtr(item.ReviewedByUserID), timePtr(item.ReviewedAt), item.ReviewNote, item.CreatedAt, item.UpdatedAt))
	}
	return out, nil
}

func (r *Repository) ReviewPickupChangeRequest(ctx context.Context, orgID uint64, params pickup.ReviewPickupChangeRequestParams) (pickup.PickupChangeRequest, error) {
	if params.Status != pickup.ChangeRequestStatusApproved && params.Status != pickup.ChangeRequestStatusRejected {
		return pickup.PickupChangeRequest{}, pickup.ErrInvalidStatus
	}
	now := time.Now().UTC()
	result, err := r.queries.ReviewPickupChangeRequest(ctx, sqlc.ReviewPickupChangeRequestParams{Status: params.Status, ReviewedByUserID: nullID(params.ReviewedByUserID), ReviewedAt: sql.NullTime{Time: now, Valid: true}, ReviewNote: strings.TrimSpace(params.ReviewNote), ID: params.ID, OrganizationID: orgID})
	if err != nil {
		return pickup.PickupChangeRequest{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return pickup.PickupChangeRequest{}, err
	}
	item, err := r.queries.GetPickupChangeRequest(ctx, sqlc.GetPickupChangeRequestParams{ID: params.ID, OrganizationID: orgID})
	if err != nil {
		return pickup.PickupChangeRequest{}, translateError(err)
	}
	return mapChangeRequest(item), nil
}

func mapOperation(item sqlc.PickupOperation) pickup.Operation {
	return pickup.Operation{ID: item.ID, OrganizationID: item.OrganizationID, OperationDate: item.OperationDate, PickupMode: item.PickupMode, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, CareClassID: idPtr(item.CareClassID), TeacherUserID: idPtr(item.TeacherUserID), TeacherName: item.TeacherName, Status: item.Status, StartedAt: timePtr(item.StartedAt), FinishedAt: timePtr(item.FinishedAt), ConfirmedAt: timePtr(item.ConfirmedAt), ConfirmedByUserID: idPtr(item.ConfirmedByUserID), ConfirmedByName: item.ConfirmedByName, ExecutingTeacherUserID: idPtr(item.ExecutingTeacherUserID), ExecutingTeacherName: item.ExecutingTeacherName, TeacherRole: item.TeacherRole, ExpectedPickupTime: item.ExpectedPickupTime, Notes: item.Notes, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapOperationStudent(item sqlc.ListPickupOperationStudentsRow) pickup.OperationStudent {
	return mapOperationStudentValues(item.OperationStudentID, item.OrganizationID, item.OperationID, item.StudentID, item.StudentName, item.Status, item.PhotoUrl, item.CheckedAt, item.Note, item.IsTemporary, item.ProfilePending, item.PickupMode, item.CreatedAt, item.UpdatedAt)
}

func mapOperationStudentByID(item sqlc.GetPickupOperationStudentRow) pickup.OperationStudent {
	return mapOperationStudentValues(item.OperationStudentID, item.OrganizationID, item.OperationID, item.StudentID, item.StudentName, item.Status, item.PhotoUrl, item.CheckedAt, item.Note, item.IsTemporary, item.ProfilePending, item.PickupMode, item.CreatedAt, item.UpdatedAt)
}

func mapOperationStudentValues(id, orgID, operationID, studentID uint64, studentName, status, photoURL string, checkedAt sql.NullTime, note string, isTemporary, profilePending bool, pickupMode string, createdAt, updatedAt time.Time) pickup.OperationStudent {
	return pickup.OperationStudent{ID: id, OrganizationID: orgID, OperationID: operationID, StudentID: studentID, StudentName: studentName, Status: status, PhotoURL: photoURL, CheckedAt: timePtr(checkedAt), Note: note, IsTemporary: isTemporary, ProfilePending: profilePending, PickupMode: pickupMode, CreatedAt: createdAt, UpdatedAt: updatedAt}
}

func mapNotification(item sqlc.ListNotificationsRow) pickup.Notification {
	return mapNotificationValues(item.ID, item.OrganizationID, item.StudentID, idPtr(item.OperationID), idPtr(item.EventID), item.RecipientType, item.Kind, item.Title, item.Content, item.Status, timePtr(item.SentAt), int(item.DeliveryAttempts), timePtr(item.LastAttemptAt), item.DeliveryError, timePtr(item.NextRetryAt), timePtr(item.ReadAt), item.CreatedAt)
}

func mapNotificationValues(id, orgID, studentID uint64, operationID, eventID *uint64, recipientType, kind, title, content, status string, sentAt *time.Time, attempts int, lastAttemptAt *time.Time, deliveryError string, nextRetryAt, readAt *time.Time, createdAt time.Time) pickup.Notification {
	return pickup.Notification{ID: id, OrganizationID: orgID, StudentID: studentID, OperationID: operationID, EventID: eventID, RecipientType: recipientType, Kind: kind, Title: title, Content: content, Status: status, SentAt: sentAt, DeliveryAttempts: attempts, LastAttemptAt: lastAttemptAt, DeliveryError: deliveryError, NextRetryAt: nextRetryAt, ReadAt: readAt, CreatedAt: createdAt}
}

func mapDeliveryLog(item sqlc.NotificationDeliveryLog) pickup.NotificationDeliveryLog {
	return pickup.NotificationDeliveryLog{ID: item.ID, OrganizationID: item.OrganizationID, NotificationID: item.NotificationID, ParentAccountID: item.ParentAccountID, MessageKind: item.MessageKind, TemplateID: item.TemplateID, Status: item.Status, Attempts: int(item.Attempts), LastAttemptAt: timePtr(item.LastAttemptAt), SentAt: timePtr(item.SentAt), NextRetryAt: timePtr(item.NextRetryAt), DeliveryError: item.DeliveryError, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapChangeRequest(item sqlc.GetPickupChangeRequestRow) pickup.PickupChangeRequest {
	return mapChangeRequestValues(item.ID, item.OrganizationID, item.StudentID, item.StudentName, idPtr(item.OperationID), item.ChangeDate, item.RequestedStatus, item.Note, item.SubmittedBy, item.Status, idPtr(item.ReviewedByUserID), timePtr(item.ReviewedAt), item.ReviewNote, item.CreatedAt, item.UpdatedAt)
}

func mapChangeRequestValues(id, orgID, studentID uint64, studentName string, operationID *uint64, changeDate time.Time, requestedStatus, note, submittedBy, status string, reviewedBy *uint64, reviewedAt *time.Time, reviewNote string, createdAt, updatedAt time.Time) pickup.PickupChangeRequest {
	return pickup.PickupChangeRequest{ID: id, OrganizationID: orgID, StudentID: studentID, StudentName: studentName, OperationID: operationID, ChangeDate: changeDate, RequestedStatus: requestedStatus, Note: note, SubmittedBy: submittedBy, Status: status, ReviewedByUserID: reviewedBy, ReviewedAt: reviewedAt, ReviewNote: reviewNote, CreatedAt: createdAt, UpdatedAt: updatedAt}
}

func idPtr(value sql.NullInt64) *uint64 {
	if !value.Valid {
		return nil
	}
	converted := uint64(value.Int64)
	return &converted
}
func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	converted := value.Time
	return &converted
}
func nullID(value *uint64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func nullTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}
func insertedID(result sql.Result) (uint64, error) {
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read created id: %w", err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("read created id: invalid id %d", id)
	}
	return uint64(id), nil
}
func ensureAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return pickup.ErrNotFound
	}
	return nil
}
func validOperationTransition(from, to string) bool {
	return ((from == pickup.OperationStatusDraft || from == pickup.OperationStatusConfirmed) && to == pickup.OperationStatusStarted) || (from == pickup.OperationStatusStarted && to == pickup.OperationStatusFinished)
}

func defaultRole(value string) string {
	switch strings.TrimSpace(value) {
	case "lead", "collaborator", "substitute":
		return strings.TrimSpace(value)
	default:
		return "lead"
	}
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func sameDay(left, right time.Time) bool {
	left, right = left.UTC(), right.UTC()
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}
func pickupNotificationTitle(status string) string {
	switch status {
	case pickup.MemberStatusPickedUp:
		return "孩子已在校门口接到"
	case pickup.MemberStatusSelfArrived:
		return "孩子已到托管班"
	case pickup.MemberStatusParentPickedUp:
		return "孩子已由家长接走"
	case pickup.MemberStatusLeave:
		return "孩子今日请假"
	default:
		return "孩子今日未到托管班"
	}
}
func pickupNotificationContent(status string) string {
	switch status {
	case pickup.MemberStatusPickedUp:
		return "老师已在学校门口接到孩子，照片已记录。"
	case pickup.MemberStatusSelfArrived:
		return "孩子已自行到达托管班。"
	case pickup.MemberStatusParentPickedUp:
		return "孩子已登记为家长临时接走。"
	case pickup.MemberStatusLeave:
		return "孩子已登记今日请假。"
	default:
		return "孩子已登记为今日未到托管班。"
	}
}

func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return pickup.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return pickup.ErrConflict
	}
	if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint fails") {
		return fmt.Errorf("%w: invalid relation", pickup.ErrNotFound)
	}
	return err
}
