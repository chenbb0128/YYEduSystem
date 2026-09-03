package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/platformadmin"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database/sqlc"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct {
	exec    database.DBTX
	queries *sqlc.Queries
}

func New(exec database.DBTX) *Repository { return &Repository{exec: exec, queries: sqlc.New(exec)} }

func (r *Repository) ListOrganizations(ctx context.Context) ([]platformadmin.Organization, error) {
	items, err := r.queries.ListOrganizations(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]platformadmin.Organization, 0, len(items))
	for _, item := range items {
		out = append(out, mapOrganizationRow(item))
	}
	return out, nil
}

func (r *Repository) CreateOrganization(ctx context.Context, params platformadmin.CreateOrganizationParams) (platformadmin.Organization, error) {
	result, err := r.queries.CreateOrganization(ctx, sqlc.CreateOrganizationParams{Name: strings.TrimSpace(params.Name), Slug: strings.TrimSpace(params.Slug), ContactName: strings.TrimSpace(params.ContactName), ContactPhone: strings.TrimSpace(params.ContactPhone), AuthorizedUntil: nullTime(params.AuthorizedUntil), Status: defaultStatus(params.Status, platformadmin.OrganizationStatusPending)})
	if err != nil {
		return platformadmin.Organization{}, translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return platformadmin.Organization{}, err
	}
	items, err := r.ListOrganizations(ctx)
	if err != nil {
		return platformadmin.Organization{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return platformadmin.Organization{}, platformadmin.ErrNotFound
}

func (r *Repository) SetOrganizationStatus(ctx context.Context, id uint64, status string) error {
	result, err := r.queries.SetOrganizationStatus(ctx, sqlc.SetOrganizationStatusParams{Status: strings.TrimSpace(status), ID: id})
	if err != nil {
		return translateError(err)
	}
	return ensureAffected(result)
}

func (r *Repository) SetOrganizationAuthorization(ctx context.Context, id uint64, authorizedUntil *time.Time) error {
	result, err := r.exec.ExecContext(ctx, "UPDATE organizations SET authorized_until = ? WHERE id = ?", nullTime(authorizedUntil), id)
	if err != nil {
		return translateError(err)
	}
	return ensureAffected(result)
}

func (r *Repository) ListInvites(ctx context.Context, status string) ([]platformadmin.Invite, error) {
	filter := sql.NullString{}
	if strings.TrimSpace(status) != "" {
		filter = sql.NullString{String: strings.TrimSpace(status), Valid: true}
	}
	items, err := r.queries.ListOrganizationInvites(ctx, sqlc.ListOrganizationInvitesParams{StatusFilter: filter})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]platformadmin.Invite, 0, len(items))
	for _, item := range items {
		out = append(out, mapInvite(item))
	}
	return out, nil
}

func (r *Repository) CreateInvite(ctx context.Context, params platformadmin.CreateInviteParams) (platformadmin.Invite, string, error) {
	if params.MaxUses == 0 {
		params.MaxUses = 1
	}
	code, err := platformadmin.NewInviteCode()
	if err != nil {
		return platformadmin.Invite{}, "", err
	}
	result, err := r.queries.CreateOrganizationInvite(ctx, sqlc.CreateOrganizationInviteParams{CodeHash: platformadmin.HashInviteCode(code), CodeHint: code[:minInt(8, len(code))], MaxUses: params.MaxUses, ExpiresAt: nullTime(params.ExpiresAt), Note: strings.TrimSpace(params.Note), CreatedByUserID: params.CreatedByID})
	if err != nil {
		return platformadmin.Invite{}, "", translateError(err)
	}
	id, err := insertedID(result)
	if err != nil {
		return platformadmin.Invite{}, "", err
	}
	items, err := r.ListInvites(ctx, "")
	if err != nil {
		return platformadmin.Invite{}, "", err
	}
	for _, item := range items {
		if item.ID == id {
			return item, code, nil
		}
	}
	return platformadmin.Invite{}, "", platformadmin.ErrNotFound
}

func (r *Repository) RevokeInvite(ctx context.Context, id uint64) error {
	result, err := r.queries.RevokeOrganizationInvite(ctx, id)
	if err != nil {
		return translateError(err)
	}
	return ensureAffected(result)
}

func (r *Repository) CreateRegistration(ctx context.Context, params platformadmin.CreateRegistrationParams) (platformadmin.Registration, error) {
	var item platformadmin.Registration
	err := r.withTransaction(ctx, func(q *sqlc.Queries) error {
		invite, err := q.GetOrganizationInviteByHash(ctx, platformadmin.HashInviteCode(params.InviteCode))
		if err != nil {
			return translateError(err)
		}
		now := time.Now().UTC()
		if invite.Status != platformadmin.InviteStatusActive || (invite.ExpiresAt.Valid && !now.Before(invite.ExpiresAt.Time)) {
			return platformadmin.ErrInvalidInvite
		}
		if invite.UsedCount >= invite.MaxUses {
			return platformadmin.ErrInviteExhausted
		}
		result, err := q.ConsumeOrganizationInvite(ctx, invite.ID)
		if err != nil {
			return translateError(err)
		}
		if err := ensureAffected(result); err != nil {
			return platformadmin.ErrInviteExhausted
		}
		result, err = q.CreateOrganizationRegistration(ctx, sqlc.CreateOrganizationRegistrationParams{InviteID: invite.ID, OrganizationName: strings.TrimSpace(params.OrganizationName), Slug: strings.TrimSpace(params.Slug), ContactName: strings.TrimSpace(params.ContactName), ContactPhone: strings.TrimSpace(params.ContactPhone), AdminUsername: strings.TrimSpace(params.AdminUsername), AdminPasswordHash: params.AdminPasswordHash})
		if err != nil {
			return translateError(err)
		}
		id, err := insertedID(result)
		if err != nil {
			return err
		}
		row, err := q.GetOrganizationRegistration(ctx, id)
		if err != nil {
			return translateError(err)
		}
		item = mapRegistration(row)
		return nil
	})
	return item, err
}

func (r *Repository) GetRegistration(ctx context.Context, id uint64) (platformadmin.Registration, error) {
	item, err := r.queries.GetOrganizationRegistration(ctx, id)
	if err != nil {
		return platformadmin.Registration{}, translateError(err)
	}
	return mapRegistration(item), nil
}

func (r *Repository) ListRegistrations(ctx context.Context, status string) ([]platformadmin.Registration, error) {
	filter := sql.NullString{}
	if strings.TrimSpace(status) != "" {
		filter = sql.NullString{String: strings.TrimSpace(status), Valid: true}
	}
	items, err := r.queries.ListOrganizationRegistrations(ctx, sqlc.ListOrganizationRegistrationsParams{StatusFilter: filter})
	if err != nil {
		return nil, translateError(err)
	}
	out := make([]platformadmin.Registration, 0, len(items))
	for _, item := range items {
		out = append(out, mapRegistration(item))
	}
	return out, nil
}

func (r *Repository) SetRegistrationStatus(ctx context.Context, params platformadmin.SetRegistrationStatusParams) error {
	result, err := r.queries.SetOrganizationRegistrationStatus(ctx, sqlc.SetOrganizationRegistrationStatusParams{Status: params.Status, OrganizationID: nullID(params.OrganizationID), ReviewNote: strings.TrimSpace(params.ReviewNote), ReviewedByUserID: nullableID(params.ReviewedByID), ID: params.ID})
	if err != nil {
		return translateError(err)
	}
	return ensureAffected(result)
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

func mapOrganization(item sqlc.Organization) platformadmin.Organization {
	return platformadmin.Organization{ID: item.ID, Name: item.Name, Slug: item.Slug, ContactName: item.ContactName, ContactPhone: item.ContactPhone, AuthorizedUntil: timePtr(item.AuthorizedUntil), Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapOrganizationRow(item sqlc.ListOrganizationsRow) platformadmin.Organization {
	return platformadmin.Organization{ID: item.ID, Name: item.Name, Slug: item.Slug, ContactName: item.ContactName, ContactPhone: item.ContactPhone, AuthorizedUntil: timePtr(item.AuthorizedUntil), Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapInvite(item sqlc.OrganizationInvite) platformadmin.Invite {
	return platformadmin.Invite{ID: item.ID, CodeHint: item.CodeHint, MaxUses: item.MaxUses, UsedCount: item.UsedCount, ExpiresAt: timePtr(item.ExpiresAt), Status: item.Status, Note: item.Note, CreatedByID: item.CreatedByUserID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func mapRegistration(item sqlc.OrganizationRegistration) platformadmin.Registration {
	return platformadmin.Registration{ID: item.ID, InviteID: item.InviteID, OrganizationID: idPtr(item.OrganizationID), OrganizationName: item.OrganizationName, Slug: item.Slug, ContactName: item.ContactName, ContactPhone: item.ContactPhone, AdminUsername: item.AdminUsername, AdminPasswordHash: item.AdminPasswordHash, Status: item.Status, ReviewNote: item.ReviewNote, ReviewedByID: idPtr(item.ReviewedByUserID), ReviewedAt: timePtr(item.ReviewedAt), CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
func insertedID(result sql.Result) (uint64, error) {
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		if err == nil {
			err = fmt.Errorf("invalid insert id %d", id)
		}
		return 0, err
	}
	return uint64(id), nil
}
func ensureAffected(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return platformadmin.ErrNotFound
	}
	return nil
}
func nullTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}
func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
func nullID(value *uint64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}
func nullableID(value uint64) sql.NullInt64 {
	if value == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}
func idPtr(value sql.NullInt64) *uint64 {
	if !value.Valid || value.Int64 <= 0 {
		return nil
	}
	result := uint64(value.Int64)
	return &result
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func defaultStatus(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return platformadmin.ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber {
		return platformadmin.ErrConflict
	}
	return err
}
