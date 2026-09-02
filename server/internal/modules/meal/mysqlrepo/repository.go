package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/meal"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database/sqlc"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct {
	queries *sqlc.Queries
	exec    database.DBTX
}

func New(exec database.DBTX) *Repository { return &Repository{queries: sqlc.New(exec), exec: exec} }

func (r *Repository) ListPlans(ctx context.Context, orgID uint64, from, to *time.Time) ([]meal.Plan, error) {
	items, err := r.queries.ListMealPlans(ctx, sqlc.ListMealPlansParams{OrganizationID: orgID, FromDate: nullDate(from), ToDate: nullDate(to)})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]meal.Plan, 0, len(items))
	for _, item := range items {
		out = append(out, mapPlan(item))
	}
	return out, nil
}
func (r *Repository) FindPlan(ctx context.Context, orgID, id uint64) (meal.Plan, error) {
	item, err := r.queries.GetMealPlanByID(ctx, sqlc.GetMealPlanByIDParams{OrganizationID: orgID, ID: id})
	if err != nil {
		return meal.Plan{}, translateError(err)
	}
	return mapPlan(item), nil
}
func (r *Repository) UpsertPlan(ctx context.Context, orgID uint64, params meal.UpsertPlanParams) (meal.Plan, error) {
	current, err := r.queries.GetMealPlanByDate(ctx, sqlc.GetMealPlanByDateParams{OrganizationID: orgID, MealDate: dateOnly(params.MealDate)})
	if err == nil {
		_, updateErr := r.queries.UpdateMealPlan(ctx, sqlc.UpdateMealPlanParams{MenuText: strings.TrimSpace(params.MenuText), PhotoUrl: strings.TrimSpace(params.PhotoURL), AdjustmentNote: strings.TrimSpace(params.AdjustmentNote), CreatedByUserID: nullID(params.CreatedByUserID), CreatedByName: strings.TrimSpace(params.CreatedByName), OrganizationID: orgID, ID: current.ID})
		if updateErr != nil {
			return meal.Plan{}, translateError(updateErr)
		}
		return r.FindPlan(ctx, orgID, current.ID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return meal.Plan{}, translateError(err)
	}
	result, createErr := r.queries.CreateMealPlan(ctx, sqlc.CreateMealPlanParams{OrganizationID: orgID, MealDate: dateOnly(params.MealDate), MenuText: strings.TrimSpace(params.MenuText), PhotoUrl: strings.TrimSpace(params.PhotoURL), AdjustmentNote: strings.TrimSpace(params.AdjustmentNote), CreatedByUserID: nullID(params.CreatedByUserID), CreatedByName: strings.TrimSpace(params.CreatedByName)})
	if createErr != nil {
		if errors.Is(translateError(createErr), meal.ErrConflict) {
			current, findErr := r.queries.GetMealPlanByDate(ctx, sqlc.GetMealPlanByDateParams{OrganizationID: orgID, MealDate: dateOnly(params.MealDate)})
			if findErr == nil {
				_, updateErr := r.queries.UpdateMealPlan(ctx, sqlc.UpdateMealPlanParams{MenuText: params.MenuText, PhotoUrl: params.PhotoURL, AdjustmentNote: params.AdjustmentNote, CreatedByUserID: nullID(params.CreatedByUserID), CreatedByName: params.CreatedByName, OrganizationID: orgID, ID: current.ID})
				if updateErr == nil {
					return r.FindPlan(ctx, orgID, current.ID)
				}
			}
		}
		return meal.Plan{}, translateError(createErr)
	}
	id, e := insertedID(result)
	if e != nil {
		return meal.Plan{}, e
	}
	return r.FindPlan(ctx, orgID, id)
}
func (r *Repository) CopyPlan(ctx context.Context, orgID uint64, params meal.CopyPlanParams) (meal.Plan, error) {
	source, err := r.queries.GetMealPlanByDate(ctx, sqlc.GetMealPlanByDateParams{OrganizationID: orgID, MealDate: dateOnly(params.SourceDate)})
	if err != nil {
		return meal.Plan{}, translateError(err)
	}
	return r.UpsertPlan(ctx, orgID, meal.UpsertPlanParams{MealDate: params.TargetDate, MenuText: source.MenuText, AdjustmentNote: source.AdjustmentNote, CreatedByUserID: params.CreatedByUserID, CreatedByName: params.CreatedByName})
}
func (r *Repository) ListDietNotes(ctx context.Context, orgID uint64, studentID *uint64) ([]meal.DietNote, error) {
	items, err := r.queries.ListStudentDietNotes(ctx, sqlc.ListStudentDietNotesParams{OrganizationID: orgID, StudentIDFilter: nullID(studentID)})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]meal.DietNote, 0, len(items))
	for _, item := range items {
		out = append(out, mapDietNote(item))
	}
	return out, nil
}
func (r *Repository) UpsertDietNote(ctx context.Context, orgID uint64, params meal.UpsertDietNoteParams) (meal.DietNote, error) {
	current, err := r.queries.GetStudentDietNote(ctx, sqlc.GetStudentDietNoteParams{OrganizationID: orgID, StudentID: params.StudentID})
	if err == nil {
		_, e := r.queries.UpdateStudentDietNote(ctx, sqlc.UpdateStudentDietNoteParams{Note: params.Note, UpdatedByUserID: nullID(params.UpdatedByUserID), UpdatedByName: params.UpdatedByName, OrganizationID: orgID, StudentID: params.StudentID})
		if e != nil {
			return meal.DietNote{}, translateError(e)
		}
		return r.getDietNote(ctx, orgID, current.ID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return meal.DietNote{}, translateError(err)
	}
	result, e := r.queries.CreateStudentDietNote(ctx, sqlc.CreateStudentDietNoteParams{OrganizationID: orgID, StudentID: params.StudentID, Note: params.Note, UpdatedByUserID: nullID(params.UpdatedByUserID), UpdatedByName: params.UpdatedByName})
	if e != nil {
		return meal.DietNote{}, translateError(e)
	}
	id, e := insertedID(result)
	if e != nil {
		return meal.DietNote{}, e
	}
	return r.getDietNote(ctx, orgID, id)
}

func (r *Repository) ListDietNoteChangeRequests(ctx context.Context, orgID uint64, studentID *uint64, status *string) ([]meal.DietNoteChangeRequest, error) {
	items, err := r.queries.ListDietNoteChangeRequests(ctx, sqlc.ListDietNoteChangeRequestsParams{OrganizationID: orgID, StudentIDFilter: nullID(studentID), StatusFilter: nullString(status)})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]meal.DietNoteChangeRequest, 0, len(items))
	for _, item := range items {
		out = append(out, mapDietNoteChangeRequest(item))
	}
	return out, nil
}

func (r *Repository) CreateDietNoteChangeRequest(ctx context.Context, orgID uint64, params meal.CreateDietNoteChangeRequestParams) (request meal.DietNoteChangeRequest, err error) {
	err = r.withTransaction(ctx, func(q *sqlc.Queries) error {
		if _, findErr := q.GetPendingDietNoteChangeRequest(ctx, sqlc.GetPendingDietNoteChangeRequestParams{OrganizationID: orgID, StudentID: params.StudentID}); findErr == nil {
			return meal.ErrConflict
		} else if !errors.Is(findErr, sql.ErrNoRows) {
			return translateError(findErr)
		}
		currentNote := ""
		current, findErr := q.GetStudentDietNote(ctx, sqlc.GetStudentDietNoteParams{OrganizationID: orgID, StudentID: params.StudentID})
		if findErr == nil {
			currentNote = current.Note
		} else if !errors.Is(findErr, sql.ErrNoRows) {
			return translateError(findErr)
		}
		result, createErr := q.CreateDietNoteChangeRequest(ctx, sqlc.CreateDietNoteChangeRequestParams{OrganizationID: orgID, StudentID: params.StudentID, ParentAccountID: params.ParentAccountID, CurrentNote: currentNote, RequestedNote: strings.TrimSpace(params.RequestedNote)})
		if createErr != nil {
			return translateError(createErr)
		}
		id, idErr := insertedID(result)
		if idErr != nil {
			return idErr
		}
		item, getErr := q.GetDietNoteChangeRequest(ctx, sqlc.GetDietNoteChangeRequestParams{OrganizationID: orgID, ID: id})
		if getErr != nil {
			return translateError(getErr)
		}
		request = mapDietNoteChangeRequest(item)
		return nil
	})
	return request, err
}

func (r *Repository) ReviewDietNoteChangeRequest(ctx context.Context, orgID uint64, params meal.ReviewDietNoteChangeRequestParams) (request meal.DietNoteChangeRequest, err error) {
	if params.Status != meal.DietNoteChangeStatusApproved && params.Status != meal.DietNoteChangeStatusRejected {
		return meal.DietNoteChangeRequest{}, meal.ErrInvalidState
	}
	err = r.withTransaction(ctx, func(q *sqlc.Queries) error {
		item, getErr := q.GetDietNoteChangeRequest(ctx, sqlc.GetDietNoteChangeRequestParams{OrganizationID: orgID, ID: params.ID})
		if getErr != nil {
			return translateError(getErr)
		}
		if item.Status != meal.DietNoteChangeStatusPending {
			return meal.ErrInvalidState
		}
		if params.Status == meal.DietNoteChangeStatusApproved {
			_, noteErr := q.GetStudentDietNote(ctx, sqlc.GetStudentDietNoteParams{OrganizationID: orgID, StudentID: item.StudentID})
			if noteErr == nil {
				if _, updateErr := q.UpdateStudentDietNote(ctx, sqlc.UpdateStudentDietNoteParams{Note: item.RequestedNote, UpdatedByUserID: nullID(&params.ReviewedByUserID), UpdatedByName: "审核人", OrganizationID: orgID, StudentID: item.StudentID}); updateErr != nil {
					return translateError(updateErr)
				}
			} else if errors.Is(noteErr, sql.ErrNoRows) {
				if _, createErr := q.CreateStudentDietNote(ctx, sqlc.CreateStudentDietNoteParams{OrganizationID: orgID, StudentID: item.StudentID, Note: item.RequestedNote, UpdatedByUserID: nullID(&params.ReviewedByUserID), UpdatedByName: "审核人"}); createErr != nil {
					return translateError(createErr)
				}
			} else {
				return translateError(noteErr)
			}
		}
		reviewedAt := sql.NullTime{Time: time.Now().UTC(), Valid: true}
		result, reviewErr := q.ReviewDietNoteChangeRequest(ctx, sqlc.ReviewDietNoteChangeRequestParams{Status: params.Status, ReviewNote: strings.TrimSpace(params.ReviewNote), ReviewedByUserID: nullID(&params.ReviewedByUserID), ReviewedAt: reviewedAt, OrganizationID: orgID, ID: params.ID})
		if reviewErr != nil {
			return translateError(reviewErr)
		}
		if err := ensureAffected(result); err != nil {
			return meal.ErrInvalidState
		}
		updated, getErr := q.GetDietNoteChangeRequest(ctx, sqlc.GetDietNoteChangeRequestParams{OrganizationID: orgID, ID: params.ID})
		if getErr != nil {
			return translateError(getErr)
		}
		request = mapDietNoteChangeRequest(updated)
		return nil
	})
	return request, err
}
func (r *Repository) getDietNote(ctx context.Context, orgID, id uint64) (meal.DietNote, error) {
	items, err := r.queries.ListStudentDietNotes(ctx, sqlc.ListStudentDietNotesParams{OrganizationID: orgID, StudentIDFilter: sql.NullInt64{}})
	if err != nil {
		return meal.DietNote{}, translateError(err)
	}
	for _, item := range items {
		if item.ID == id {
			return mapDietNote(item), nil
		}
	}
	return meal.DietNote{}, meal.ErrNotFound
}

func mapPlan(item sqlc.MealPlan) meal.Plan {
	return meal.Plan{ID: item.ID, OrganizationID: item.OrganizationID, MealDate: item.MealDate, MenuText: item.MenuText, PhotoURL: item.PhotoUrl, AdjustmentNote: item.AdjustmentNote, CreatedByUserID: idPtr(item.CreatedByUserID), CreatedByName: item.CreatedByName, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapDietNote(item sqlc.StudentDietNote) meal.DietNote {
	return meal.DietNote{ID: item.ID, OrganizationID: item.OrganizationID, StudentID: item.StudentID, Note: item.Note, UpdatedByUserID: idPtr(item.UpdatedByUserID), UpdatedByName: item.UpdatedByName, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapDietNoteChangeRequest(item sqlc.DietNoteChangeRequest) meal.DietNoteChangeRequest {
	return meal.DietNoteChangeRequest{ID: item.ID, OrganizationID: item.OrganizationID, StudentID: item.StudentID, ParentAccountID: item.ParentAccountID, CurrentNote: item.CurrentNote, RequestedNote: item.RequestedNote, Status: item.Status, ReviewNote: item.ReviewNote, ReviewedByUserID: idPtr(item.ReviewedByUserID), ReviewedAt: timePtr(item.ReviewedAt), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func nullID(value *uint64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}
func nullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: strings.TrimSpace(*value), Valid: true}
}
func idPtr(value sql.NullInt64) *uint64 {
	if !value.Valid {
		return nil
	}
	v := uint64(value.Int64)
	return &v
}
func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	copy := value.Time
	return &copy
}
func nullDate(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: dateOnly(*value), Valid: true}
}
func dateOnly(value time.Time) time.Time {
	v := value.UTC()
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
}
func insertedID(result sql.Result) (uint64, error) {
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, fmt.Errorf("invalid inserted id %d", id)
	}
	return uint64(id), nil
}
func ensureAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return meal.ErrInvalidState
	}
	return nil
}
func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return meal.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return meal.ErrConflict
	}
	if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint fails") {
		return fmt.Errorf("%w: invalid relation", meal.ErrNotFound)
	}
	return err
}
