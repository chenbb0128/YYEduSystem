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

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/summary"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database/sqlc"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct {
	queries *sqlc.Queries
	exec    database.DBTX
}

func New(exec database.DBTX) *Repository {
	return &Repository{queries: sqlc.New(exec), exec: exec}
}

func (r *Repository) List(ctx context.Context, orgID uint64, date *time.Time) ([]summary.DailySummary, error) {
	filter := sql.NullTime{}
	if date != nil {
		filter = sql.NullTime{Time: dateOnly(*date), Valid: true}
	}
	items, err := r.queries.ListDailySummaries(ctx, sqlc.ListDailySummariesParams{OrganizationID: orgID, SummaryDateFilter: filter})
	if err != nil {
		return nil, translate(err)
	}
	out := make([]summary.DailySummary, 0, len(items))
	for _, item := range items {
		mapped, err := mapListItem(item)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped)
	}
	return out, nil
}

func (r *Repository) Find(ctx context.Context, orgID, id uint64) (summary.DailySummary, error) {
	item, err := r.queries.GetDailySummaryByID(ctx, sqlc.GetDailySummaryByIDParams{OrganizationID: orgID, ID: id})
	if err != nil {
		return summary.DailySummary{}, translate(err)
	}
	return mapByIDItem(item)
}

func (r *Repository) Generate(ctx context.Context, orgID uint64, p summary.GenerateParams) (summary.DailySummary, error) {
	updates, err := json.Marshal(p.ChildUpdates)
	if err != nil {
		return summary.DailySummary{}, fmt.Errorf("encode summary child updates: %w", err)
	}
	now := time.Now().UTC()
	var id uint64
	err = r.withQueries(ctx, func(q *sqlc.Queries) error {
		existing, getErr := q.GetDailySummaryByDate(ctx, sqlc.GetDailySummaryByDateParams{OrganizationID: orgID, SummaryDate: dateOnly(p.SummaryDate)})
		if getErr == nil {
			if existing.Status != summary.StatusDraft {
				return summary.ErrInvalidState
			}
			id = existing.ID
			version := existing.CurrentVersion + 1
			result, updateErr := q.UpdateDailySummary(ctx, sqlc.UpdateDailySummaryParams{Content: strings.TrimSpace(p.Content), ChildUpdatesJson: string(updates), OrganizationID: orgID, ID: id})
			if updateErr != nil {
				return translate(updateErr)
			}
			if affected, _ := result.RowsAffected(); affected == 0 {
				return summary.ErrInvalidState
			}
			return q.CreateDailySummaryVersion(ctx, versionParams(orgID, id, version, "generated", strings.TrimSpace(p.Content), string(updates), "", idPtr(existing.CreatedByUserID), existing.CreatedByName, now))
		}
		if !errors.Is(getErr, sql.ErrNoRows) {
			return translate(getErr)
		}
		result, createErr := q.CreateDailySummary(ctx, sqlc.CreateDailySummaryParams{OrganizationID: orgID, SummaryDate: dateOnly(p.SummaryDate), Content: strings.TrimSpace(p.Content), ChildUpdatesJson: string(updates), CreatedByUserID: nullID(p.CreatedByUserID), CreatedByName: strings.TrimSpace(p.CreatedByName), GeneratedAt: sql.NullTime{Time: now, Valid: true}})
		if createErr != nil {
			return translate(createErr)
		}
		id, createErr = insertedID(result)
		if createErr != nil {
			return createErr
		}
		return q.CreateDailySummaryVersion(ctx, versionParams(orgID, id, 1, "generated", strings.TrimSpace(p.Content), string(updates), "", p.CreatedByUserID, p.CreatedByName, now))
	})
	if err != nil {
		return summary.DailySummary{}, err
	}
	return r.Find(ctx, orgID, id)
}

func (r *Repository) Update(ctx context.Context, orgID uint64, p summary.UpdateParams) (summary.DailySummary, error) {
	updates, err := json.Marshal(p.ChildUpdates)
	if err != nil {
		return summary.DailySummary{}, fmt.Errorf("encode summary child updates: %w", err)
	}
	now := time.Now().UTC()
	err = r.withQueries(ctx, func(q *sqlc.Queries) error {
		existing, getErr := q.GetDailySummaryByID(ctx, sqlc.GetDailySummaryByIDParams{OrganizationID: orgID, ID: p.ID})
		if getErr != nil {
			return translate(getErr)
		}
		if existing.Status != summary.StatusDraft {
			return summary.ErrInvalidState
		}
		result, updateErr := q.UpdateDailySummary(ctx, sqlc.UpdateDailySummaryParams{Content: strings.TrimSpace(p.Content), ChildUpdatesJson: string(updates), OrganizationID: orgID, ID: p.ID})
		if updateErr != nil {
			return translate(updateErr)
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return summary.ErrInvalidState
		}
		return q.CreateDailySummaryVersion(ctx, versionParams(orgID, p.ID, existing.CurrentVersion+1, "updated", strings.TrimSpace(p.Content), string(updates), "", idPtr(existing.CreatedByUserID), existing.CreatedByName, now))
	})
	if err != nil {
		return summary.DailySummary{}, err
	}
	return r.Find(ctx, orgID, p.ID)
}

func (r *Repository) SetStatus(ctx context.Context, orgID, id uint64, status string) (summary.DailySummary, error) {
	now := time.Now().UTC()
	err := r.withQueries(ctx, func(q *sqlc.Queries) error {
		existing, getErr := q.GetDailySummaryByID(ctx, sqlc.GetDailySummaryByIDParams{OrganizationID: orgID, ID: id})
		if getErr != nil {
			return translate(getErr)
		}
		var result sql.Result
		var updateErr error
		switch status {
		case summary.StatusPublished:
			if existing.Status != summary.StatusDraft {
				return summary.ErrInvalidState
			}
			result, updateErr = q.PublishDailySummary(ctx, sqlc.PublishDailySummaryParams{PublishedAt: sql.NullTime{Time: now, Valid: true}, OrganizationID: orgID, ID: id})
		case summary.StatusClosed:
			if existing.Status != summary.StatusPublished && existing.Status != summary.StatusDraft {
				return summary.ErrInvalidState
			}
			result, updateErr = q.CloseDailySummary(ctx, sqlc.CloseDailySummaryParams{ClosedAt: sql.NullTime{Time: now, Valid: true}, OrganizationID: orgID, ID: id})
		default:
			return summary.ErrInvalidState
		}
		if updateErr != nil {
			return translate(updateErr)
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return summary.ErrInvalidState
		}
		return q.CreateDailySummaryVersion(ctx, versionParams(orgID, id, existing.CurrentVersion, status, existing.Content, existing.ChildUpdatesJson, "", idPtr(existing.CreatedByUserID), existing.CreatedByName, now))
	})
	if err != nil {
		return summary.DailySummary{}, err
	}
	return r.Find(ctx, orgID, id)
}

func (r *Repository) Withdraw(ctx context.Context, orgID, id uint64, reason string) (summary.DailySummary, error) {
	now := time.Now().UTC()
	err := r.withQueries(ctx, func(q *sqlc.Queries) error {
		existing, getErr := q.GetDailySummaryByID(ctx, sqlc.GetDailySummaryByIDParams{OrganizationID: orgID, ID: id})
		if getErr != nil {
			return translate(getErr)
		}
		if existing.Status != summary.StatusPublished {
			return summary.ErrInvalidState
		}
		result, updateErr := q.WithdrawDailySummary(ctx, sqlc.WithdrawDailySummaryParams{WithdrawnAt: sql.NullTime{Time: now, Valid: true}, WithdrawalReason: strings.TrimSpace(reason), OrganizationID: orgID, ID: id})
		if updateErr != nil {
			return translate(updateErr)
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return summary.ErrInvalidState
		}
		return q.CreateDailySummaryVersion(ctx, versionParams(orgID, id, existing.CurrentVersion, "withdrawn", existing.Content, existing.ChildUpdatesJson, reason, idPtr(existing.CreatedByUserID), existing.CreatedByName, now))
	})
	if err != nil {
		return summary.DailySummary{}, err
	}
	return r.Find(ctx, orgID, id)
}

func (r *Repository) Correct(ctx context.Context, orgID uint64, p summary.CorrectParams) (summary.DailySummary, error) {
	updates, err := json.Marshal(p.ChildUpdates)
	if err != nil {
		return summary.DailySummary{}, fmt.Errorf("encode summary child updates: %w", err)
	}
	now := time.Now().UTC()
	err = r.withQueries(ctx, func(q *sqlc.Queries) error {
		existing, getErr := q.GetDailySummaryByID(ctx, sqlc.GetDailySummaryByIDParams{OrganizationID: orgID, ID: p.ID})
		if getErr != nil {
			return translate(getErr)
		}
		if existing.Status != summary.StatusPublished && existing.Status != summary.StatusClosed && existing.Status != summary.StatusWithdrawn {
			return summary.ErrInvalidState
		}
		result, updateErr := q.CorrectDailySummary(ctx, sqlc.CorrectDailySummaryParams{Content: strings.TrimSpace(p.Content), ChildUpdatesJson: string(updates), PublishedAt: sql.NullTime{Time: now, Valid: true}, CorrectionReason: strings.TrimSpace(p.Reason), CreatedByUserID: nullID(p.CreatedByUserID), CreatedByName: strings.TrimSpace(p.CreatedByName), OrganizationID: orgID, ID: p.ID})
		if updateErr != nil {
			return translate(updateErr)
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return summary.ErrInvalidState
		}
		return q.CreateDailySummaryVersion(ctx, versionParams(orgID, p.ID, existing.CurrentVersion+1, "corrected", strings.TrimSpace(p.Content), string(updates), p.Reason, p.CreatedByUserID, p.CreatedByName, now))
	})
	if err != nil {
		return summary.DailySummary{}, err
	}
	return r.Find(ctx, orgID, p.ID)
}

func (r *Repository) ListVersions(ctx context.Context, orgID, summaryID uint64) ([]summary.Version, error) {
	items, err := r.queries.ListDailySummaryVersions(ctx, sqlc.ListDailySummaryVersionsParams{OrganizationID: orgID, DailySummaryID: summaryID})
	if err != nil {
		return nil, translate(err)
	}
	out := make([]summary.Version, 0, len(items))
	for _, item := range items {
		updates := map[uint64]string{}
		if strings.TrimSpace(item.ChildUpdatesJson) != "" {
			if err := json.Unmarshal([]byte(item.ChildUpdatesJson), &updates); err != nil {
				return nil, fmt.Errorf("decode summary version child updates: %w", err)
			}
		}
		out = append(out, summary.Version{ID: item.ID, SummaryID: item.DailySummaryID, Version: item.Version, Action: item.Action, Content: item.Content, ChildUpdates: updates, Reason: item.Reason, CreatedByUserID: idPtr(item.CreatedByUserID), CreatedByName: item.CreatedByName, CreatedAt: item.CreatedAt})
	}
	return out, nil
}

func (r *Repository) MarkRead(ctx context.Context, orgID, summaryID, parentID uint64, version uint32) error {
	err := r.queries.MarkDailySummaryRead(ctx, sqlc.MarkDailySummaryReadParams{OrganizationID: orgID, DailySummaryID: summaryID, ParentAccountID: parentID, ReadVersion: version, ReadAt: time.Now().UTC()})
	return translate(err)
}

func (r *Repository) ReadAt(ctx context.Context, orgID, summaryID, parentID uint64) (*time.Time, error) {
	value, err := r.queries.GetDailySummaryReadAt(ctx, sqlc.GetDailySummaryReadAtParams{OrganizationID: orgID, DailySummaryID: summaryID, ParentAccountID: parentID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, translate(err)
	}
	if value.ReadVersion != value.CurrentVersion {
		return nil, nil
	}
	return &value.ReadAt, nil
}

func (r *Repository) withQueries(ctx context.Context, fn func(*sqlc.Queries) error) error {
	db, ok := r.exec.(*sql.DB)
	if !ok {
		return fn(r.queries)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(sqlc.New(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func versionParams(orgID, summaryID uint64, version uint32, action, content, childUpdates, reason string, userID *uint64, userName string, createdAt time.Time) sqlc.CreateDailySummaryVersionParams {
	return sqlc.CreateDailySummaryVersionParams{OrganizationID: orgID, DailySummaryID: summaryID, Version: version, Action: action, Content: content, ChildUpdatesJson: childUpdates, Reason: strings.TrimSpace(reason), CreatedByUserID: nullID(userID), CreatedByName: strings.TrimSpace(userName), CreatedAt: createdAt}
}

type summaryFields struct {
	id               uint64
	organizationID   uint64
	summaryDate      time.Time
	content          string
	childUpdatesJSON string
	status           string
	currentVersion   uint32
	createdByUserID  sql.NullInt64
	createdByName    string
	generatedAt      sql.NullTime
	publishedAt      sql.NullTime
	closedAt         sql.NullTime
	withdrawnAt      sql.NullTime
	withdrawalReason string
	correctionReason string
	createdAt        time.Time
	updatedAt        time.Time
}

func mapListItem(item sqlc.ListDailySummariesRow) (summary.DailySummary, error) {
	return mapFields(summaryFields{id: item.ID, organizationID: item.OrganizationID, summaryDate: item.SummaryDate, content: item.Content, childUpdatesJSON: item.ChildUpdatesJson, status: item.Status, currentVersion: item.CurrentVersion, createdByUserID: item.CreatedByUserID, createdByName: item.CreatedByName, generatedAt: item.GeneratedAt, publishedAt: item.PublishedAt, closedAt: item.ClosedAt, withdrawnAt: item.WithdrawnAt, withdrawalReason: item.WithdrawalReason, correctionReason: item.CorrectionReason, createdAt: item.CreatedAt, updatedAt: item.UpdatedAt})
}

func mapByIDItem(item sqlc.GetDailySummaryByIDRow) (summary.DailySummary, error) {
	return mapFields(summaryFields{id: item.ID, organizationID: item.OrganizationID, summaryDate: item.SummaryDate, content: item.Content, childUpdatesJSON: item.ChildUpdatesJson, status: item.Status, currentVersion: item.CurrentVersion, createdByUserID: item.CreatedByUserID, createdByName: item.CreatedByName, generatedAt: item.GeneratedAt, publishedAt: item.PublishedAt, closedAt: item.ClosedAt, withdrawnAt: item.WithdrawnAt, withdrawalReason: item.WithdrawalReason, correctionReason: item.CorrectionReason, createdAt: item.CreatedAt, updatedAt: item.UpdatedAt})
}

func mapFields(item summaryFields) (summary.DailySummary, error) {
	updates := map[uint64]string{}
	if strings.TrimSpace(item.childUpdatesJSON) != "" {
		if err := json.Unmarshal([]byte(item.childUpdatesJSON), &updates); err != nil {
			return summary.DailySummary{}, fmt.Errorf("decode summary child updates: %w", err)
		}
	}
	return summary.DailySummary{ID: item.id, OrganizationID: item.organizationID, SummaryDate: item.summaryDate, Content: item.content, ChildUpdates: updates, Status: item.status, Version: item.currentVersion, WithdrawnAt: timePtr(item.withdrawnAt), WithdrawalReason: item.withdrawalReason, CorrectionReason: item.correctionReason, CreatedByUserID: idPtr(item.createdByUserID), CreatedByName: item.createdByName, GeneratedAt: timePtr(item.generatedAt), PublishedAt: timePtr(item.publishedAt), ClosedAt: timePtr(item.closedAt), CreatedAt: item.createdAt, UpdatedAt: item.updatedAt}, nil
}

func nullID(v *uint64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func idPtr(v sql.NullInt64) *uint64 {
	if !v.Valid {
		return nil
	}
	x := uint64(v.Int64)
	return &x
}

func timePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	x := v.Time
	return &x
}

func dateOnly(v time.Time) time.Time {
	v = v.UTC()
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
}

func insertedID(r sql.Result) (uint64, error) {
	id, err := r.LastInsertId()
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("read created summary id: %w", err)
	}
	return uint64(id), nil
}

func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return summary.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return summary.ErrConflict
	}
	if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint fails") {
		return fmt.Errorf("%w: invalid relation", summary.ErrNotFound)
	}
	return err
}
