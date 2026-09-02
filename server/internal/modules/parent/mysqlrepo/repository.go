package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database/sqlc"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct{ queries *sqlc.Queries }

func New(exec database.DBTX) *Repository { return &Repository{queries: sqlc.New(exec)} }

func (r *Repository) CreateAccount(ctx context.Context, orgID uint64, params parent.CreateAccountParams) (parent.Account, error) {
	result, err := r.queries.CreateParentAccount(ctx, sqlc.CreateParentAccountParams{OrganizationID: orgID, Openid: strings.TrimSpace(params.OpenID), Nickname: strings.TrimSpace(params.Nickname), Avatar: strings.TrimSpace(params.Avatar)})
	if err != nil {
		return parent.Account{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return parent.Account{}, err
	}
	return r.FindAccountByID(ctx, orgID, id)
}

func (r *Repository) FindAccountByID(ctx context.Context, orgID, id uint64) (parent.Account, error) {
	item, err := r.queries.GetParentAccountByID(ctx, sqlc.GetParentAccountByIDParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return parent.Account{}, translateError(err)
	}
	return mapAccount(item), nil
}

func (r *Repository) FindAccountByOpenID(ctx context.Context, orgID uint64, openID string) (parent.Account, error) {
	item, err := r.queries.GetParentAccountByOpenID(ctx, sqlc.GetParentAccountByOpenIDParams{Openid: strings.TrimSpace(openID), OrganizationID: orgID})
	if err != nil {
		return parent.Account{}, translateError(err)
	}
	return mapAccount(item), nil
}

func (r *Repository) GetLatestPrivacyConsent(ctx context.Context, orgID, parentID uint64) (parent.PrivacyConsent, error) {
	item, err := r.queries.GetLatestParentPrivacyConsent(ctx, sqlc.GetLatestParentPrivacyConsentParams{OrganizationID: orgID, ParentAccountID: parentID})
	if err != nil {
		return parent.PrivacyConsent{}, translateError(err)
	}
	return mapPrivacyConsent(item), nil
}

func (r *Repository) RecordPrivacyConsent(ctx context.Context, orgID, parentID uint64, params parent.RecordPrivacyConsentParams) (parent.PrivacyConsent, error) {
	consentedAt := time.Now().UTC()
	if _, err := r.queries.RecordParentPrivacyConsent(ctx, sqlc.RecordParentPrivacyConsentParams{OrganizationID: orgID, ParentAccountID: parentID, PolicyVersion: strings.TrimSpace(params.PolicyVersion), ConsentedAt: consentedAt}); err != nil {
		return parent.PrivacyConsent{}, translateError(err)
	}
	return r.GetLatestPrivacyConsent(ctx, orgID, parentID)
}

func (r *Repository) ListAccountsForStudent(ctx context.Context, orgID, studentID uint64) ([]parent.Account, error) {
	items, err := r.queries.ListParentAccountsForStudent(ctx, sqlc.ListParentAccountsForStudentParams{OrganizationID: orgID, StudentID: studentID})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]parent.Account, 0, len(items))
	for _, item := range items {
		out = append(out, mapAccount(item))
	}
	return out, nil
}

func (r *Repository) ListMessageSubscriptions(ctx context.Context, orgID, parentID uint64) ([]parent.MessageSubscription, error) {
	items, err := r.queries.ListParentMessageSubscriptions(ctx, sqlc.ListParentMessageSubscriptionsParams{OrganizationID: orgID, ParentAccountID: parentID})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]parent.MessageSubscription, 0, len(items))
	for _, item := range items {
		out = append(out, parent.MessageSubscription{ParentAccountID: item.ParentAccountID, Kind: item.MessageKind, Status: item.Status, TemplateVersion: item.TemplateVersion, AuthorizedAt: timePtr(item.AuthorizedAt), UpdatedAt: item.UpdatedAt})
	}
	return out, nil
}

func (r *Repository) UpdateMessageSubscriptions(ctx context.Context, orgID, parentID uint64, params []parent.UpdateMessageSubscriptionParams) error {
	for _, param := range params {
		if _, err := r.queries.UpsertParentMessageSubscription(ctx, sqlc.UpsertParentMessageSubscriptionParams{OrganizationID: orgID, ParentAccountID: parentID, MessageKind: strings.TrimSpace(param.Kind), Status: strings.TrimSpace(param.Status), TemplateVersion: strings.TrimSpace(param.TemplateVersion)}); err != nil {
			return translateError(err)
		}
	}
	return nil
}

func (r *Repository) CreateBinding(ctx context.Context, orgID uint64, params parent.BindStudentParams) (parent.Binding, error) {
	result, err := r.queries.CreateParentStudentBinding(ctx, sqlc.CreateParentStudentBindingParams{OrganizationID: orgID, ParentAccountID: params.ParentAccountID, StudentID: params.StudentID, Relationship: strings.TrimSpace(params.Relationship), IsPrimary: params.IsPrimary})
	if err != nil {
		return parent.Binding{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return parent.Binding{}, err
	}
	items, err := r.queries.ListParentStudentBindings(ctx, sqlc.ListParentStudentBindingsParams{ParentAccountID: params.ParentAccountID, OrganizationID: orgID})
	if err != nil {
		return parent.Binding{}, translateError(err)
	}
	for _, item := range items {
		if item.BindingID == id {
			return mapBinding(item), nil
		}
	}
	return parent.Binding{}, parent.ErrNotFound
}

func (r *Repository) ListBindings(ctx context.Context, orgID, parentID uint64) ([]parent.Binding, error) {
	items, err := r.queries.ListParentStudentBindings(ctx, sqlc.ListParentStudentBindingsParams{ParentAccountID: parentID, OrganizationID: orgID})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]parent.Binding, 0, len(items))
	for _, item := range items {
		out = append(out, mapBinding(item))
	}
	return out, nil
}

func (r *Repository) CreateChildApplication(ctx context.Context, orgID uint64, params parent.CreateChildApplicationParams) (parent.ChildApplication, error) {
	result, err := r.queries.CreateParentChildApplication(ctx, sqlc.CreateParentChildApplicationParams{
		OrganizationID:  orgID,
		ParentAccountID: params.ParentAccountID,
		StudentName:     strings.TrimSpace(params.StudentName),
		SchoolNameInput: strings.TrimSpace(params.SchoolNameInput),
		GradeInput:      strings.TrimSpace(params.GradeInput),
		ClassNameInput:  strings.TrimSpace(params.ClassNameInput),
		SchoolID:        nullableIDPtr(params.SchoolID),
		SchoolClassID:   nullableIDPtr(params.SchoolClassID),
		Grade:           strings.TrimSpace(params.Grade),
		ClassName:       strings.TrimSpace(params.ClassName),
		GuardianName:    strings.TrimSpace(params.GuardianName),
		GuardianPhone:   strings.TrimSpace(params.GuardianPhone),
		Relationship:    strings.TrimSpace(params.Relationship),
		Notes:           strings.TrimSpace(params.Notes),
	})
	if err != nil {
		return parent.ChildApplication{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return parent.ChildApplication{}, err
	}
	return r.GetChildApplication(ctx, orgID, id)
}

func (r *Repository) UpdateChildApplication(ctx context.Context, orgID uint64, params parent.UpdateChildApplicationParams) (parent.ChildApplication, error) {
	result, err := r.queries.UpdateParentChildApplication(ctx, sqlc.UpdateParentChildApplicationParams{
		StudentName:     strings.TrimSpace(params.StudentName),
		SchoolNameInput: strings.TrimSpace(params.SchoolNameInput),
		GradeInput:      strings.TrimSpace(params.GradeInput),
		ClassNameInput:  strings.TrimSpace(params.ClassNameInput),
		SchoolID:        nullableIDPtr(params.SchoolID),
		SchoolClassID:   nullableIDPtr(params.SchoolClassID),
		Grade:           strings.TrimSpace(params.Grade),
		ClassName:       strings.TrimSpace(params.ClassName),
		GuardianName:    strings.TrimSpace(params.GuardianName),
		GuardianPhone:   strings.TrimSpace(params.GuardianPhone),
		Relationship:    strings.TrimSpace(params.Relationship),
		Notes:           strings.TrimSpace(params.Notes),
		ID:              params.ID,
		OrganizationID:  orgID,
		ParentAccountID: params.ParentAccountID,
	})
	if err != nil {
		return parent.ChildApplication{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return parent.ChildApplication{}, err
	}
	return r.GetChildApplication(ctx, orgID, params.ID)
}

func (r *Repository) GetChildApplication(ctx context.Context, orgID, id uint64) (parent.ChildApplication, error) {
	item, err := r.queries.GetParentChildApplication(ctx, sqlc.GetParentChildApplicationParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return parent.ChildApplication{}, translateError(err)
	}
	return mapGetChildApplication(item), nil
}

func (r *Repository) ListChildApplications(ctx context.Context, orgID uint64, parentID *uint64) ([]parent.ChildApplication, error) {
	if parentID != nil {
		items, err := r.queries.ListParentChildApplications(ctx, sqlc.ListParentChildApplicationsParams{OrganizationID: orgID, ParentAccountID: *parentID})
		if err != nil {
			return nil, translateError(err)
		}
		out := make([]parent.ChildApplication, 0, len(items))
		for _, item := range items {
			out = append(out, mapListChildApplication(item))
		}
		return out, nil
	}
	items, err := r.queries.ListAllParentChildApplications(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]parent.ChildApplication, 0, len(items))
	for _, item := range items {
		out = append(out, mapAllChildApplication(item))
	}
	return out, nil
}

func (r *Repository) ReviewChildApplication(ctx context.Context, orgID uint64, params parent.ReviewChildApplicationParams) (parent.ChildApplication, error) {
	if params.Status != parent.ChildApplicationStatusApproved && params.Status != parent.ChildApplicationStatusRejected && params.Status != parent.ChildApplicationStatusNeedsInfo {
		return parent.ChildApplication{}, parent.ErrInvalidStatus
	}
	result, err := r.queries.ReviewParentChildApplication(ctx, sqlc.ReviewParentChildApplicationParams{
		Status:           params.Status,
		StudentID:        nullableIDPtr(params.StudentID),
		SchoolID:         nullableIDPtr(params.SchoolID),
		SchoolClassID:    nullableIDPtr(params.SchoolClassID),
		ReviewNote:       strings.TrimSpace(params.ReviewNote),
		ReviewedByUserID: nullableID(params.ReviewedByUserID),
		ReviewedAt:       sql.NullTime{Time: time.Now().UTC(), Valid: true},
		ID:               params.ID,
		OrganizationID:   orgID,
	})
	if err != nil {
		return parent.ChildApplication{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return parent.ChildApplication{}, err
	}
	return r.GetChildApplication(ctx, orgID, params.ID)
}

func (r *Repository) CreateLeaveRequest(ctx context.Context, orgID uint64, params parent.CreateLeaveRequestParams) (parent.LeaveRequest, error) {
	parentID := sql.NullInt64{Int64: int64(params.ParentAccountID), Valid: true}
	result, err := r.queries.CreateLeaveRequest(ctx, sqlc.CreateLeaveRequestParams{OrganizationID: orgID, StudentID: params.StudentID, ParentAccountID: parentID, SubmittedByType: parent.LeaveSubmittedByParent, SubmittedByUserID: sql.NullInt64{}, LeaveDate: params.LeaveDate, Reason: strings.TrimSpace(params.Reason)})
	if err != nil {
		return parent.LeaveRequest{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return parent.LeaveRequest{}, err
	}
	return r.findLeaveRequest(ctx, orgID, id)
}

func (r *Repository) CreateTeacherLeaveRequest(ctx context.Context, orgID uint64, params parent.CreateTeacherLeaveRequestParams) (parent.LeaveRequest, error) {
	existing, err := r.queries.FindActiveTeacherLeaveRequest(ctx, sqlc.FindActiveTeacherLeaveRequestParams{OrganizationID: orgID, StudentID: params.StudentID, LeaveDate: params.LeaveDate})
	if err == nil {
		return parent.LeaveRequest{}, fmt.Errorf("%w: leave request %d", parent.ErrConflict, existing.ID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return parent.LeaveRequest{}, translateError(err)
	}
	result, err := r.queries.CreateTeacherLeaveRequest(ctx, sqlc.CreateTeacherLeaveRequestParams{OrganizationID: orgID, StudentID: params.StudentID, SubmittedByUserID: sql.NullInt64{Int64: int64(params.SubmittedByUserID), Valid: true}, LeaveDate: params.LeaveDate, Reason: strings.TrimSpace(params.Reason), ReviewedByUserID: sql.NullInt64{Int64: int64(params.SubmittedByUserID), Valid: true}})
	if err != nil {
		return parent.LeaveRequest{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return parent.LeaveRequest{}, err
	}
	return r.findLeaveRequest(ctx, orgID, id)
}

func (r *Repository) UpdateLeaveRequest(ctx context.Context, orgID uint64, params parent.UpdateLeaveRequestParams) (parent.LeaveRequest, error) {
	result, err := r.queries.UpdateParentLeaveRequest(ctx, sqlc.UpdateParentLeaveRequestParams{LeaveDate: params.LeaveDate, Reason: strings.TrimSpace(params.Reason), ID: params.ID, OrganizationID: orgID, ParentAccountID: sql.NullInt64{Int64: int64(params.ParentAccountID), Valid: true}})
	if err != nil {
		return parent.LeaveRequest{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return parent.LeaveRequest{}, err
	}
	return r.findLeaveRequest(ctx, orgID, params.ID)
}

func (r *Repository) CancelLeaveRequest(ctx context.Context, orgID uint64, params parent.CancelLeaveRequestParams) (parent.LeaveRequest, error) {
	result, err := r.queries.CancelParentLeaveRequest(ctx, sqlc.CancelParentLeaveRequestParams{ID: params.ID, OrganizationID: orgID, ParentAccountID: sql.NullInt64{Int64: int64(params.ParentAccountID), Valid: true}})
	if err != nil {
		return parent.LeaveRequest{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return parent.LeaveRequest{}, err
	}
	return r.findLeaveRequest(ctx, orgID, params.ID)
}

func (r *Repository) ListLeaveRequests(ctx context.Context, orgID uint64, parentID *uint64) ([]parent.LeaveRequest, error) {
	var out []parent.LeaveRequest
	if parentID != nil {
		items, err := r.queries.ListParentLeaveRequests(ctx, sqlc.ListParentLeaveRequestsParams{OrganizationID: orgID, ParentAccountID: sql.NullInt64{Int64: int64(*parentID), Valid: true}})
		if err != nil {
			return nil, translateError(err)
		}
		out = make([]parent.LeaveRequest, 0, len(items))
		for _, item := range items {
			out = append(out, mapLeaveRequest(item))
		}
		return out, nil
	}
	items, err := r.queries.ListAllLeaveRequests(ctx, orgID)
	if err != nil {
		return nil, translateError(err)
	}
	out = make([]parent.LeaveRequest, 0, len(items))
	for _, item := range items {
		out = append(out, mapLeaveRequest(item))
	}
	return out, nil
}

func (r *Repository) ListApprovedLeaveStudentIDs(ctx context.Context, orgID uint64, leaveDate time.Time) (map[uint64]struct{}, error) {
	items, err := r.queries.ListApprovedLeaveStudentIDs(ctx, sqlc.ListApprovedLeaveStudentIDsParams{OrganizationID: orgID, LeaveDate: leaveDate})
	if err != nil {
		return nil, translateError(err)
	}
	out := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		out[item] = struct{}{}
	}
	return out, nil
}

func (r *Repository) ReviewLeaveRequest(ctx context.Context, orgID uint64, params parent.ReviewLeaveRequestParams) (parent.LeaveRequest, error) {
	if params.Status != parent.LeaveStatusApproved && params.Status != parent.LeaveStatusRejected {
		return parent.LeaveRequest{}, parent.ErrInvalidStatus
	}
	reviewedAt := sql.NullTime{Time: time.Now().UTC(), Valid: true}
	result, err := r.queries.ReviewLeaveRequest(ctx, sqlc.ReviewLeaveRequestParams{Status: params.Status, TeacherNote: strings.TrimSpace(params.TeacherNote), ReviewedByUserID: nullableID(params.ReviewedByUserID), ReviewedAt: reviewedAt, ID: params.ID, OrganizationID: orgID})
	if err != nil {
		return parent.LeaveRequest{}, translateError(err)
	}
	if err := ensureAffected(result); err != nil {
		return parent.LeaveRequest{}, err
	}
	return r.findLeaveRequest(ctx, orgID, params.ID)
}

func (r *Repository) findLeaveRequest(ctx context.Context, orgID, id uint64) (parent.LeaveRequest, error) {
	item, err := r.queries.GetLeaveRequestByID(ctx, sqlc.GetLeaveRequestByIDParams{ID: id, OrganizationID: orgID})
	if err != nil {
		return parent.LeaveRequest{}, translateError(err)
	}
	return mapLeaveRequest(item), nil
}

func mapAccount(item sqlc.ParentAccount) parent.Account {
	return parent.Account{ID: item.ID, OrganizationID: item.OrganizationID, OpenID: item.Openid, Nickname: item.Nickname, Avatar: item.Avatar, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapPrivacyConsent(item sqlc.ParentPrivacyConsent) parent.PrivacyConsent {
	return parent.PrivacyConsent{ID: item.ID, OrganizationID: item.OrganizationID, ParentAccountID: item.ParentAccountID, PolicyVersion: item.PolicyVersion, ConsentedAt: item.ConsentedAt, CreatedAt: item.CreatedAt}
}
func mapBinding(item sqlc.ListParentStudentBindingsRow) parent.Binding {
	return parent.Binding{ID: item.BindingID, OrganizationID: item.OrganizationID, ParentAccountID: item.ParentAccountID, StudentID: item.StudentID, StudentName: item.StudentName, SchoolClassID: item.SchoolClassID, CareClassID: idPtr(item.CareClassID), Relationship: item.Relationship, IsPrimary: item.IsPrimary, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

type childApplicationParts struct {
	id               uint64
	organizationID   uint64
	parentAccountID  uint64
	studentID        sql.NullInt64
	studentName      string
	schoolNameInput  string
	gradeInput       string
	classNameInput   string
	schoolID         sql.NullInt64
	schoolClassID    sql.NullInt64
	grade            string
	className        string
	guardianName     string
	guardianPhone    string
	relationship     string
	notes            string
	status           string
	reviewNote       string
	reviewedByUserID sql.NullInt64
	reviewedAt       sql.NullTime
	createdAt        time.Time
	updatedAt        time.Time
}

func mapChildApplication(parts childApplicationParts) parent.ChildApplication {
	return parent.ChildApplication{
		ID:               parts.id,
		OrganizationID:   parts.organizationID,
		ParentAccountID:  parts.parentAccountID,
		StudentID:        idPtr(parts.studentID),
		StudentName:      parts.studentName,
		SchoolNameInput:  parts.schoolNameInput,
		GradeInput:       parts.gradeInput,
		ClassNameInput:   parts.classNameInput,
		SchoolID:         idPtr(parts.schoolID),
		SchoolClassID:    idPtr(parts.schoolClassID),
		Grade:            parts.grade,
		ClassName:        parts.className,
		GuardianName:     parts.guardianName,
		GuardianPhone:    parts.guardianPhone,
		Relationship:     parts.relationship,
		Notes:            parts.notes,
		Status:           parts.status,
		ReviewNote:       parts.reviewNote,
		ReviewedByUserID: idPtr(parts.reviewedByUserID),
		ReviewedAt:       timePtr(parts.reviewedAt),
		CreatedAt:        parts.createdAt,
		UpdatedAt:        parts.updatedAt,
	}
}

func mapGetChildApplication(item sqlc.GetParentChildApplicationRow) parent.ChildApplication {
	return mapChildApplication(childApplicationParts{
		id: item.ID, organizationID: item.OrganizationID, parentAccountID: item.ParentAccountID,
		studentID: item.StudentID, studentName: item.StudentName, schoolNameInput: item.SchoolNameInput,
		gradeInput: item.GradeInput, classNameInput: item.ClassNameInput, schoolID: item.SchoolID,
		schoolClassID: item.SchoolClassID, grade: item.Grade, className: item.ClassName,
		guardianName: item.GuardianName, guardianPhone: item.GuardianPhone, relationship: item.Relationship,
		notes: item.Notes, status: item.Status, reviewNote: item.ReviewNote,
		reviewedByUserID: item.ReviewedByUserID, reviewedAt: item.ReviewedAt, createdAt: item.CreatedAt, updatedAt: item.UpdatedAt,
	})
}

func mapListChildApplication(item sqlc.ListParentChildApplicationsRow) parent.ChildApplication {
	return mapChildApplication(childApplicationParts{
		id: item.ID, organizationID: item.OrganizationID, parentAccountID: item.ParentAccountID,
		studentID: item.StudentID, studentName: item.StudentName, schoolNameInput: item.SchoolNameInput,
		gradeInput: item.GradeInput, classNameInput: item.ClassNameInput, schoolID: item.SchoolID,
		schoolClassID: item.SchoolClassID, grade: item.Grade, className: item.ClassName,
		guardianName: item.GuardianName, guardianPhone: item.GuardianPhone, relationship: item.Relationship,
		notes: item.Notes, status: item.Status, reviewNote: item.ReviewNote,
		reviewedByUserID: item.ReviewedByUserID, reviewedAt: item.ReviewedAt, createdAt: item.CreatedAt, updatedAt: item.UpdatedAt,
	})
}

func mapAllChildApplication(item sqlc.ListAllParentChildApplicationsRow) parent.ChildApplication {
	return mapChildApplication(childApplicationParts{
		id: item.ID, organizationID: item.OrganizationID, parentAccountID: item.ParentAccountID,
		studentID: item.StudentID, studentName: item.StudentName, schoolNameInput: item.SchoolNameInput,
		gradeInput: item.GradeInput, classNameInput: item.ClassNameInput, schoolID: item.SchoolID,
		schoolClassID: item.SchoolClassID, grade: item.Grade, className: item.ClassName,
		guardianName: item.GuardianName, guardianPhone: item.GuardianPhone, relationship: item.Relationship,
		notes: item.Notes, status: item.Status, reviewNote: item.ReviewNote,
		reviewedByUserID: item.ReviewedByUserID, reviewedAt: item.ReviewedAt, createdAt: item.CreatedAt, updatedAt: item.UpdatedAt,
	})
}
func mapLeaveRequest(item sqlc.LeaveRequest) parent.LeaveRequest {
	return parent.LeaveRequest{ID: item.ID, OrganizationID: item.OrganizationID, StudentID: item.StudentID, ParentAccountID: idPtr(item.ParentAccountID), SubmittedByType: item.SubmittedByType, SubmittedByUserID: idPtr(item.SubmittedByUserID), LeaveDate: item.LeaveDate, Reason: item.Reason, Status: item.Status, TeacherNote: item.TeacherNote, ReviewedByUserID: idPtr(item.ReviewedByUserID), ReviewedAt: timePtr(item.ReviewedAt), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
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
func nullableID(value uint64) sql.NullInt64 {
	if value == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}

func nullableIDPtr(value *uint64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return nullableID(*value)
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
		return fmt.Errorf("read affected rows: %w", err)
	}
	if affected == 0 {
		return parent.ErrInvalidState
	}
	return nil
}

func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return parent.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return parent.ErrConflict
	}
	if strings.Contains(strings.ToLower(err.Error()), "foreign key constraint fails") {
		return fmt.Errorf("%w: invalid relation", parent.ErrNotFound)
	}
	return err
}
