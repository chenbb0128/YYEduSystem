package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/schedule"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database/sqlc"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct{ queries *sqlc.Queries }

func New(exec database.DBTX) *Repository { return &Repository{queries: sqlc.New(exec)} }

func (r *Repository) List(ctx context.Context, orgID uint64) ([]schedule.PickupSchedule, error) {
	items, err := r.queries.ListPickupSchedules(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]schedule.PickupSchedule, 0, len(items))
	for _, item := range items {
		out = append(out, mapSchedule(item))
	}
	return out, nil
}

func (r *Repository) Find(ctx context.Context, orgID, id uint64) (schedule.PickupSchedule, error) {
	item, err := r.queries.GetPickupScheduleByID(ctx, sqlc.GetPickupScheduleByIDParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return schedule.PickupSchedule{}, translateError(err)
	}
	return mapSchedule(item), nil
}

func (r *Repository) Create(ctx context.Context, orgID uint64, params schedule.CreateParams) (schedule.PickupSchedule, error) {
	if err := validateParams(params); err != nil {
		return schedule.PickupSchedule{}, err
	}
	if err := r.ensureNoOverlap(ctx, orgID, 0, params); err != nil {
		return schedule.PickupSchedule{}, err
	}
	result, err := r.queries.CreatePickupSchedule(ctx, sqlc.CreatePickupScheduleParams{
		OrganizationID: orgID, SchoolID: params.SchoolID, SchoolClassID: params.SchoolClassID, CareClassID: nullID(params.CareClassID), Weekday: uint8(params.Weekday), PickupMode: params.PickupMode, TeacherUserID: nullID(params.TeacherUserID), TeacherName: strings.TrimSpace(params.TeacherName), ExpectedPickupTime: strings.TrimSpace(params.ExpectedPickupTime), EffectiveFrom: params.EffectiveFrom, EffectiveTo: nullTime(params.EffectiveTo), Enabled: params.Enabled, Notes: strings.TrimSpace(params.Notes),
	})
	if err != nil {
		return schedule.PickupSchedule{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return schedule.PickupSchedule{}, err
	}
	return r.Find(ctx, orgID, id)
}

func (r *Repository) Update(ctx context.Context, orgID uint64, params schedule.UpdateParams) (schedule.PickupSchedule, error) {
	if err := validateParams(params.CreateParams); err != nil {
		return schedule.PickupSchedule{}, err
	}
	if err := r.ensureNoOverlap(ctx, orgID, params.ID, params.CreateParams); err != nil {
		return schedule.PickupSchedule{}, err
	}
	result, err := r.queries.UpdatePickupSchedule(ctx, sqlc.UpdatePickupScheduleParams{
		SchoolID: params.SchoolID, SchoolClassID: params.SchoolClassID, CareClassID: nullID(params.CareClassID), Weekday: uint8(params.Weekday), PickupMode: params.PickupMode, TeacherUserID: nullID(params.TeacherUserID), TeacherName: strings.TrimSpace(params.TeacherName), ExpectedPickupTime: strings.TrimSpace(params.ExpectedPickupTime), EffectiveFrom: params.EffectiveFrom, EffectiveTo: nullTime(params.EffectiveTo), Enabled: params.Enabled, Notes: strings.TrimSpace(params.Notes), ID: params.ID, OrganizationID: orgID,
	})
	if err != nil {
		return schedule.PickupSchedule{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return schedule.PickupSchedule{}, err
	}
	return r.Find(ctx, orgID, params.ID)
}

func (r *Repository) ensureNoOverlap(ctx context.Context, orgID, ignoreID uint64, params schedule.CreateParams) error {
	if !params.Enabled {
		return nil
	}
	items, err := r.List(ctx, orgID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == ignoreID || !item.Enabled || item.SchoolClassID != params.SchoolClassID || item.Weekday != params.Weekday {
			continue
		}
		if item.EffectiveTo != nil && item.EffectiveTo.Before(params.EffectiveFrom) {
			continue
		}
		if params.EffectiveTo != nil && params.EffectiveTo.Before(item.EffectiveFrom) {
			continue
		}
		return schedule.ErrConflict
	}
	return nil
}

func validateParams(params schedule.CreateParams) error {
	if params.SchoolID == 0 || params.SchoolClassID == 0 || (params.Weekday != time.Sunday && (params.Weekday < time.Monday || params.Weekday > time.Saturday)) || params.EffectiveFrom.IsZero() {
		return schedule.ErrInvalid
	}
	if params.PickupMode != schedule.PickupModeSchool && params.PickupMode != schedule.PickupModeSelf {
		return schedule.ErrInvalid
	}
	if params.EffectiveTo != nil && params.EffectiveTo.Before(params.EffectiveFrom) {
		return schedule.ErrInvalid
	}
	if len([]rune(strings.TrimSpace(params.ExpectedPickupTime))) > 16 || len([]rune(strings.TrimSpace(params.Notes))) > 500 {
		return schedule.ErrInvalid
	}
	return nil
}

func mapSchedule(item sqlc.PickupSchedule) schedule.PickupSchedule {
	return schedule.PickupSchedule{ID: item.ID, OrganizationID: item.OrganizationID, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, CareClassID: idPtr(item.CareClassID), Weekday: time.Weekday(item.Weekday % 7), PickupMode: item.PickupMode, TeacherUserID: idPtr(item.TeacherUserID), TeacherName: item.TeacherName, ExpectedPickupTime: item.ExpectedPickupTime, EffectiveFrom: item.EffectiveFrom, EffectiveTo: timePtr(item.EffectiveTo), Enabled: item.Enabled, Notes: item.Notes, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func idPtr(value sql.NullInt64) *uint64 {
	if !value.Valid {
		return nil
	}
	item := uint64(value.Int64)
	return &item
}
func nullID(value *uint64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}
func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	item := value.Time
	return &item
}
func nullTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}
func insertedID(result sql.Result) (uint64, error) {
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("schedule: insert returned no id")
	}
	return uint64(id), nil
}
func translateError(err error) error {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return schedule.ErrConflict
	}
	if errors.Is(err, sql.ErrNoRows) {
		return schedule.ErrNotFound
	}
	return err
}
func ensureAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return schedule.ErrNotFound
	}
	return nil
}
