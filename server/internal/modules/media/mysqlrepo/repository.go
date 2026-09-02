package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/media"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
)

const duplicateEntryErrorNumber uint16 = 1062

type Repository struct{ exec database.DBTX }

func New(exec database.DBTX) *Repository { return &Repository{exec: exec} }

func (r *Repository) Create(ctx context.Context, orgID uint64, params media.CreateParams) (media.Asset, error) {
	result, err := r.exec.ExecContext(ctx, `INSERT INTO media_assets (organization_id, object_key, resource_type, resource_id, owner_type, owner_id, content_type, size_bytes, sha256_hex, status, retention_until, created_by_user_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`, orgID, strings.TrimSpace(params.ObjectKey), strings.TrimSpace(params.ResourceType), nullableID(params.ResourceID), strings.TrimSpace(params.OwnerType), nullableID(params.OwnerID), strings.TrimSpace(params.ContentType), params.SizeBytes, strings.TrimSpace(params.SHA256), nullableTime(params.RetentionUntil), nullableID(params.CreatedByUserID))
	if err != nil {
		if isDuplicate(err) {
			return media.Asset{}, media.ErrConflict
		}
		return media.Asset{}, err
	}
	id, err := result.LastInsertId()
	if err != nil || id <= 0 {
		return media.Asset{}, fmt.Errorf("media: read inserted id: %w", err)
	}
	return r.findByID(ctx, orgID, uint64(id))
}

func (r *Repository) FindByKey(ctx context.Context, orgID uint64, key string) (media.Asset, error) {
	row := r.exec.QueryRowContext(ctx, `SELECT id, organization_id, object_key, resource_type, resource_id, owner_type, owner_id, content_type, size_bytes, sha256_hex, status, retention_until, created_by_user_id, created_at, deleted_at FROM media_assets WHERE organization_id = ? AND object_key = ? AND status = 'active' LIMIT 1`, orgID, strings.TrimSpace(key))
	return scan(row)
}

func (r *Repository) List(ctx context.Context, orgID uint64, resourceType string, limit int) ([]media.Asset, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, organization_id, object_key, resource_type, resource_id, owner_type, owner_id, content_type, size_bytes, sha256_hex, status, retention_until, created_by_user_id, created_at, deleted_at FROM media_assets WHERE organization_id = ? AND status = 'active'`
	args := []any{orgID}
	if value := strings.TrimSpace(resourceType); value != "" {
		query += ` AND resource_type = ?`
		args = append(args, value)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]media.Asset, 0, limit)
	for rows.Next() {
		item, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type rowScanner interface{ Scan(...any) error }

func (r *Repository) findByID(ctx context.Context, orgID, id uint64) (media.Asset, error) {
	row := r.exec.QueryRowContext(ctx, `SELECT id, organization_id, object_key, resource_type, resource_id, owner_type, owner_id, content_type, size_bytes, sha256_hex, status, retention_until, created_by_user_id, created_at, deleted_at FROM media_assets WHERE organization_id = ? AND id = ? LIMIT 1`, orgID, id)
	return scan(row)
}

func scan(row rowScanner) (media.Asset, error) {
	return scanRows(row)
}

func scanRows(row rowScanner) (media.Asset, error) {
	var item media.Asset
	var resourceID, ownerID, createdBy sql.NullInt64
	var retentionUntil, createdAt, deletedAt sql.NullTime
	if err := row.Scan(&item.ID, &item.OrganizationID, &item.ObjectKey, &item.ResourceType, &resourceID, &item.OwnerType, &ownerID, &item.ContentType, &item.SizeBytes, &item.SHA256, &item.Status, &retentionUntil, &createdBy, &createdAt, &deletedAt); err != nil {
		if err == sql.ErrNoRows {
			return media.Asset{}, media.ErrNotFound
		}
		return media.Asset{}, err
	}
	item.ResourceID = idPtr(resourceID)
	item.OwnerID = idPtr(ownerID)
	item.CreatedByUserID = idPtr(createdBy)
	if retentionUntil.Valid {
		value := retentionUntil.Time.UTC()
		item.RetentionUntil = &value
	}
	if createdAt.Valid {
		item.CreatedAt = createdAt.Time.UTC()
	}
	if deletedAt.Valid {
		value := deletedAt.Time.UTC()
		item.DeletedAt = &value
	}
	return item, nil
}

func nullableID(value *uint64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func idPtr(value sql.NullInt64) *uint64 {
	if !value.Valid || value.Int64 <= 0 {
		return nil
	}
	result := uint64(value.Int64)
	return &result
}

func isDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == duplicateEntryErrorNumber
}
