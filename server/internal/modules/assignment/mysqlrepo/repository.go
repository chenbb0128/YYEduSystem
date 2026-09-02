package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database/sqlc"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct{ queries *sqlc.Queries }

func New(exec database.DBTX) *Repository { return &Repository{queries: sqlc.New(exec)} }

func (r *Repository) List(ctx context.Context, orgID, teacherUserID, schoolClassID uint64) ([]assignment.TeacherClassAssignment, error) {
	items, err := r.queries.ListTeacherClassAssignments(ctx, sqlc.ListTeacherClassAssignmentsParams{OrganizationID: orgID, TeacherUserID: teacherUserID, SchoolClassID: schoolClassID})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]assignment.TeacherClassAssignment, 0, len(items))
	for _, item := range items {
		out = append(out, mapAssignment(item))
	}
	return out, nil
}

func (r *Repository) Find(ctx context.Context, orgID, id uint64) (assignment.TeacherClassAssignment, error) {
	item, err := r.queries.GetTeacherClassAssignment(ctx, sqlc.GetTeacherClassAssignmentParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return assignment.TeacherClassAssignment{}, translateError(err)
	}
	return mapAssignment(item), nil
}

func (r *Repository) FindByPair(ctx context.Context, orgID, teacherUserID, schoolClassID uint64) (assignment.TeacherClassAssignment, error) {
	item, err := r.queries.GetTeacherClassAssignmentByPair(ctx, sqlc.GetTeacherClassAssignmentByPairParams{OrganizationID: orgID, TeacherUserID: teacherUserID, SchoolClassID: schoolClassID})
	if err != nil {
		return assignment.TeacherClassAssignment{}, translateError(err)
	}
	return mapAssignment(item), nil
}

func (r *Repository) Create(ctx context.Context, orgID uint64, params assignment.CreateParams) (assignment.TeacherClassAssignment, error) {
	result, err := r.queries.CreateTeacherClassAssignment(ctx, sqlc.CreateTeacherClassAssignmentParams{OrganizationID: orgID, TeacherUserID: params.TeacherUserID, SchoolClassID: params.SchoolClassID})
	if err != nil {
		return assignment.TeacherClassAssignment{}, translateError(err)
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		return assignment.TeacherClassAssignment{}, fmt.Errorf("read created assignment id: %w", err)
	}
	return r.Find(ctx, orgID, uint64(id))
}

func (r *Repository) SetStatus(ctx context.Context, orgID uint64, params assignment.SetStatusParams) (assignment.TeacherClassAssignment, error) {
	if params.Status != assignment.AssignmentStatusActive && params.Status != assignment.AssignmentStatusDisabled {
		return assignment.TeacherClassAssignment{}, assignment.ErrInvalidStatus
	}
	result, err := r.queries.SetTeacherClassAssignmentStatus(ctx, sqlc.SetTeacherClassAssignmentStatusParams{Status: params.Status, ID: params.ID, OrganizationID: orgID})
	if err != nil {
		return assignment.TeacherClassAssignment{}, translateError(err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return assignment.TeacherClassAssignment{}, err
	}
	if affected == 0 {
		return assignment.TeacherClassAssignment{}, assignment.ErrNotFound
	}
	return r.Find(ctx, orgID, params.ID)
}

func mapAssignment(item sqlc.TeacherClassAssignment) assignment.TeacherClassAssignment {
	return assignment.TeacherClassAssignment{ID: item.ID, OrganizationID: item.OrganizationID, TeacherUserID: item.TeacherUserID, SchoolClassID: item.SchoolClassID, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return assignment.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return assignment.ErrConflict
	}
	if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint fails") {
		return fmt.Errorf("%w: invalid relation", assignment.ErrNotFound)
	}
	return err
}
