package meal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	auditmodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/audit"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	mediamodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/media"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/storage"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	store              Store
	masterData         masterdata.Store
	parents            parent.Store
	notifications      pickup.NotificationWriter
	assignments        assignment.Store
	photos             storage.Store
	photoSigner        *storage.URLSigner
	assets             mediamodule.Store
	assetRetentionDays int
	audit              auditmodule.Writer
	orgID              uint64
}

func NewHandler(store Store, masterData masterdata.Store) *Handler {
	return &Handler{store: store, masterData: masterData, orgID: masterdata.DefaultOrganizationID}
}
func (h *Handler) SetParentStore(value parent.Store)                     { h.parents = value }
func (h *Handler) SetNotificationWriter(value pickup.NotificationWriter) { h.notifications = value }
func (h *Handler) SetStaffScope(value assignment.Store)                  { h.assignments = value }
func (h *Handler) SetPhotoStore(value storage.Store)                     { h.photos = value }
func (h *Handler) SetPhotoSigner(value *storage.URLSigner)               { h.photoSigner = value }
func (h *Handler) SetAssetStore(value mediamodule.Store)                 { h.assets = value }
func (h *Handler) SetAssetRetentionDays(days int)                        { h.assetRetentionDays = days }
func (h *Handler) SetAuditWriter(value auditmodule.Writer)               { h.audit = value }

func (h *Handler) recordAudit(c *gin.Context, action, resourceType string, resourceID uint64) {
	auditmodule.RecordForContext(c.Request.Context(), h.audit, h.orgID, action, resourceType, &resourceID, "{}", c.GetHeader("X-Request-ID"))
}

func (h *Handler) RegisterStaffRoutes(api *gin.RouterGroup) {
	api.GET("/meals", h.listPlans)
	api.POST("/meals", h.upsertPlan)
	api.POST("/meals/copy", h.copyPlan)
	api.POST("/uploads/meals", h.uploadPhoto)
	api.GET("/meal-diet-notes", h.listDietNotes)
	api.GET("/diet-note-change-requests", h.listDietNoteChangeRequests)
	api.POST("/diet-note-change-requests/:id/review", h.reviewDietNoteChangeRequest)
	// Keep the wildcard name aligned with the existing /students/:id routes.
	// The parameter name is internal to Gin; the public URL is unchanged.
	api.PUT("/students/:id/diet-note", h.upsertDietNote)
}
func (h *Handler) RegisterParentRoutes(api *gin.RouterGroup) {
	api.GET("/parent/meals", h.listParentPlans)
	api.GET("/parent/students/:student_id/diet-note", h.getParentDietNote)
	api.GET("/parent/students/:student_id/diet-note-requests", h.listParentDietNoteChangeRequests)
	api.POST("/parent/students/:student_id/diet-note-requests", h.parentCreateDietNoteChangeRequest)
	// Backward-compatible path: it now creates a review request and never
	// writes the official note directly from a parent account.
	api.PUT("/parent/students/:student_id/diet-note", h.parentUpsertDietNote)
}

type planView struct {
	ID             uint64 `json:"id"`
	MealDate       string `json:"meal_date"`
	MenuText       string `json:"menu_text"`
	PhotoURL       string `json:"photo_url,omitempty"`
	AdjustmentNote string `json:"adjustment_note"`
	CreatedByName  string `json:"created_by_name"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}
type dietNoteView struct {
	ID            uint64 `json:"id"`
	StudentID     uint64 `json:"student_id"`
	Note          string `json:"note"`
	UpdatedByName string `json:"updated_by_name"`
	UpdatedAt     string `json:"updated_at"`
}
type dietNoteChangeRequestView struct {
	ID              uint64  `json:"id"`
	StudentID       uint64  `json:"student_id"`
	ParentAccountID uint64  `json:"parent_account_id,omitempty"`
	CurrentNote     string  `json:"current_note"`
	RequestedNote   string  `json:"requested_note"`
	Status          string  `json:"status"`
	ReviewNote      string  `json:"review_note"`
	ReviewedAt      *string `json:"reviewed_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}
type upsertPlanRequest struct {
	MealDate       string `json:"meal_date"`
	MenuText       string `json:"menu_text"`
	PhotoURL       string `json:"photo_url"`
	AdjustmentNote string `json:"adjustment_note"`
}

func (r upsertPlanRequest) Validate() []response.ValidationDetail {
	details := []response.ValidationDetail{}
	if _, err := parseDate(r.MealDate); err != nil {
		details = append(details, response.ValidationDetail{Field: "meal_date", Reason: "date_format"})
	}
	if strings.TrimSpace(r.MenuText) == "" {
		details = append(details, response.ValidationDetail{Field: "menu_text", Reason: "required"})
	}
	if len([]rune(r.MenuText)) > 2000 || len([]rune(r.AdjustmentNote)) > 500 {
		details = append(details, response.ValidationDetail{Field: "menu_text", Reason: "too_long"})
	}
	return details
}

type copyPlanRequest struct {
	SourceDate string `json:"source_date"`
	TargetDate string `json:"target_date"`
}

func (r copyPlanRequest) Validate() []response.ValidationDetail {
	var out []response.ValidationDetail
	if _, err := parseDate(r.SourceDate); err != nil {
		out = append(out, response.ValidationDetail{Field: "source_date", Reason: "date_format"})
	}
	if _, err := parseDate(r.TargetDate); err != nil {
		out = append(out, response.ValidationDetail{Field: "target_date", Reason: "date_format"})
	}
	return out
}

type dietNoteRequest struct {
	Note string `json:"note"`
}

func (r dietNoteRequest) Validate() []response.ValidationDetail {
	if len([]rune(r.Note)) > 500 {
		return []response.ValidationDetail{{Field: "note", Reason: "too_long"}}
	}
	return nil
}

type reviewDietNoteChangeRequestRequest struct {
	Status     string `json:"status"`
	ReviewNote string `json:"review_note"`
}

func (r reviewDietNoteChangeRequestRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if r.Status != DietNoteChangeStatusApproved && r.Status != DietNoteChangeStatusRejected {
		details = append(details, response.ValidationDetail{Field: "status", Reason: "invalid_value"})
	}
	if len([]rune(r.ReviewNote)) > 500 {
		details = append(details, response.ValidationDetail{Field: "review_note", Reason: "too_long"})
	}
	return details
}

func (h *Handler) listPlans(c *gin.Context) {
	from, to, err := dateRange(c.Query("from"), c.Query("to"))
	if err != nil {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
		return
	}
	items, err := h.store.ListPlans(c.Request.Context(), h.orgID, from, to)
	if err != nil {
		h.respondError(c, err)
		return
	}
	out := make([]planView, 0, len(items))
	for _, item := range items {
		view := toPlanView(item)
		view.PhotoURL = h.signedPhotoURL(item.PhotoURL)
		out = append(out, view)
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}
func (h *Handler) listParentPlans(c *gin.Context) {
	var from, to *time.Time
	var err error
	if strings.TrimSpace(c.Query("date")) != "" {
		from, err = optionalDate(c.Query("date"))
		to = from
	} else {
		from, to, err = dateRange(c.Query("from"), c.Query("to"))
		if from == nil && to != nil {
			from = to
		}
		if to == nil && from != nil {
			to = from
		}
	}
	if err != nil {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
		return
	}
	if from == nil && to == nil {
		now := time.Now().UTC()
		from, to = &now, &now
	}
	items, err := h.store.ListPlans(c.Request.Context(), h.orgID, from, to)
	if err != nil {
		h.respondError(c, err)
		return
	}
	out := make([]planView, 0, len(items))
	for _, item := range items {
		view := toPlanView(item)
		view.PhotoURL = h.signedPhotoURL(item.PhotoURL)
		out = append(out, view)
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func (h *Handler) upsertPlan(c *gin.Context) {
	if !canWrite(c) {
		return
	}
	var req upsertPlanRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	date, _ := parseDate(req.MealDate)
	principal, _ := identity.PrincipalFromContext(c.Request.Context())
	userID := principal.SubjectID
	item, err := h.store.UpsertPlan(c.Request.Context(), h.orgID, UpsertPlanParams{MealDate: date, MenuText: req.MenuText, PhotoURL: req.PhotoURL, AdjustmentNote: req.AdjustmentNote, CreatedByUserID: &userID, CreatedByName: staffName(principal)})
	if err != nil {
		h.respondError(c, err)
		return
	}
	h.notifyMeal(c, item)
	h.recordAudit(c, "meal.plan.upsert", "meal_plan", item.ID)
	response.OK(c, toPlanView(item))
}
func (h *Handler) copyPlan(c *gin.Context) {
	if !canWrite(c) {
		return
	}
	var req copyPlanRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	source, _ := parseDate(req.SourceDate)
	target, _ := parseDate(req.TargetDate)
	principal, _ := identity.PrincipalFromContext(c.Request.Context())
	uid := principal.SubjectID
	item, err := h.store.CopyPlan(c.Request.Context(), h.orgID, CopyPlanParams{SourceDate: source, TargetDate: target, CreatedByUserID: &uid, CreatedByName: staffName(principal)})
	if err != nil {
		h.respondError(c, err)
		return
	}
	h.notifyMeal(c, item)
	h.recordAudit(c, "meal.plan.copy", "meal_plan", item.ID)
	response.OK(c, toPlanView(item))
}
func (h *Handler) uploadPhoto(c *gin.Context) {
	if !canWrite(c) {
		return
	}
	if h.photos == nil {
		response.Error(c, response.Internal(errors.New("photo storage is not configured")))
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, response.BadRequest("请上传餐食照片", err))
		return
	}
	file, err := header.Open()
	if err != nil {
		response.Error(c, response.BadRequest("无法读取餐食照片", err))
		return
	}
	defer func() { _ = file.Close() }()
	asset, err := saveMealPhoto(c, h.photos, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		if errors.Is(err, storage.ErrFileTooLarge) {
			response.Error(c, response.PayloadTooLarge(err))
			return
		}
		if errors.Is(err, storage.ErrUnsupportedContentType) {
			response.Error(c, response.UnsupportedMediaTypeMessage("仅支持 JPEG、PNG 或 WebP 图片", err))
			return
		}
		response.Error(c, response.BadRequest("餐食照片上传失败", err))
		return
	}
	if err := h.recordAsset(c, asset, "meal_photo", formUint64(c, "meal_plan_id")); err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	response.Created(c, asset.URL, gin.H{"url": asset.URL, "key": asset.Key, "sha256": asset.SHA256, "size": asset.Size, "content_type": asset.ContentType})
}

type categorizedPhotoStore interface {
	SaveIn(context.Context, string, string, string, io.Reader) (storage.Asset, error)
}

func saveMealPhoto(c *gin.Context, photos storage.Store, filename, contentType string, reader io.Reader) (storage.Asset, error) {
	if store, ok := photos.(categorizedPhotoStore); ok {
		return store.SaveIn(c.Request.Context(), "meals", filename, contentType, reader)
	}
	return photos.Save(c.Request.Context(), filename, contentType, reader)
}

func (h *Handler) recordAsset(c *gin.Context, asset storage.Asset, resourceType string, resourceID *uint64) error {
	if h.assets == nil {
		return nil
	}
	key := strings.TrimSpace(asset.Key)
	if key == "" {
		key = strings.TrimPrefix(strings.TrimSpace(asset.URL), "/uploads/")
	}
	var ownerID *uint64
	ownerType := "system"
	if principal, ok := identity.PrincipalFromContext(c.Request.Context()); ok {
		ownerType = "staff"
		ownerID = &principal.SubjectID
	}
	var retentionUntil *time.Time
	if h.assetRetentionDays > 0 {
		value := time.Now().UTC().AddDate(0, 0, h.assetRetentionDays)
		retentionUntil = &value
	}
	_, err := h.assets.Create(c.Request.Context(), h.orgID, mediamodule.CreateParams{ObjectKey: key, ResourceType: resourceType, ResourceID: resourceID, OwnerType: ownerType, OwnerID: ownerID, ContentType: asset.ContentType, SizeBytes: asset.Size, SHA256: asset.SHA256, RetentionUntil: retentionUntil, CreatedByUserID: ownerID})
	return err
}

func formUint64(c *gin.Context, key string) *uint64 {
	value := strings.TrimSpace(c.PostForm(key))
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return nil
	}
	return &parsed
}

func (h *Handler) listDietNotes(c *gin.Context) {
	studentID, err := optionalID(c.Query("student_id"))
	if err != nil {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "student_id", Reason: "invalid_value"}}))
		return
	}
	items, err := h.store.ListDietNotes(c.Request.Context(), h.orgID, studentID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	out := make([]dietNoteView, 0, len(items))
	for _, item := range items {
		if !h.staffStudentAllowed(c, item.StudentID) {
			continue
		}
		out = append(out, toDietNoteView(item))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}
func (h *Handler) getParentDietNote(c *gin.Context) {
	studentID, ok := parsePathID(c, "student_id")
	if !ok {
		return
	}
	if !h.parentOwnsStudent(c, studentID) {
		response.Error(c, response.NotFound())
		return
	}
	items, err := h.store.ListDietNotes(c.Request.Context(), h.orgID, &studentID)
	if err != nil {
		h.respondError(c, err)
		return
	}
	if len(items) == 0 {
		response.OK(c, nil)
		return
	}
	response.OK(c, toDietNoteView(items[0]))
}

func (h *Handler) listParentDietNoteChangeRequests(c *gin.Context) {
	studentID, ok := parsePathID(c, "student_id")
	if !ok || !h.parentOwnsStudent(c, studentID) {
		response.Error(c, response.NotFound())
		return
	}
	items, err := h.store.ListDietNoteChangeRequests(c.Request.Context(), h.orgID, &studentID, nil)
	if err != nil {
		h.respondError(c, err)
		return
	}
	out := make([]dietNoteChangeRequestView, 0, len(items))
	for _, item := range items {
		out = append(out, toDietNoteChangeRequestView(item))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func (h *Handler) parentCreateDietNoteChangeRequest(c *gin.Context) {
	studentID, ok := parsePathID(c, "student_id")
	if !ok || !h.parentOwnsStudent(c, studentID) {
		response.Error(c, response.NotFound())
		return
	}
	principal, principalOK := identity.PrincipalFromContext(c.Request.Context())
	if !principalOK || principal.Kind != identity.PrincipalKindParent {
		response.Error(c, response.Unauthorized())
		return
	}
	var req dietNoteRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.store.CreateDietNoteChangeRequest(c.Request.Context(), h.orgID, CreateDietNoteChangeRequestParams{StudentID: studentID, ParentAccountID: principal.SubjectID, RequestedNote: req.Note})
	if err != nil {
		h.respondError(c, err)
		return
	}
	h.recordAudit(c, "parent.meal.diet_note_change_request.create", "diet_note_change_request", item.ID)
	response.Created(c, "/api/v1/parent/students/"+strconv.FormatUint(studentID, 10)+"/diet-note-requests/"+strconv.FormatUint(item.ID, 10), toDietNoteChangeRequestView(item))
}

func (h *Handler) listDietNoteChangeRequests(c *gin.Context) {
	studentID, err := optionalID(c.Query("student_id"))
	if err != nil {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "student_id", Reason: "invalid_value"}}))
		return
	}
	var status *string
	if value := strings.TrimSpace(c.Query("status")); value != "" {
		if value != DietNoteChangeStatusPending && value != DietNoteChangeStatusApproved && value != DietNoteChangeStatusRejected {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}))
			return
		}
		status = &value
	}
	items, err := h.store.ListDietNoteChangeRequests(c.Request.Context(), h.orgID, studentID, status)
	if err != nil {
		h.respondError(c, err)
		return
	}
	out := make([]dietNoteChangeRequestView, 0, len(items))
	for _, item := range items {
		if !h.staffStudentAllowed(c, item.StudentID) {
			continue
		}
		out = append(out, toDietNoteChangeRequestView(item))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func (h *Handler) reviewDietNoteChangeRequest(c *gin.Context) {
	if !canWrite(c) {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	items, err := h.store.ListDietNoteChangeRequests(c.Request.Context(), h.orgID, nil, nil)
	if err != nil {
		h.respondError(c, err)
		return
	}
	var existing *DietNoteChangeRequest
	for index := range items {
		if items[index].ID == id {
			candidate := items[index]
			existing = &candidate
			break
		}
	}
	if existing == nil {
		response.Error(c, response.NotFound())
		return
	}
	if !h.staffStudentAllowed(c, existing.StudentID) {
		response.Error(c, response.Forbidden())
		return
	}
	var req reviewDietNoteChangeRequestRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	principal, _ := identity.PrincipalFromContext(c.Request.Context())
	item, err := h.store.ReviewDietNoteChangeRequest(c.Request.Context(), h.orgID, ReviewDietNoteChangeRequestParams{ID: id, Status: req.Status, ReviewNote: req.ReviewNote, ReviewedByUserID: principal.SubjectID})
	if err != nil {
		h.respondError(c, err)
		return
	}
	h.notifyDietNoteChange(c, item)
	h.recordAudit(c, "meal.diet_note_change_request.review", "diet_note_change_request", item.ID)
	response.OK(c, toDietNoteChangeRequestView(item))
}
func (h *Handler) upsertDietNote(c *gin.Context) {
	if !canWrite(c) {
		return
	}
	studentID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if !h.staffStudentAllowed(c, studentID) {
		response.Error(c, response.Forbidden())
		return
	}
	var req dietNoteRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	principal, _ := identity.PrincipalFromContext(c.Request.Context())
	uid := principal.SubjectID
	item, err := h.store.UpsertDietNote(c.Request.Context(), h.orgID, UpsertDietNoteParams{StudentID: studentID, Note: req.Note, UpdatedByUserID: &uid, UpdatedByName: staffName(principal)})
	if err != nil {
		h.respondError(c, err)
		return
	}
	h.recordAudit(c, "meal.diet_note.upsert", "student", studentID)
	response.OK(c, toDietNoteView(item))
}

func (h *Handler) parentUpsertDietNote(c *gin.Context) {
	h.parentCreateDietNoteChangeRequest(c)
}

func (h *Handler) notifyDietNoteChange(c *gin.Context, item DietNoteChangeRequest) {
	if h.notifications == nil {
		return
	}
	title, content := "饮食备注申请已处理", "老师已驳回饮食备注变更申请"
	if item.Status == DietNoteChangeStatusApproved {
		title, content = "饮食备注已更新", "老师已确认新的过敏和特殊饮食备注"
	}
	if item.ReviewNote != "" {
		content += "：" + item.ReviewNote
	}
	_, _ = h.notifications.CreateNotification(c.Request.Context(), h.orgID, pickup.CreateNotificationParams{StudentID: item.StudentID, Kind: "meal_diet_note_review", Title: title, Content: content})
}

func (h *Handler) notifyMeal(c *gin.Context, item Plan) {
	if h.notifications == nil || h.masterData == nil {
		return
	}
	students, err := h.masterData.ListStudents(c.Request.Context(), h.orgID)
	if err != nil {
		return
	}
	content := item.MealDate.Format("2006-01-02") + "餐食已更新"
	if item.AdjustmentNote != "" {
		content += "；" + item.AdjustmentNote
	}
	for _, student := range students {
		if student.Status == "active" {
			_, _ = h.notifications.CreateNotification(c.Request.Context(), h.orgID, pickup.CreateNotificationParams{StudentID: student.ID, Kind: "meal_updated", Title: "餐食安排已更新", Content: content})
		}
	}
}
func (h *Handler) parentOwnsStudent(c *gin.Context, studentID uint64) bool {
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindParent || h.parents == nil {
		return false
	}
	items, err := h.parents.ListBindings(c.Request.Context(), h.orgID, p.SubjectID)
	if err != nil {
		return false
	}
	for _, item := range items {
		if item.StudentID == studentID {
			return true
		}
	}
	return false
}
func (h *Handler) staffStudentAllowed(c *gin.Context, studentID uint64) bool {
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindUser {
		return false
	}
	if p.Role != identity.UserRoleTeacher {
		return true
	}
	if h.assignments == nil {
		return false
	}
	student, err := h.masterData.FindStudent(c.Request.Context(), h.orgID, studentID)
	if err != nil {
		return false
	}
	item, err := h.assignments.FindByPair(c.Request.Context(), h.orgID, p.SubjectID, student.SchoolClassID)
	return err == nil && item.Status == assignment.AssignmentStatusActive
}
func canWrite(c *gin.Context) bool {
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindUser || p.Role == identity.UserRoleViewer {
		response.Error(c, response.Forbidden())
		return false
	}
	return true
}
func staffName(p identity.Principal) string {
	if p.SubjectID == 0 {
		return "老师"
	}
	return fmt.Sprintf("用户%d", p.SubjectID)
}
func toPlanView(item Plan) planView {
	return planView{ID: item.ID, MealDate: item.MealDate.Format("2006-01-02"), MenuText: item.MenuText, PhotoURL: item.PhotoURL, AdjustmentNote: item.AdjustmentNote, CreatedByName: item.CreatedByName, Status: item.Status, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}
func toDietNoteView(item DietNote) dietNoteView {
	return dietNoteView{ID: item.ID, StudentID: item.StudentID, Note: item.Note, UpdatedByName: item.UpdatedByName, UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}
func toDietNoteChangeRequestView(item DietNoteChangeRequest) dietNoteChangeRequestView {
	view := dietNoteChangeRequestView{ID: item.ID, StudentID: item.StudentID, ParentAccountID: item.ParentAccountID, CurrentNote: item.CurrentNote, RequestedNote: item.RequestedNote, Status: item.Status, ReviewNote: item.ReviewNote, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
	if item.ReviewedAt != nil {
		value := item.ReviewedAt.Format(time.RFC3339)
		view.ReviewedAt = &value
	}
	return view
}
func (h *Handler) signedPhotoURL(value string) string {
	if h.photoSigner == nil || strings.TrimSpace(value) == "" {
		return value
	}
	return h.photoSigner.Sign(value, 15*time.Minute)
}
func parseDate(v string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(v), time.UTC)
}
func optionalDate(v string) (*time.Time, error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	d, e := parseDate(v)
	return &d, e
}
func optionalID(v string) (*uint64, error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	id, e := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
	if e != nil || id == 0 {
		return nil, e
	}
	return &id, nil
}
func parsePathID(c *gin.Context, key string) (uint64, bool) {
	value, e := strconv.ParseUint(c.Param(key), 10, 64)
	if e != nil || value == 0 {
		response.Error(c, response.BadRequest(key+"不合法", e))
		return 0, false
	}
	return value, true
}
func dateRange(from, to string) (*time.Time, *time.Time, error) {
	f, e := optionalDate(from)
	if e != nil {
		return nil, nil, e
	}
	t, e := optionalDate(to)
	if e != nil {
		return nil, nil, e
	}
	return f, t, nil
}
func (h *Handler) respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(c, response.NotFound())
	case errors.Is(err, ErrConflict):
		response.Error(c, response.BadRequest("已有待审核的饮食备注变更，请等待老师处理", err))
	case errors.Is(err, ErrInvalidState):
		response.Error(c, response.BadRequest("申请状态已变化，请刷新后重试", err))
	default:
		response.Error(c, response.Internal(err))
	}
}
