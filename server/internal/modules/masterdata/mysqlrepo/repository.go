package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
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

func (r *Repository) ListSchools(ctx context.Context, orgID uint64) ([]masterdata.School, error) {
	items, err := r.queries.ListSchools(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]masterdata.School, 0, len(items))
	for _, item := range items {
		out = append(out, mapSchool(item))
	}
	return out, nil
}

func (r *Repository) CreateSchool(ctx context.Context, orgID uint64, params masterdata.CreateSchoolParams) (masterdata.School, error) {
	result, err := r.queries.CreateSchool(ctx, sqlc.CreateSchoolParams{OrganizationID: orgID, Name: params.Name, Address: params.Address, ContactPhone: params.ContactPhone, Status: "active"})
	if err != nil {
		return masterdata.School{}, translateError(err)
	}
	return r.findSchoolAfterInsert(ctx, orgID, result)
}

func (r *Repository) ListAcademicTerms(ctx context.Context, orgID uint64) ([]masterdata.AcademicTerm, error) {
	items, err := r.queries.ListAcademicTerms(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]masterdata.AcademicTerm, 0, len(items))
	for _, item := range items {
		out = append(out, mapAcademicTerm(item))
	}
	return out, nil
}

func (r *Repository) CreateAcademicTerm(ctx context.Context, orgID uint64, params masterdata.CreateAcademicTermParams) (masterdata.AcademicTerm, error) {
	if params.IsCurrent {
		if err := r.queries.UnsetCurrentAcademicTerms(ctx, orgID); err != nil {
			return masterdata.AcademicTerm{}, translateError(err)
		}
	}
	result, err := r.queries.CreateAcademicTerm(ctx, sqlc.CreateAcademicTermParams{OrganizationID: orgID, Name: params.Name, StartsOn: params.StartsOn, EndsOn: params.EndsOn, IsCurrent: params.IsCurrent, Status: "active"})
	if err != nil {
		return masterdata.AcademicTerm{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return masterdata.AcademicTerm{}, err
	}
	items, err := r.queries.ListAcademicTerms(ctx, orgID)
	if err != nil {
		return masterdata.AcademicTerm{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return mapAcademicTerm(item), nil
		}
	}
	return masterdata.AcademicTerm{}, masterdata.ErrNotFound
}

func (r *Repository) ListSchoolClasses(ctx context.Context, orgID uint64) ([]masterdata.SchoolClass, error) {
	items, err := r.queries.ListSchoolClasses(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]masterdata.SchoolClass, 0, len(items))
	for _, item := range items {
		out = append(out, mapSchoolClass(item))
	}
	return out, nil
}

func (r *Repository) CreateSchoolClass(ctx context.Context, orgID uint64, params masterdata.CreateSchoolClassParams) (masterdata.SchoolClass, error) {
	result, err := r.queries.CreateSchoolClass(ctx, sqlc.CreateSchoolClassParams{OrganizationID: orgID, SchoolID: params.SchoolID, TermID: params.TermID, Grade: params.Grade, Name: params.Name, Status: "active"})
	if err != nil {
		return masterdata.SchoolClass{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return masterdata.SchoolClass{}, err
	}
	items, err := r.queries.ListSchoolClasses(ctx, orgID)
	if err != nil {
		return masterdata.SchoolClass{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return mapSchoolClass(item), nil
		}
	}
	return masterdata.SchoolClass{}, masterdata.ErrNotFound
}

func (r *Repository) ListCareClasses(ctx context.Context, orgID uint64) ([]masterdata.CareClass, error) {
	items, err := r.queries.ListCareClasses(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]masterdata.CareClass, 0, len(items))
	for _, item := range items {
		out = append(out, mapCareClass(item))
	}
	return out, nil
}

func (r *Repository) CreateCareClass(ctx context.Context, orgID uint64, params masterdata.CreateCareClassParams) (masterdata.CareClass, error) {
	result, err := r.queries.CreateCareClass(ctx, sqlc.CreateCareClassParams{OrganizationID: orgID, Name: params.Name, Capacity: params.Capacity, Status: "active"})
	if err != nil {
		return masterdata.CareClass{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return masterdata.CareClass{}, err
	}
	items, err := r.queries.ListCareClasses(ctx, orgID)
	if err != nil {
		return masterdata.CareClass{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return mapCareClass(item), nil
		}
	}
	return masterdata.CareClass{}, masterdata.ErrNotFound
}

func (r *Repository) ListStudents(ctx context.Context, orgID uint64) ([]masterdata.Student, error) {
	items, err := r.queries.ListStudents(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]masterdata.Student, 0, len(items))
	for _, item := range items {
		out = append(out, mapStudent(item))
	}
	return out, nil
}

func (r *Repository) CreateStudent(ctx context.Context, orgID uint64, params masterdata.CreateStudentParams) (masterdata.Student, error) {
	result, err := r.queries.CreateStudent(ctx, sqlc.CreateStudentParams{OrganizationID: orgID, SchoolID: params.SchoolID, TermID: params.TermID, SchoolClassID: params.SchoolClassID, CareClassID: nullID(params.CareClassID), Name: params.Name, Gender: params.Gender, BirthDate: nullTime(params.BirthDate), StudentNo: params.StudentNo, GuardianPhone: params.GuardianPhone, EmergencyContact: params.EmergencyContact, EmergencyPhone: params.EmergencyPhone, Status: "active", Notes: params.Notes})
	if err != nil {
		return masterdata.Student{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return masterdata.Student{}, err
	}
	return r.FindStudent(ctx, orgID, id)
}

func (r *Repository) BulkCreateStudents(ctx context.Context, orgID uint64, params masterdata.BulkCreateStudentsParams) ([]masterdata.Student, error) {
	if len(params.Items) == 0 {
		return []masterdata.Student{}, nil
	}
	ids := make([]uint64, 0, len(params.Items))
	if err := r.withTransaction(ctx, func(q *sqlc.Queries) error {
		for _, item := range params.Items {
			result, err := q.CreateStudent(ctx, sqlc.CreateStudentParams{
				OrganizationID: orgID, SchoolID: item.SchoolID, TermID: item.TermID,
				SchoolClassID: item.SchoolClassID, CareClassID: nullID(item.CareClassID),
				Name: item.Name, Gender: item.Gender, BirthDate: nullTime(item.BirthDate),
				StudentNo: item.StudentNo, GuardianPhone: item.GuardianPhone,
				EmergencyContact: item.EmergencyContact, EmergencyPhone: item.EmergencyPhone,
				Status: "active", Notes: item.Notes,
			})
			if err != nil {
				return translateError(err)
			}
			id, err := insertedID(result)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	created := make([]masterdata.Student, 0, len(ids))
	for _, id := range ids {
		item, err := r.FindStudent(ctx, orgID, id)
		if err != nil {
			return nil, err
		}
		created = append(created, item)
	}
	return created, nil
}

func (r *Repository) withTransaction(ctx context.Context, fn func(*sqlc.Queries) error) (err error) {
	beginner, ok := r.exec.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	})
	if !ok {
		return fn(r.queries)
	}
	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			panic(recovered)
		}
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		}
	}()
	if err = fn(r.queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) FindStudent(ctx context.Context, orgID, id uint64) (masterdata.Student, error) {
	item, err := r.queries.GetStudentByID(ctx, sqlc.GetStudentByIDParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return masterdata.Student{}, translateError(err)
	}
	return mapStudent(item), nil
}

func (r *Repository) UpdateStudent(ctx context.Context, orgID uint64, params masterdata.UpdateStudentParams) (masterdata.Student, error) {
	result, err := r.queries.UpdateStudent(ctx, sqlc.UpdateStudentParams{SchoolID: params.SchoolID, TermID: params.TermID, SchoolClassID: params.SchoolClassID, CareClassID: nullID(params.CareClassID), Name: params.Name, Gender: params.Gender, BirthDate: nullTime(params.BirthDate), StudentNo: params.StudentNo, GuardianPhone: params.GuardianPhone, EmergencyContact: params.EmergencyContact, EmergencyPhone: params.EmergencyPhone, Status: params.Status, Notes: params.Notes, ID: params.ID, OrganizationID: orgID})
	if err != nil {
		return masterdata.Student{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return masterdata.Student{}, err
	}
	return r.FindStudent(ctx, orgID, params.ID)
}

func (r *Repository) findSchoolAfterInsert(ctx context.Context, orgID uint64, result sql.Result) (masterdata.School, error) {
	id, err := insertedID(result)
	if err != nil {
		return masterdata.School{}, err
	}
	item, err := r.queries.GetSchoolByID(ctx, sqlc.GetSchoolByIDParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return masterdata.School{}, translateError(err)
	}
	return mapSchool(item), nil
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

func mapSchool(item sqlc.School) masterdata.School {
	return masterdata.School{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, Address: item.Address, ContactPhone: item.ContactPhone, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapAcademicTerm(item sqlc.AcademicTerm) masterdata.AcademicTerm {
	return masterdata.AcademicTerm{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, StartsOn: item.StartsOn, EndsOn: item.EndsOn, IsCurrent: item.IsCurrent, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapSchoolClass(item sqlc.SchoolClass) masterdata.SchoolClass {
	return masterdata.SchoolClass{ID: item.ID, OrganizationID: item.OrganizationID, SchoolID: item.SchoolID, TermID: item.TermID, Grade: item.Grade, Name: item.Name, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapCareClass(item sqlc.CareClass) masterdata.CareClass {
	return masterdata.CareClass{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, Capacity: item.Capacity, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapStudent(item sqlc.Student) masterdata.Student {
	var careClassID *uint64
	if item.CareClassID.Valid {
		value := uint64(item.CareClassID.Int64)
		careClassID = &value
	}
	var birthDate *time.Time
	if item.BirthDate.Valid {
		value := item.BirthDate.Time
		birthDate = &value
	}
	return masterdata.Student{ID: item.ID, OrganizationID: item.OrganizationID, SchoolID: item.SchoolID, TermID: item.TermID, SchoolClassID: item.SchoolClassID, CareClassID: careClassID, Name: item.Name, Gender: item.Gender, BirthDate: birthDate, StudentNo: item.StudentNo, GuardianPhone: item.GuardianPhone, EmergencyContact: item.EmergencyContact, EmergencyPhone: item.EmergencyPhone, Status: item.Status, Notes: item.Notes, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
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
	return sql.NullTime{Time: *value, Valid: true}
}

func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return masterdata.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return masterdata.ErrConflict
	}
	if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint fails") {
		return fmt.Errorf("%w: invalid relation", masterdata.ErrNotFound)
	}
	return err
}

func ensureAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return masterdata.ErrNotFound
	}
	return nil
}
