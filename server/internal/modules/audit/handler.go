package audit

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	store Store
	orgID uint64
}

func NewHandler(store Store, organizationID uint64) *Handler {
	return &Handler{store: store, orgID: organizationID}
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/audit-logs", h.list)
}

type entryView struct {
	ID           uint64  `json:"id"`
	ActorType    string  `json:"actor_type"`
	ActorID      *uint64 `json:"actor_id,omitempty"`
	Action       string  `json:"action"`
	ResourceType string  `json:"resource_type"`
	ResourceID   *uint64 `json:"resource_id,omitempty"`
	Metadata     any     `json:"metadata"`
	RequestID    string  `json:"request_id,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

func (h *Handler) list(c *gin.Context) {
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindUser || (p.Role != identity.UserRoleAdmin && p.Role != identity.UserRoleEditor) {
		response.Error(c, response.Forbidden())
		return
	}
	limit := 100
	if value := strings.TrimSpace(c.Query("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 200 {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "limit", Reason: "invalid_value"}}))
			return
		}
		limit = parsed
	}
	items, err := h.store.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), ListFilter{Action: strings.TrimSpace(c.Query("action")), ResourceType: strings.TrimSpace(c.Query("resource_type")), Limit: limit})
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	out := make([]entryView, 0, len(items))
	for _, item := range items {
		var metadata any = map[string]any{}
		if strings.TrimSpace(item.MetadataJSON) != "" {
			if err := json.Unmarshal([]byte(item.MetadataJSON), &metadata); err != nil {
				metadata = item.MetadataJSON
			}
		}
		out = append(out, entryView{ID: item.ID, ActorType: item.ActorType, ActorID: item.ActorID, Action: item.Action, ResourceType: item.ResourceType, ResourceID: item.ResourceID, Metadata: metadata, RequestID: item.RequestID, CreatedAt: item.CreatedAt})
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func principalActor(ctx context.Context) (string, *uint64) {
	p, ok := identity.PrincipalFromContext(ctx)
	if !ok {
		return "anonymous", nil
	}
	id := p.SubjectID
	if p.Kind == identity.PrincipalKindParent {
		return "parent", &id
	}
	if p.Kind == identity.PrincipalKindUser {
		return "staff", &id
	}
	return "anonymous", nil
}

func RecordForContext(ctx context.Context, writer Writer, organizationID uint64, action, resourceType string, resourceID *uint64, metadataJSON, requestID string) {
	if writer == nil {
		return
	}
	actorType, actorID := principalActor(ctx)
	_ = writer.Record(ctx, RecordParams{OrganizationID: organizationID, ActorType: actorType, ActorID: actorID, Action: action, ResourceType: resourceType, ResourceID: resourceID, MetadataJSON: metadataJSON, RequestID: requestID})
}
