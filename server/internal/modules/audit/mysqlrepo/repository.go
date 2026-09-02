package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/audit"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/database"
)

type Repository struct{ exec database.DBTX }

func New(exec database.DBTX) *Repository { return &Repository{exec: exec} }

func (r *Repository) Record(ctx context.Context, params audit.RecordParams) error {
	metadata := strings.TrimSpace(params.MetadataJSON)
	if metadata == "" {
		metadata = "{}"
	}
	if !json.Valid([]byte(metadata)) {
		return errors.New("audit metadata must be valid JSON")
	}
	_, err := r.exec.ExecContext(ctx, `INSERT INTO audit_logs (organization_id, actor_type, actor_id, action, resource_type, resource_id, metadata_json, request_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, params.OrganizationID, actorType(params.ActorType), nullableID(params.ActorID), strings.TrimSpace(params.Action), strings.TrimSpace(params.ResourceType), nullableID(params.ResourceID), metadata, strings.TrimSpace(params.RequestID))
	return err
}

func (r *Repository) List(ctx context.Context, orgID uint64, filter audit.ListFilter) ([]audit.Entry, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `SELECT id, organization_id, actor_type, actor_id, action, resource_type, resource_id, metadata_json, request_id, created_at FROM audit_logs WHERE organization_id = ?`
	args := []any{orgID}
	if strings.TrimSpace(filter.Action) != "" {
		query += " AND action = ?"
		args = append(args, strings.TrimSpace(filter.Action))
	}
	if strings.TrimSpace(filter.ResourceType) != "" {
		query += " AND resource_type = ?"
		args = append(args, strings.TrimSpace(filter.ResourceType))
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := r.exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]audit.Entry, 0, limit)
	for rows.Next() {
		var item audit.Entry
		var actorID, resourceID sql.NullInt64
		var createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.ActorType, &actorID, &item.Action, &item.ResourceType, &resourceID, &item.MetadataJSON, &item.RequestID, &createdAt); err != nil {
			return nil, err
		}
		item.ActorID = idPtr(actorID)
		item.ResourceID = idPtr(resourceID)
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.UTC().Format("2006-01-02 15:04:05")
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func actorType(value string) string {
	switch strings.TrimSpace(value) {
	case "staff", "parent", "system", "anonymous":
		return strings.TrimSpace(value)
	default:
		return "anonymous"
	}
}

func nullableID(value *uint64) any {
	if value == nil {
		return nil
	}
	return *value
}

func idPtr(value sql.NullInt64) *uint64 {
	if !value.Valid || value.Int64 <= 0 {
		return nil
	}
	x := uint64(value.Int64)
	return &x
}
