package platformadmin

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	store Store
	users identity.UserStore
}

func NewHandler(store Store, users identity.UserStore) *Handler {
	return &Handler{store: store, users: users}
}

func (h *Handler) RegisterPublicRoutes(api *gin.RouterGroup) {
	api.POST("/auth/organization-register", h.registerOrganization)
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/platform/organizations", h.listOrganizations)
	api.POST("/platform/organizations/:id/status", h.setOrganizationStatus)
	api.GET("/platform/invites", h.listInvites)
	api.POST("/platform/invites", h.createInvite)
	api.POST("/platform/invites/:id/revoke", h.revokeInvite)
	api.GET("/platform/registrations", h.listRegistrations)
	api.POST("/platform/registrations/:id/review", h.reviewRegistration)
}

type registerOrganizationRequest struct {
	InviteCode       string `json:"inviteCode"`
	OrganizationName string `json:"organizationName"`
	Slug             string `json:"slug"`
	ContactName      string `json:"contactName"`
	ContactPhone     string `json:"contactPhone"`
	AdminUsername    string `json:"adminUsername"`
	AdminPassword    string `json:"adminPassword"`
}

func (r registerOrganizationRequest) Validate() []response.ValidationDetail {
	var details []response.ValidationDetail
	if len(strings.TrimSpace(r.InviteCode)) < 8 {
		details = append(details, response.ValidationDetail{Field: "inviteCode", Reason: "required"})
	}
	if len([]rune(strings.TrimSpace(r.OrganizationName))) < 2 {
		details = append(details, response.ValidationDetail{Field: "organizationName", Reason: "required"})
	}
	if len(strings.TrimSpace(r.AdminUsername)) < 3 || len(strings.TrimSpace(r.AdminUsername)) > 64 {
		details = append(details, response.ValidationDetail{Field: "adminUsername", Reason: "length"})
	}
	if len(r.AdminPassword) < 6 {
		details = append(details, response.ValidationDetail{Field: "adminPassword", Reason: "min_length_6"})
	}
	return details
}

func (h *Handler) registerOrganization(c *gin.Context) {
	var req registerOrganizationRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if h.users != nil {
		if _, err := h.users.FindUserByUsername(c.Request.Context(), strings.TrimSpace(req.AdminUsername)); err == nil {
			response.Error(c, response.BadRequest("管理员账号已存在，请换一个账号", identity.ErrUsernameTaken))
			return
		} else if !errors.Is(err, identity.ErrUserNotFound) {
			response.Error(c, response.Internal(err))
			return
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	item, err := h.store.CreateRegistration(c.Request.Context(), CreateRegistrationParams{InviteCode: req.InviteCode, OrganizationName: req.OrganizationName, Slug: normalizeSlug(req.Slug, req.OrganizationName), ContactName: req.ContactName, ContactPhone: req.ContactPhone, AdminUsername: req.AdminUsername, AdminPasswordHash: string(hash)})
	if err != nil {
		h.respondError(c, err)
		return
	}
	response.Created(c, "/api/v1/auth/organization-register/"+strconv.FormatUint(item.ID, 10), gin.H{"id": item.ID, "status": item.Status, "message": "注册申请已提交，请等待平台管理员审核"})
}

type organizationView struct {
	ID              uint64  `json:"id"`
	Name            string  `json:"name"`
	Slug            string  `json:"slug"`
	ContactName     string  `json:"contactName"`
	ContactPhone    string  `json:"contactPhone"`
	AuthorizedUntil *string `json:"authorizedUntil,omitempty"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"createdAt"`
}
type inviteView struct {
	ID        uint64  `json:"id"`
	CodeHint  string  `json:"codeHint"`
	MaxUses   uint32  `json:"maxUses"`
	UsedCount uint32  `json:"usedCount"`
	ExpiresAt *string `json:"expiresAt,omitempty"`
	Status    string  `json:"status"`
	Note      string  `json:"note"`
	CreatedAt string  `json:"createdAt"`
}
type registrationView struct {
	ID               uint64  `json:"id"`
	InviteID         uint64  `json:"inviteId"`
	OrganizationID   *uint64 `json:"organizationId,omitempty"`
	OrganizationName string  `json:"organizationName"`
	Slug             string  `json:"slug"`
	ContactName      string  `json:"contactName"`
	ContactPhone     string  `json:"contactPhone"`
	AdminUsername    string  `json:"adminUsername"`
	Status           string  `json:"status"`
	ReviewNote       string  `json:"reviewNote"`
	ReviewedAt       *string `json:"reviewedAt,omitempty"`
	CreatedAt        string  `json:"createdAt"`
}

func (h *Handler) listOrganizations(c *gin.Context) {
	if _, ok := platformPrincipal(c); !ok {
		return
	}
	items, err := h.store.ListOrganizations(c.Request.Context())
	if err != nil {
		h.respondError(c, err)
		return
	}
	out := make([]organizationView, 0, len(items))
	for _, item := range items {
		out = append(out, toOrganizationView(item))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

type setOrganizationStatusRequest struct {
	Status string `json:"status"`
}

func (r setOrganizationStatusRequest) Validate() []response.ValidationDetail {
	status := strings.TrimSpace(r.Status)
	if status != OrganizationStatusPending && status != OrganizationStatusActive && status != OrganizationStatusDisabled {
		return []response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}
	}
	return nil
}

func (h *Handler) setOrganizationStatus(c *gin.Context) {
	if _, ok := platformPrincipal(c); !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req setOrganizationStatusRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.store.SetOrganizationStatus(c.Request.Context(), id, strings.TrimSpace(req.Status)); err != nil {
		h.respondError(c, err)
		return
	}
	response.OK(c, true)
}

type createInviteRequest struct {
	MaxUses   uint32 `json:"maxUses"`
	ExpiresAt string `json:"expiresAt"`
	Note      string `json:"note"`
}

func (h *Handler) createInvite(c *gin.Context) {
	principal, ok := platformPrincipal(c)
	if !ok {
		return
	}
	var req createInviteRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	var expiry *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(req.ExpiresAt), time.UTC)
		if err != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "expiresAt", Reason: "date_format"}}))
			return
		}
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
		expiry = &parsed
	}
	item, plain, err := h.store.CreateInvite(c.Request.Context(), CreateInviteParams{MaxUses: req.MaxUses, ExpiresAt: expiry, Note: req.Note, CreatedByID: principal.SubjectID})
	if err != nil {
		h.respondError(c, err)
		return
	}
	response.Created(c, "/api/v1/platform/invites/"+strconv.FormatUint(item.ID, 10), gin.H{"invite": toInviteView(item), "code": plain, "warning": "邀请码只在本次显示，请立即保存"})
}

func (h *Handler) listInvites(c *gin.Context) {
	if _, ok := platformPrincipal(c); !ok {
		return
	}
	items, err := h.store.ListInvites(c.Request.Context(), strings.TrimSpace(c.Query("status")))
	if err != nil {
		h.respondError(c, err)
		return
	}
	out := make([]inviteView, 0, len(items))
	for _, item := range items {
		out = append(out, toInviteView(item))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}
func (h *Handler) revokeInvite(c *gin.Context) {
	if _, ok := platformPrincipal(c); !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.store.RevokeInvite(c.Request.Context(), id); err != nil {
		h.respondError(c, err)
		return
	}
	response.OK(c, true)
}
func (h *Handler) listRegistrations(c *gin.Context) {
	if _, ok := platformPrincipal(c); !ok {
		return
	}
	items, err := h.store.ListRegistrations(c.Request.Context(), strings.TrimSpace(c.Query("status")))
	if err != nil {
		h.respondError(c, err)
		return
	}
	out := make([]registrationView, 0, len(items))
	for _, item := range items {
		out = append(out, toRegistrationView(item))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

type reviewRegistrationRequest struct {
	Status     string `json:"status"`
	ReviewNote string `json:"reviewNote"`
}

func (h *Handler) reviewRegistration(c *gin.Context) {
	principal, ok := platformPrincipal(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req reviewRegistrationRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.store.GetRegistration(c.Request.Context(), id)
	if err != nil {
		h.respondError(c, err)
		return
	}
	status := strings.TrimSpace(req.Status)
	if item.Status != RegistrationStatusPending {
		response.Error(c, response.BadRequest("该注册申请已处理，不能重复审核", ErrInvalidStatus))
		return
	}
	if status == RegistrationStatusRejected {
		if err := h.store.SetRegistrationStatus(c.Request.Context(), SetRegistrationStatusParams{ID: id, Status: status, ReviewNote: req.ReviewNote, ReviewedByID: principal.SubjectID}); err != nil {
			h.respondError(c, err)
			return
		}
		response.OK(c, gin.H{"status": status})
		return
	}
	if status != RegistrationStatusApproved {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "status", Reason: "must_be_approved_or_rejected"}}))
		return
	}
	if h.users == nil {
		response.Error(c, response.Internal(errors.New("用户服务未配置")))
		return
	}
	if _, err := h.users.FindUserByUsername(c.Request.Context(), item.AdminUsername); err == nil {
		response.Error(c, response.BadRequest("管理员账号已存在，无法审核通过", identity.ErrUsernameTaken))
		return
	} else if !errors.Is(err, identity.ErrUserNotFound) {
		response.Error(c, response.Internal(err))
		return
	}
	organization, err := h.store.CreateOrganization(c.Request.Context(), CreateOrganizationParams{Name: item.OrganizationName, Slug: normalizeSlug(item.Slug, item.OrganizationName), ContactName: item.ContactName, ContactPhone: item.ContactPhone, Status: OrganizationStatusPending})
	if err != nil {
		h.respondError(c, err)
		return
	}
	user, err := h.users.CreateUser(c.Request.Context(), identity.CreateUserParams{OrganizationID: organization.ID, Username: item.AdminUsername, PasswordHash: item.AdminPasswordHash, Role: identity.UserRoleAdmin, Nickname: item.ContactName, Status: identity.UserStatusActive})
	if err != nil {
		_ = h.store.SetOrganizationStatus(c.Request.Context(), organization.ID, OrganizationStatusDisabled)
		h.respondError(c, err)
		return
	}
	if err := h.store.SetOrganizationStatus(c.Request.Context(), organization.ID, OrganizationStatusActive); err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	if err := h.store.SetRegistrationStatus(c.Request.Context(), SetRegistrationStatusParams{ID: id, Status: status, OrganizationID: &organization.ID, ReviewNote: req.ReviewNote, ReviewedByID: principal.SubjectID}); err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	response.OK(c, gin.H{"status": status, "organizationId": organization.ID, "adminUserId": user.ID})
}

func platformPrincipal(c *gin.Context) (identity.Principal, bool) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || !identity.IsPlatformAdmin(principal.Role) {
		response.Error(c, response.Forbidden())
		return identity.Principal{}, false
	}
	return principal, true
}
func parseID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, response.BadRequest("ID 不合法", err))
		return 0, false
	}
	return id, true
}
func normalizeSlug(slug, name string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug != "" {
		return slug
	}
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "org-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return b.String()
}
func optionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}
func toOrganizationView(item Organization) organizationView {
	return organizationView{ID: item.ID, Name: item.Name, Slug: item.Slug, ContactName: item.ContactName, ContactPhone: item.ContactPhone, AuthorizedUntil: optionalDateString(item.AuthorizedUntil), Status: item.Status, CreatedAt: item.CreatedAt.Format(time.RFC3339)}
}
func optionalDateString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("2006-01-02")
	return &formatted
}
func toInviteView(item Invite) inviteView {
	return inviteView{ID: item.ID, CodeHint: item.CodeHint, MaxUses: item.MaxUses, UsedCount: item.UsedCount, ExpiresAt: optionalTime(item.ExpiresAt), Status: item.Status, Note: item.Note, CreatedAt: item.CreatedAt.Format(time.RFC3339)}
}
func toRegistrationView(item Registration) registrationView {
	return registrationView{ID: item.ID, InviteID: item.InviteID, OrganizationID: item.OrganizationID, OrganizationName: item.OrganizationName, Slug: item.Slug, ContactName: item.ContactName, ContactPhone: item.ContactPhone, AdminUsername: item.AdminUsername, Status: item.Status, ReviewNote: item.ReviewNote, ReviewedAt: optionalTime(item.ReviewedAt), CreatedAt: item.CreatedAt.Format(time.RFC3339)}
}
func (h *Handler) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInvite):
		response.Error(c, response.BadRequest("邀请码无效或已过期", err))
	case errors.Is(err, ErrInviteExhausted):
		response.Error(c, response.BadRequest("邀请码已达到使用次数", err))
	case errors.Is(err, ErrConflict), errors.Is(err, identity.ErrUsernameTaken):
		response.Error(c, response.BadRequest("数据已存在或账号已被占用", err))
	case errors.Is(err, ErrNotFound):
		response.Error(c, response.NotFound())
	case errors.Is(err, ErrInvalidStatus):
		response.Error(c, response.BadRequest("当前状态不允许此操作", err))
	default:
		response.Error(c, response.Internal(fmt.Errorf("platform admin: %w", err)))
	}
}
