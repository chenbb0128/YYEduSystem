package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/wrongbook"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct {
	exec database.DBTX
}

func New(exec database.DBTX) *Repository { return &Repository{exec: exec} }

func (r *Repository) ListQuestions(ctx context.Context, orgID uint64, params wrongbook.ListQuestionsParams) ([]wrongbook.Question, error) {
	query := strings.Builder{}
	query.WriteString(`SELECT wq.id, wq.organization_id, wq.student_id, s.name AS student_name,
       wq.subject, wq.question_text, wq.answer_text, wq.explanation, wq.knowledge_point,
       wq.source_image_url, wq.source_homework_task_id, wq.teacher_note, wq.status,
       wq.created_by_user_id, wq.created_by_name, wq.last_reviewed_at, wq.created_at, wq.updated_at
FROM wrong_questions wq
JOIN students s ON s.id = wq.student_id
WHERE wq.organization_id = ?`)
	args := []any{orgID}
	if params.StudentID != 0 {
		query.WriteString(" AND wq.student_id = ?")
		args = append(args, params.StudentID)
	}
	if strings.TrimSpace(params.Subject) != "" {
		query.WriteString(" AND wq.subject = ?")
		args = append(args, strings.TrimSpace(params.Subject))
	}
	if strings.TrimSpace(params.Status) != "" {
		query.WriteString(" AND wq.status = ?")
		args = append(args, strings.TrimSpace(params.Status))
	}
	if strings.TrimSpace(params.Keyword) != "" {
		query.WriteString(" AND (wq.question_text LIKE ? OR wq.answer_text LIKE ? OR wq.knowledge_point LIKE ? OR wq.teacher_note LIKE ?)")
		keyword := "%" + strings.TrimSpace(params.Keyword) + "%"
		args = append(args, keyword, keyword, keyword, keyword)
	}
	query.WriteString(" ORDER BY wq.created_at DESC, wq.id DESC")

	rows, err := r.exec.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, translateError(err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]wrongbook.Question, 0)
	for rows.Next() {
		item, scanErr := scanQuestion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}
	return out, nil
}

func (r *Repository) FindQuestion(ctx context.Context, orgID, id uint64) (wrongbook.Question, error) {
	row := r.exec.QueryRowContext(ctx, `SELECT wq.id, wq.organization_id, wq.student_id, s.name AS student_name,
       wq.subject, wq.question_text, wq.answer_text, wq.explanation, wq.knowledge_point,
       wq.source_image_url, wq.source_homework_task_id, wq.teacher_note, wq.status,
       wq.created_by_user_id, wq.created_by_name, wq.last_reviewed_at, wq.created_at, wq.updated_at
FROM wrong_questions wq
JOIN students s ON s.id = wq.student_id
WHERE wq.id = ? AND wq.organization_id = ?
LIMIT 1`, id, orgID)
	item, err := scanQuestion(row)
	if err != nil {
		return wrongbook.Question{}, translateError(err)
	}
	return item, nil
}

func (r *Repository) CreateQuestion(ctx context.Context, orgID uint64, params wrongbook.CreateQuestionParams) (wrongbook.Question, error) {
	result, err := r.exec.ExecContext(ctx, `INSERT INTO wrong_questions (
    organization_id, student_id, subject, question_text, answer_text, explanation,
    knowledge_point, source_image_url, source_homework_task_id, teacher_note,
    status, created_by_user_id, created_by_name
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		orgID,
		params.StudentID,
		strings.TrimSpace(params.Subject),
		strings.TrimSpace(params.QuestionText),
		strings.TrimSpace(params.AnswerText),
		strings.TrimSpace(params.Explanation),
		strings.TrimSpace(params.KnowledgePoint),
		strings.TrimSpace(params.SourceImageURL),
		nullID(params.SourceHomeworkTaskID),
		strings.TrimSpace(params.TeacherNote),
		nullID(params.CreatedByUserID),
		strings.TrimSpace(params.CreatedByName),
	)
	if err != nil {
		return wrongbook.Question{}, translateError(err)
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		return wrongbook.Question{}, fmt.Errorf("read created wrong question id: %w", err)
	}
	return r.FindQuestion(ctx, orgID, uint64(id))
}

func (r *Repository) BulkCreateQuestions(ctx context.Context, orgID uint64, params wrongbook.BulkCreateQuestionsParams) ([]wrongbook.Question, error) {
	if len(params.Items) == 0 {
		return nil, fmt.Errorf("%w: empty questions", wrongbook.ErrInvalidState)
	}
	apply := func(ctx context.Context, store *Repository) ([]wrongbook.Question, error) {
		out := make([]wrongbook.Question, 0, len(params.Items))
		for _, item := range params.Items {
			created, err := store.CreateQuestion(ctx, orgID, item)
			if err != nil {
				return nil, err
			}
			out = append(out, created)
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
		items, applyErr := apply(ctx, New(tx))
		if applyErr != nil {
			_ = tx.Rollback()
			return nil, applyErr
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return items, nil
	}
	return apply(ctx, r)
}

func (r *Repository) UpdateQuestion(ctx context.Context, orgID uint64, params wrongbook.UpdateQuestionParams) (wrongbook.Question, error) {
	if !validQuestionStatus(params.Status) {
		return wrongbook.Question{}, wrongbook.ErrInvalidStatus
	}
	var reviewedAt sql.NullTime
	if params.Status == wrongbook.QuestionStatusMastered {
		reviewedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}
	result, err := r.exec.ExecContext(ctx, `UPDATE wrong_questions
SET subject = ?, question_text = ?, answer_text = ?, explanation = ?, knowledge_point = ?,
    teacher_note = ?, status = ?, last_reviewed_at = ?
WHERE id = ? AND organization_id = ?`,
		strings.TrimSpace(params.Subject),
		strings.TrimSpace(params.QuestionText),
		strings.TrimSpace(params.AnswerText),
		strings.TrimSpace(params.Explanation),
		strings.TrimSpace(params.KnowledgePoint),
		strings.TrimSpace(params.TeacherNote),
		params.Status,
		reviewedAt,
		params.ID,
		orgID,
	)
	if err != nil {
		return wrongbook.Question{}, translateError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return wrongbook.Question{}, err
	}
	if affected == 0 {
		return wrongbook.Question{}, wrongbook.ErrNotFound
	}
	return r.FindQuestion(ctx, orgID, params.ID)
}

func (r *Repository) ListPapers(ctx context.Context, orgID uint64, params wrongbook.ListPapersParams) ([]wrongbook.Paper, error) {
	query := strings.Builder{}
	query.WriteString(`SELECT wp.id, wp.organization_id, wp.student_id, s.name AS student_name, wp.title,
       wp.source, wp.status, wp.generated_by_type, wp.generated_by_user_id,
       COUNT(wpq.id) AS question_count, wp.created_at, wp.updated_at
FROM wrong_papers wp
JOIN students s ON s.id = wp.student_id
LEFT JOIN wrong_paper_questions wpq ON wpq.paper_id = wp.id
WHERE wp.organization_id = ?`)
	args := []any{orgID}
	if params.StudentID != 0 {
		query.WriteString(" AND wp.student_id = ?")
		args = append(args, params.StudentID)
	}
	query.WriteString(" GROUP BY wp.id, wp.organization_id, wp.student_id, s.name, wp.title, wp.source, wp.status, wp.generated_by_type, wp.generated_by_user_id, wp.created_at, wp.updated_at ORDER BY wp.created_at DESC, wp.id DESC")
	rows, err := r.exec.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, translateError(err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]wrongbook.Paper, 0)
	for rows.Next() {
		item, scanErr := scanPaperSummary(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}
	return out, nil
}

func (r *Repository) FindPaper(ctx context.Context, orgID, id uint64) (wrongbook.Paper, error) {
	row := r.exec.QueryRowContext(ctx, `SELECT wp.id, wp.organization_id, wp.student_id, s.name AS student_name, wp.title,
       wp.source, wp.status, wp.generated_by_type, wp.generated_by_user_id,
       COUNT(wpq.id) AS question_count, wp.created_at, wp.updated_at
FROM wrong_papers wp
JOIN students s ON s.id = wp.student_id
LEFT JOIN wrong_paper_questions wpq ON wpq.paper_id = wp.id
WHERE wp.id = ? AND wp.organization_id = ?
GROUP BY wp.id, wp.organization_id, wp.student_id, s.name, wp.title, wp.source, wp.status, wp.generated_by_type, wp.generated_by_user_id, wp.created_at, wp.updated_at
LIMIT 1`, id, orgID)
	paper, err := scanPaperSummary(row)
	if err != nil {
		return wrongbook.Paper{}, translateError(err)
	}
	questions, err := r.listPaperQuestions(ctx, orgID, id)
	if err != nil {
		return wrongbook.Paper{}, err
	}
	paper.Questions = questions
	paper.QuestionCount = len(questions)
	return paper, nil
}

func (r *Repository) CreatePaper(ctx context.Context, orgID uint64, params wrongbook.CreatePaperParams) (wrongbook.Paper, error) {
	if len(params.QuestionIDs) == 0 {
		return wrongbook.Paper{}, fmt.Errorf("%w: no questions selected", wrongbook.ErrInvalidState)
	}
	apply := func(ctx context.Context, store *Repository) (wrongbook.Paper, error) {
		seen := make(map[uint64]struct{}, len(params.QuestionIDs))
		questionIDs := make([]uint64, 0, len(params.QuestionIDs))
		for _, id := range params.QuestionIDs {
			if id == 0 {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			question, err := store.FindQuestion(ctx, orgID, id)
			if err != nil {
				return wrongbook.Paper{}, err
			}
			if question.StudentID != params.StudentID {
				return wrongbook.Paper{}, fmt.Errorf("%w: question %d belongs to another student", wrongbook.ErrInvalidState, id)
			}
			if question.Status == wrongbook.QuestionStatusArchived {
				return wrongbook.Paper{}, fmt.Errorf("%w: question %d is archived", wrongbook.ErrInvalidState, id)
			}
			questionIDs = append(questionIDs, id)
		}
		if len(questionIDs) == 0 {
			return wrongbook.Paper{}, fmt.Errorf("%w: no questions selected", wrongbook.ErrInvalidState)
		}
		title := strings.TrimSpace(params.Title)
		if title == "" {
			title = defaultPaperTitle(time.Now().UTC())
		}
		result, err := store.exec.ExecContext(ctx, `INSERT INTO wrong_papers (
    organization_id, student_id, title, source, status, generated_by_type, generated_by_user_id
) VALUES (?, ?, ?, ?, 'generated', ?, ?)`,
			orgID,
			params.StudentID,
			title,
			defaultPaperSource(params.Source),
			defaultGeneratedBy(params.GeneratedByType),
			nullID(params.GeneratedByUserID),
		)
		if err != nil {
			return wrongbook.Paper{}, translateError(err)
		}
		id, err := result.LastInsertId()
		if err != nil || id <= 0 {
			return wrongbook.Paper{}, fmt.Errorf("read created wrong paper id: %w", err)
		}
		for index, questionID := range questionIDs {
			if _, err := store.exec.ExecContext(ctx, `INSERT INTO wrong_paper_questions (organization_id, paper_id, question_id, sort_order) VALUES (?, ?, ?, ?)`, orgID, uint64(id), questionID, index+1); err != nil {
				return wrongbook.Paper{}, translateError(err)
			}
		}
		return store.FindPaper(ctx, orgID, uint64(id))
	}
	if beginner, ok := r.exec.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}); ok {
		tx, err := beginner.BeginTx(ctx, nil)
		if err != nil {
			return wrongbook.Paper{}, err
		}
		paper, applyErr := apply(ctx, New(tx))
		if applyErr != nil {
			_ = tx.Rollback()
			return wrongbook.Paper{}, applyErr
		}
		if err := tx.Commit(); err != nil {
			return wrongbook.Paper{}, err
		}
		return paper, nil
	}
	return apply(ctx, r)
}

func (r *Repository) listPaperQuestions(ctx context.Context, orgID, paperID uint64) ([]wrongbook.Question, error) {
	rows, err := r.exec.QueryContext(ctx, `SELECT wq.id, wq.organization_id, wq.student_id, s.name AS student_name,
       wq.subject, wq.question_text, wq.answer_text, wq.explanation, wq.knowledge_point,
       wq.source_image_url, wq.source_homework_task_id, wq.teacher_note, wq.status,
       wq.created_by_user_id, wq.created_by_name, wq.last_reviewed_at, wq.created_at, wq.updated_at
FROM wrong_paper_questions wpq
JOIN wrong_questions wq ON wq.id = wpq.question_id
JOIN students s ON s.id = wq.student_id
WHERE wpq.paper_id = ? AND wpq.organization_id = ? AND wq.organization_id = ?
ORDER BY wpq.sort_order, wpq.id`, paperID, orgID, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]wrongbook.Question, 0)
	for rows.Next() {
		item, scanErr := scanQuestion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}
	return out, nil
}

type questionScanner interface {
	Scan(dest ...any) error
}

func scanQuestion(scanner questionScanner) (wrongbook.Question, error) {
	var item wrongbook.Question
	var sourceHomeworkTaskID, createdByUserID sql.NullInt64
	var lastReviewedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.OrganizationID,
		&item.StudentID,
		&item.StudentName,
		&item.Subject,
		&item.QuestionText,
		&item.AnswerText,
		&item.Explanation,
		&item.KnowledgePoint,
		&item.SourceImageURL,
		&sourceHomeworkTaskID,
		&item.TeacherNote,
		&item.Status,
		&createdByUserID,
		&item.CreatedByName,
		&lastReviewedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return wrongbook.Question{}, err
	}
	item.SourceHomeworkTaskID = idPtr(sourceHomeworkTaskID)
	item.CreatedByUserID = idPtr(createdByUserID)
	item.LastReviewedAt = timePtr(lastReviewedAt)
	return item, nil
}

func scanPaperSummary(scanner questionScanner) (wrongbook.Paper, error) {
	var item wrongbook.Paper
	var generatedByUserID sql.NullInt64
	if err := scanner.Scan(
		&item.ID,
		&item.OrganizationID,
		&item.StudentID,
		&item.StudentName,
		&item.Title,
		&item.Source,
		&item.Status,
		&item.GeneratedByType,
		&generatedByUserID,
		&item.QuestionCount,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return wrongbook.Paper{}, err
	}
	item.GeneratedByUserID = idPtr(generatedByUserID)
	return item, nil
}

func validQuestionStatus(status string) bool {
	return status == wrongbook.QuestionStatusActive || status == wrongbook.QuestionStatusMastered || status == wrongbook.QuestionStatusArchived
}

func defaultPaperTitle(now time.Time) string {
	return now.Format("2006-01-02") + " 错题复习卷"
}

func defaultPaperSource(value string) string {
	switch value {
	case wrongbook.PaperSourceParent, wrongbook.PaperSourceSystem, wrongbook.PaperSourceTeacher:
		return value
	default:
		return wrongbook.PaperSourceTeacher
	}
}

func defaultGeneratedBy(value string) string {
	switch value {
	case wrongbook.GeneratedByParent, wrongbook.GeneratedByStaff, wrongbook.GeneratedBySystem:
		return value
	default:
		return wrongbook.GeneratedByStaff
	}
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

func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return wrongbook.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return wrongbook.ErrConflict
	}
	if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint fails") {
		return fmt.Errorf("%w: invalid relation", wrongbook.ErrNotFound)
	}
	return err
}
