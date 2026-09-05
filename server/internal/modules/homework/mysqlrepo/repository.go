package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/homework"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database/sqlc"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct {
	queries *sqlc.Queries
	exec    database.DBTX
}

func New(exec database.DBTX) *Repository { return &Repository{queries: sqlc.New(exec), exec: exec} }

func (r *Repository) ListTasks(ctx context.Context, orgID uint64) ([]homework.Task, error) {
	items, err := r.queries.ListHomeworkTasks(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]homework.Task, 0, len(items))
	for _, item := range items {
		out = append(out, mapListTask(item))
	}
	return out, nil
}

func (r *Repository) FindTask(ctx context.Context, orgID, id uint64) (homework.Task, error) {
	item, err := r.queries.GetHomeworkTask(ctx, sqlc.GetHomeworkTaskParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return homework.Task{}, translateError(err)
	}
	return mapGetTask(item), nil
}

func (r *Repository) CreateTask(ctx context.Context, orgID uint64, params homework.CreateTaskParams, roster []homework.StudentRef) (homework.Task, error) {
	if len(roster) == 0 {
		return homework.Task{}, fmt.Errorf("%w: roster is empty", homework.ErrNotFound)
	}
	attachments, err := json.Marshal(params.AttachmentURLs)
	if err != nil {
		return homework.Task{}, fmt.Errorf("encode homework attachments: %w", err)
	}
	result, err := r.queries.CreateHomeworkTask(ctx, sqlc.CreateHomeworkTaskParams{OrganizationID: orgID, HomeworkDate: params.HomeworkDate, SchoolID: params.SchoolID, SchoolClassID: params.SchoolClassID, Subject: params.Subject, Content: params.Content, AttachmentUrls: attachments, CreatedByUserID: nullID(params.CreatedByUserID), CreatorName: params.CreatorName})
	if err != nil {
		return homework.Task{}, translateError(err)
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		return homework.Task{}, fmt.Errorf("read created homework id: %w", err)
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
		if _, err := r.queries.CreateHomeworkTaskStudent(ctx, sqlc.CreateHomeworkTaskStudentParams{OrganizationID: orgID, TaskID: uint64(id), StudentID: student.ID}); err != nil {
			return homework.Task{}, translateError(err)
		}
	}
	if len(seen) == 0 {
		return homework.Task{}, fmt.Errorf("%w: roster is empty", homework.ErrNotFound)
	}
	return r.FindTask(ctx, orgID, uint64(id))
}

func (r *Repository) ListTaskStudents(ctx context.Context, orgID, taskID uint64) ([]homework.TaskStudent, error) {
	items, err := r.queries.ListHomeworkTaskStudents(ctx, sqlc.ListHomeworkTaskStudentsParams{TaskID: taskID, OrganizationID: orgID})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]homework.TaskStudent, 0, len(items))
	for _, item := range items {
		out = append(out, mapTaskStudent(item))
	}
	return out, nil
}

func (r *Repository) ReviewStudent(ctx context.Context, orgID uint64, params homework.ReviewStudentParams) (homework.TaskStudent, error) {
	if !validStudentStatus(params.Status) {
		return homework.TaskStudent{}, homework.ErrInvalidStatus
	}
	result, err := r.queries.UpdateHomeworkTaskStudentReview(ctx, sqlc.UpdateHomeworkTaskStudentReviewParams{Status: params.Status, CorrectionNote: strings.TrimSpace(params.CorrectionNote), ReviewedByUserID: nullID(params.ReviewedByUserID), ReviewedAt: sql.NullTime{Time: nowUTC(), Valid: true}, TaskID: params.TaskID, StudentID: params.StudentID, OrganizationID: orgID})
	if err != nil {
		return homework.TaskStudent{}, translateError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return homework.TaskStudent{}, err
	}
	if affected == 0 {
		return homework.TaskStudent{}, homework.ErrNotFound
	}
	item, err := r.queries.GetHomeworkTaskStudent(ctx, sqlc.GetHomeworkTaskStudentParams{TaskID: params.TaskID, StudentID: params.StudentID, OrganizationID: orgID})
	if err != nil {
		return homework.TaskStudent{}, translateError(err)
	}
	return mapTaskStudentByID(item), nil
}

func (r *Repository) BulkReviewStudents(ctx context.Context, orgID uint64, params homework.BulkReviewStudentsParams) ([]homework.TaskStudent, error) {
	if len(params.Items) == 0 {
		return nil, fmt.Errorf("%w: empty review batch", homework.ErrConflict)
	}
	seen := make(map[uint64]struct{}, len(params.Items))
	for _, review := range params.Items {
		if review.StudentID == 0 {
			return nil, fmt.Errorf("%w: student is required", homework.ErrNotFound)
		}
		if !validStudentStatus(review.Status) {
			return nil, homework.ErrInvalidStatus
		}
		if _, exists := seen[review.StudentID]; exists {
			return nil, fmt.Errorf("%w: duplicate student %d", homework.ErrConflict, review.StudentID)
		}
		seen[review.StudentID] = struct{}{}
	}
	apply := func(queries *sqlc.Queries) ([]homework.TaskStudent, error) {
		out := make([]homework.TaskStudent, 0, len(params.Items))
		for _, review := range params.Items {
			if _, err := queries.GetHomeworkTaskStudent(ctx, sqlc.GetHomeworkTaskStudentParams{TaskID: params.TaskID, StudentID: review.StudentID, OrganizationID: orgID}); err != nil {
				return nil, translateError(err)
			}
			if _, err := queries.UpdateHomeworkTaskStudentReview(ctx, sqlc.UpdateHomeworkTaskStudentReviewParams{Status: review.Status, CorrectionNote: strings.TrimSpace(review.CorrectionNote), ReviewedByUserID: nullID(params.ReviewedByUserID), ReviewedAt: sql.NullTime{Time: nowUTC(), Valid: true}, TaskID: params.TaskID, StudentID: review.StudentID, OrganizationID: orgID}); err != nil {
				return nil, translateError(err)
			}
			// Re-read below so the result uses the database's canonical timestamps.
			updated, err := queries.GetHomeworkTaskStudent(ctx, sqlc.GetHomeworkTaskStudentParams{TaskID: params.TaskID, StudentID: review.StudentID, OrganizationID: orgID})
			if err != nil {
				return nil, translateError(err)
			}
			out = append(out, mapTaskStudentByID(updated))
		}
		return out, nil
	}
	if beginner, ok := r.exec.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}); ok {
		tx, err := beginner.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		items, applyErr := apply(r.queries.WithTx(tx))
		if applyErr != nil {
			_ = tx.Rollback()
			return nil, applyErr
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return items, nil
	}
	// Lightweight DBTX implementations used by focused tests do not expose a
	// transaction. Production uses *sql.DB and always takes the atomic branch.
	return apply(r.queries)
}

func (r *Repository) ListStudentHomework(ctx context.Context, orgID, studentID uint64) ([]homework.StudentHomework, error) {
	items, err := r.queries.ListStudentHomework(ctx, sqlc.ListStudentHomeworkParams{OrganizationID: orgID, StudentID: studentID})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]homework.StudentHomework, 0, len(items))
	for _, item := range items {
		out = append(out, mapStudentHomework(item))
	}
	return out, nil
}

func mapTask(item sqlc.HomeworkTask) homework.Task {
	return homework.Task{ID: item.ID, OrganizationID: item.OrganizationID, HomeworkDate: item.HomeworkDate, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, Subject: item.Subject, Content: item.Content, AttachmentURLs: decodeAttachments(item.AttachmentUrls), CreatedByUserID: idPtr(item.CreatedByUserID), CreatorName: item.CreatorName, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapListTask(item sqlc.ListHomeworkTasksRow) homework.Task {
	return homework.Task{ID: item.ID, OrganizationID: item.OrganizationID, HomeworkDate: item.HomeworkDate, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, Subject: item.Subject, Content: item.Content, AttachmentURLs: decodeAttachments(item.AttachmentUrls), CreatedByUserID: idPtr(item.CreatedByUserID), CreatorName: item.CreatorName, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapGetTask(item sqlc.GetHomeworkTaskRow) homework.Task {
	return homework.Task{ID: item.ID, OrganizationID: item.OrganizationID, HomeworkDate: item.HomeworkDate, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, Subject: item.Subject, Content: item.Content, AttachmentURLs: decodeAttachments(item.AttachmentUrls), CreatedByUserID: idPtr(item.CreatedByUserID), CreatorName: item.CreatorName, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapTaskStudent(item sqlc.ListHomeworkTaskStudentsRow) homework.TaskStudent {
	return homework.TaskStudent{ID: item.HomeworkTaskStudentID, OrganizationID: item.OrganizationID, TaskID: item.TaskID, StudentID: item.StudentID, StudentName: item.StudentName, Status: item.Status, CorrectionNote: item.CorrectionNote, ReviewedByUserID: idPtr(item.ReviewedByUserID), ReviewedAt: timePtr(item.ReviewedAt), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapTaskStudentByID(item sqlc.GetHomeworkTaskStudentRow) homework.TaskStudent {
	return homework.TaskStudent{ID: item.HomeworkTaskStudentID, OrganizationID: item.OrganizationID, TaskID: item.TaskID, StudentID: item.StudentID, StudentName: item.StudentName, Status: item.Status, CorrectionNote: item.CorrectionNote, ReviewedByUserID: idPtr(item.ReviewedByUserID), ReviewedAt: timePtr(item.ReviewedAt), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func mapStudentHomework(item sqlc.ListStudentHomeworkRow) homework.StudentHomework {
	return homework.StudentHomework{Task: homework.Task{ID: item.TaskID, OrganizationID: item.OrganizationID, HomeworkDate: item.HomeworkDate, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, Subject: item.Subject, Content: item.Content, AttachmentURLs: decodeAttachments(item.AttachmentUrls), CreatedByUserID: idPtr(item.CreatedByUserID), CreatorName: item.CreatorName, Status: item.TaskStatus, CreatedAt: item.TaskCreatedAt, UpdatedAt: item.TaskUpdatedAt}, TaskStudent: homework.TaskStudent{ID: item.HomeworkTaskStudentID, OrganizationID: item.OrganizationID, TaskID: item.TaskID, StudentID: item.StudentID, StudentName: item.StudentName, Status: item.StudentStatus, CorrectionNote: item.CorrectionNote, ReviewedByUserID: idPtr(item.ReviewedByUserID), ReviewedAt: timePtr(item.ReviewedAt), CreatedAt: item.StudentCreatedAt, UpdatedAt: item.StudentUpdatedAt}}
}

func decodeAttachments(value json.RawMessage) []string {
	var attachments []string
	if len(value) == 0 || json.Unmarshal(value, &attachments) != nil {
		return []string{}
	}
	return attachments
}

func validStudentStatus(status string) bool {
	return status == homework.StudentStatusCompleted || status == homework.StudentStatusIncomplete || status == homework.StudentStatusNotSubmitted
}

func nullID(value *uint64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
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

func nowUTC() time.Time { return time.Now().UTC() }

func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return homework.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return homework.ErrConflict
	}
	if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint fails") {
		return fmt.Errorf("%w: invalid relation", homework.ErrNotFound)
	}
	return err
}
