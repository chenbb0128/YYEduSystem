package summary

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	auditmodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/audit"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/homework"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/meal"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/businessdate"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	store         Store
	pickup        pickup.Store
	homework      homework.Store
	meals         meal.Store
	masterData    masterdata.Store
	parents       parent.Store
	notifications pickup.NotificationWriter
	assignments   assignment.Store
	audit         auditmodule.Writer
	orgID         uint64
}

func NewHandler(store Store, pickupStore pickup.Store, homeworkStore homework.Store, mealStore meal.Store, masterData masterdata.Store) *Handler {
	return &Handler{store: store, pickup: pickupStore, homework: homeworkStore, meals: mealStore, masterData: masterData, orgID: masterdata.DefaultOrganizationID}
}
func (h *Handler) SetParentStore(value parent.Store)                     { h.parents = value }
func (h *Handler) SetNotificationWriter(value pickup.NotificationWriter) { h.notifications = value }
func (h *Handler) SetStaffScope(value assignment.Store)                  { h.assignments = value }
func (h *Handler) SetAuditWriter(value auditmodule.Writer)               { h.audit = value }

func (h *Handler) recordAudit(c *gin.Context, action, resourceType string, resourceID uint64) {
	auditmodule.RecordForContext(c.Request.Context(), h.audit, identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), action, resourceType, &resourceID, "{}", c.GetHeader("X-Request-ID"))
}
func (h *Handler) RegisterStaffRoutes(api *gin.RouterGroup) {
	api.GET("/daily-summaries", h.list)
	api.GET("/daily-summaries/:id/versions", h.listVersions)
	api.POST("/daily-summaries/generate", h.generate)
	api.PUT("/daily-summaries/:id", h.update)
	api.POST("/daily-summaries/:id/publish", h.publish)
	api.POST("/daily-summaries/:id/close", h.close)
	api.POST("/daily-summaries/:id/withdraw", h.withdraw)
	api.POST("/daily-summaries/:id/correct", h.correct)
}
func (h *Handler) RegisterParentRoutes(api *gin.RouterGroup) {
	api.GET("/parent/daily-summary", h.parentSummary)
	api.POST("/parent/daily-summary/:id/read", h.parentRead)
}

type view struct {
	ID               uint64            `json:"id"`
	SummaryDate      string            `json:"summary_date"`
	Content          string            `json:"content"`
	ChildUpdates     map[uint64]string `json:"child_updates,omitempty"`
	Status           string            `json:"status"`
	Version          uint32            `json:"version"`
	CreatedByName    string            `json:"created_by_name"`
	GeneratedAt      *string           `json:"generated_at,omitempty"`
	PublishedAt      *string           `json:"published_at,omitempty"`
	ClosedAt         *string           `json:"closed_at,omitempty"`
	WithdrawnAt      *string           `json:"withdrawn_at,omitempty"`
	WithdrawalReason string            `json:"withdrawal_reason,omitempty"`
	CorrectionReason string            `json:"correction_reason,omitempty"`
	ReadAt           *string           `json:"read_at,omitempty"`
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
}

type versionView struct {
	ID            uint64            `json:"id"`
	Version       uint32            `json:"version"`
	Action        string            `json:"action"`
	Content       string            `json:"content"`
	ChildUpdates  map[uint64]string `json:"child_updates,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	CreatedByName string            `json:"created_by_name"`
	CreatedAt     string            `json:"created_at"`
}
type generateRequest struct {
	SummaryDate string `json:"summary_date"`
}

func (r generateRequest) Validate() []response.ValidationDetail {
	if _, e := parseDate(r.SummaryDate); e != nil {
		return []response.ValidationDetail{{Field: "summary_date", Reason: "date_format"}}
	}
	return nil
}

type updateRequest struct {
	Content      string            `json:"content"`
	ChildUpdates map[uint64]string `json:"child_updates"`
}

type withdrawRequest struct {
	Reason string `json:"reason"`
}

func (r withdrawRequest) Validate() []response.ValidationDetail {
	if strings.TrimSpace(r.Reason) == "" {
		return []response.ValidationDetail{{Field: "reason", Reason: "required"}}
	}
	if len([]rune(r.Reason)) > 500 {
		return []response.ValidationDetail{{Field: "reason", Reason: "too_long"}}
	}
	return nil
}

type correctRequest struct {
	Content      string            `json:"content"`
	ChildUpdates map[uint64]string `json:"child_updates"`
	Reason       string            `json:"reason"`
}

func (r correctRequest) Validate() []response.ValidationDetail {
	details := updateRequest{Content: r.Content, ChildUpdates: r.ChildUpdates}.Validate()
	if strings.TrimSpace(r.Reason) == "" {
		details = append(details, response.ValidationDetail{Field: "reason", Reason: "required"})
	} else if len([]rune(r.Reason)) > 500 {
		details = append(details, response.ValidationDetail{Field: "reason", Reason: "too_long"})
	}
	return details
}

func (r updateRequest) Validate() []response.ValidationDetail {
	if strings.TrimSpace(r.Content) == "" {
		return []response.ValidationDetail{{Field: "content", Reason: "required"}}
	}
	if len([]rune(r.Content)) > 4000 {
		return []response.ValidationDetail{{Field: "content", Reason: "too_long"}}
	}
	return nil
}

func (h *Handler) list(c *gin.Context) {
	var date *time.Time
	if strings.TrimSpace(c.Query("date")) != "" {
		parsed, e := parseDate(c.Query("date"))
		if e != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
		date = &parsed
	}
	items, e := h.store.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), date)
	if e != nil {
		h.respond(c, e)
		return
	}
	out := make([]view, 0, len(items))
	for _, item := range items {
		if !h.staffAllowedSummary(c, item) {
			continue
		}
		out = append(out, toView(item, false))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func (h *Handler) listVersions(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.store.Find(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if err != nil {
		h.respond(c, err)
		return
	}
	if !h.staffAllowedSummary(c, item) {
		response.Error(c, response.Forbidden())
		return
	}
	items, err := h.store.ListVersions(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if err != nil {
		h.respond(c, err)
		return
	}
	out := make([]versionView, 0, len(items))
	for _, version := range items {
		out = append(out, versionView{ID: version.ID, Version: version.Version, Action: version.Action, Content: version.Content, ChildUpdates: version.ChildUpdates, Reason: version.Reason, CreatedByName: version.CreatedByName, CreatedAt: version.CreatedAt.Format(time.RFC3339)})
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}
func (h *Handler) generate(c *gin.Context) {
	if !canWrite(c) {
		return
	}
	if !h.staffCanUseSummary(c) {
		response.Error(c, response.Forbidden())
		return
	}
	var req generateRequest
	if e := request.BindJSON(c, &req); e != nil {
		response.Error(c, e)
		return
	}
	date, _ := parseDate(req.SummaryDate)
	content, updates := h.buildDraft(c, date)
	principal, _ := identity.PrincipalFromContext(c.Request.Context())
	uid := principal.SubjectID
	item, e := h.store.Generate(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), GenerateParams{SummaryDate: date, Content: content, ChildUpdates: updates, CreatedByUserID: &uid, CreatedByName: fmt.Sprintf("用户%d", uid)})
	if e != nil {
		h.respond(c, e)
		return
	}
	h.recordAudit(c, "summary.generate", "daily_summary", item.ID)
	response.OK(c, toView(item, false))
}
func (h *Handler) update(c *gin.Context) {
	if !canWrite(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	existing, e := h.store.Find(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if e != nil {
		h.respond(c, e)
		return
	}
	if !h.staffAllowedSummary(c, existing) {
		response.Error(c, response.Forbidden())
		return
	}
	var req updateRequest
	if e := request.BindJSON(c, &req); e != nil {
		response.Error(c, e)
		return
	}
	childUpdates, allowed := h.filterChildUpdates(c, req.ChildUpdates)
	if !allowed {
		response.Error(c, response.Forbidden())
		return
	}
	item, e := h.store.Update(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), UpdateParams{ID: id, Content: req.Content, ChildUpdates: childUpdates})
	if e != nil {
		h.respond(c, e)
		return
	}
	h.recordAudit(c, "summary.update", "daily_summary", item.ID)
	response.OK(c, toView(item, false))
}
func (h *Handler) publish(c *gin.Context) { h.setStatus(c, StatusPublished) }
func (h *Handler) close(c *gin.Context)   { h.setStatus(c, StatusClosed) }

func (h *Handler) withdraw(c *gin.Context) {
	if !canWrite(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	existing, err := h.store.Find(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if err != nil {
		h.respond(c, err)
		return
	}
	if !h.staffAllowedSummary(c, existing) {
		response.Error(c, response.Forbidden())
		return
	}
	var req withdrawRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.store.Withdraw(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id, req.Reason)
	if err != nil {
		h.respond(c, err)
		return
	}
	h.recordAudit(c, "summary.withdraw", "daily_summary", item.ID)
	h.notifyWithdrawn(c, item)
	response.OK(c, toView(item, false))
}

func (h *Handler) correct(c *gin.Context) {
	if !canWrite(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	existing, err := h.store.Find(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if err != nil {
		h.respond(c, err)
		return
	}
	if !h.staffAllowedSummary(c, existing) {
		response.Error(c, response.Forbidden())
		return
	}
	var req correctRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	childUpdates, allowed := h.filterChildUpdates(c, req.ChildUpdates)
	if !allowed {
		response.Error(c, response.Forbidden())
		return
	}
	principal, _ := identity.PrincipalFromContext(c.Request.Context())
	item, err := h.store.Correct(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CorrectParams{ID: id, Content: req.Content, ChildUpdates: childUpdates, Reason: req.Reason, CreatedByUserID: &principal.SubjectID, CreatedByName: fmt.Sprintf("用户%d", principal.SubjectID)})
	if err != nil {
		h.respond(c, err)
		return
	}
	h.recordAudit(c, "summary.correct", "daily_summary", item.ID)
	h.notifyPublished(c, item)
	response.OK(c, toView(item, false))
}
func (h *Handler) setStatus(c *gin.Context, status string) {
	if !canWrite(c) {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	existing, e := h.store.Find(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if e != nil {
		h.respond(c, e)
		return
	}
	if !h.staffAllowedSummary(c, existing) {
		response.Error(c, response.Forbidden())
		return
	}
	item, e := h.store.SetStatus(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id, status)
	if e != nil {
		h.respond(c, e)
		return
	}
	if status == StatusPublished {
		h.notifyPublished(c, item)
	}
	h.recordAudit(c, "summary."+status, "daily_summary", item.ID)
	response.OK(c, toView(item, false))
}

func (h *Handler) notifyPublished(c *gin.Context, item DailySummary) {
	if h.notifications == nil || h.masterData == nil {
		return
	}
	students, err := h.masterData.ListStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return
	}
	for _, student := range students {
		if student.Status != "active" || !h.studentAllowedForStaff(c, student) {
			continue
		}
		content := fmt.Sprintf("%s 的托管每日总结已发布", student.Name)
		if update := strings.TrimSpace(item.ChildUpdates[student.ID]); update != "" {
			content += "：" + update
		}
		_, _ = h.notifications.CreateNotification(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), pickup.CreateNotificationParams{StudentID: student.ID, Kind: "daily_summary_published", Title: "教师每日总结已发布", Content: content})
	}
}

func (h *Handler) notifyWithdrawn(c *gin.Context, item DailySummary) {
	if h.notifications == nil || h.masterData == nil {
		return
	}
	students, err := h.masterData.ListStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return
	}
	for _, student := range students {
		if student.Status != "active" || !h.studentAllowedForStaff(c, student) {
			continue
		}
		_, _ = h.notifications.CreateNotification(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), pickup.CreateNotificationParams{StudentID: student.ID, Kind: "daily_summary_withdrawn", Title: "教师每日总结已撤回", Content: fmt.Sprintf("%s 的托管每日总结已撤回，请以最新通知为准", student.Name)})
	}
}
func (h *Handler) parentSummary(c *gin.Context) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindParent || h.parents == nil {
		response.Error(c, response.Unauthorized())
		return
	}
	date := businessdate.Today(time.Now())
	if strings.TrimSpace(c.Query("date")) != "" {
		parsed, err := parseDate(c.Query("date"))
		if err != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
		date = parsed
	}
	items, e := h.store.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), &date)
	if e != nil {
		h.respond(c, e)
		return
	}
	bindings, e := h.parents.ListBindings(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), principal.SubjectID)
	if e != nil {
		response.Error(c, response.NotFound())
		return
	}
	if len(bindings) == 0 {
		response.Error(c, response.NotFound())
		return
	}
	allowed := map[uint64]struct{}{}
	for _, b := range bindings {
		allowed[b.StudentID] = struct{}{}
	}
	for _, item := range items {
		if item.Status != StatusPublished && item.Status != StatusClosed {
			continue
		}
		child := map[uint64]string{}
		for id, text := range item.ChildUpdates {
			if _, yes := allowed[id]; yes {
				child[id] = text
			}
		}
		readAt, _ := h.store.ReadAt(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), item.ID, principal.SubjectID)
		response.OK(c, toViewWithChildren(item, child, readAt))
		return
	}
	response.OK(c, nil)
}

func (h *Handler) parentRead(c *gin.Context) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindParent || h.parents == nil {
		response.Error(c, response.Unauthorized())
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.store.Find(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if err != nil {
		h.respond(c, err)
		return
	}
	if item.Status != StatusPublished && item.Status != StatusClosed {
		response.Error(c, response.NotFound())
		return
	}
	bindings, err := h.parents.ListBindings(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), principal.SubjectID)
	if err != nil {
		response.Error(c, response.NotFound())
		return
	}
	if len(bindings) == 0 {
		response.Error(c, response.NotFound())
		return
	}
	// A summary without child-specific updates is an institution-wide daily
	// notice and can be read by any bound parent. When child updates exist,
	// require at least one update to belong to this parent's bound children so
	// an unrelated parent cannot acknowledge another child's summary record.
	if len(item.ChildUpdates) > 0 {
		allowed := make(map[uint64]struct{}, len(bindings))
		for _, binding := range bindings {
			allowed[binding.StudentID] = struct{}{}
		}
		belongsToParent := false
		for studentID := range item.ChildUpdates {
			if _, exists := allowed[studentID]; exists {
				belongsToParent = true
				break
			}
		}
		if !belongsToParent {
			response.Error(c, response.NotFound())
			return
		}
	}
	if err := h.store.MarkRead(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id, principal.SubjectID, item.Version); err != nil {
		h.respond(c, err)
		return
	}
	readAt, _ := h.store.ReadAt(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id, principal.SubjectID)
	response.OK(c, gin.H{"id": id, "read": true, "read_at": format(readAt)})
}

func (h *Handler) buildDraft(c *gin.Context, date time.Time) (string, map[uint64]string) {
	ops := []pickup.Operation{}
	if h.pickup != nil {
		all, _ := h.pickup.ListOperations(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		for _, op := range all {
			if sameDay(op.OperationDate, date) && h.operationAllowed(c, op) {
				ops = append(ops, op)
			}
		}
	}
	tasks := []homework.Task{}
	if h.homework != nil {
		all, _ := h.homework.ListTasks(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		for _, task := range all {
			if sameDay(task.HomeworkDate, date) && h.staffClassAllowed(c, task.SchoolClassID) {
				tasks = append(tasks, task)
			}
		}
	}
	mealCount := 0
	if h.meals != nil {
		from, to := date, date
		plans, _ := h.meals.ListPlans(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), &from, &to)
		mealCount = len(plans)
	}
	students := 0
	arrived := 0
	left := 0
	updates := map[uint64]string{}
	for _, op := range ops {
		members, _ := h.pickup.ListOperationStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), op.ID)
		students += len(members)
		for _, member := range members {
			switch member.Status {
			case pickup.MemberStatusArrived, pickup.MemberStatusLeft, pickup.MemberStatusMidwayLeft:
				arrived++
				if member.Status == pickup.MemberStatusLeft || member.Status == pickup.MemberStatusMidwayLeft {
					left++
				}
			}
			if member.StudentID != 0 {
				updates[member.StudentID] = pickupStatusText(member.Status)
			}
		}
	}
	content := fmt.Sprintf("今日完成 %d 个接送任务，记录 %d 名孩子；其中 %d 名已到班，%d 名已离班；发布 %d 份作业。", len(ops), students, arrived, left, len(tasks))
	if mealCount > 0 {
		content += " 今日餐食已登记。"
	}
	return content, updates
}
func (h *Handler) staffAllowedSummary(c *gin.Context, item DailySummary) bool {
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindUser || p.Role != identity.UserRoleTeacher || h.assignments == nil {
		return true
	}
	if !h.staffCanUseSummary(c) {
		return false
	}
	if h.pickup == nil {
		return true
	}
	operations, err := h.pickup.ListOperations(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return false
	}
	for _, operation := range operations {
		if sameDay(operation.OperationDate, item.SummaryDate) && h.operationAllowed(c, operation) {
			return true
		}
	}
	return true
}

func (h *Handler) staffCanUseSummary(c *gin.Context) bool {
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindUser || p.Role != identity.UserRoleTeacher {
		return true
	}
	if h.assignments == nil {
		return false
	}
	items, err := h.assignments.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), p.SubjectID, 0)
	if err != nil {
		return false
	}
	for _, item := range items {
		if item.Status == assignment.AssignmentStatusActive {
			return true
		}
	}
	return false
}

func (h *Handler) filterChildUpdates(c *gin.Context, updates map[uint64]string) (map[uint64]string, bool) {
	filtered := make(map[uint64]string, len(updates))
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindUser || p.Role != identity.UserRoleTeacher {
		for id, text := range updates {
			filtered[id] = strings.TrimSpace(text)
		}
		return filtered, true
	}
	if h.masterData == nil || h.assignments == nil {
		return nil, false
	}
	for id, text := range updates {
		student, err := h.masterData.FindStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
		if err != nil || !h.classAllowed(c, p.SubjectID, student.SchoolClassID) {
			return nil, false
		}
		filtered[id] = strings.TrimSpace(text)
	}
	return filtered, true
}

func (h *Handler) studentAllowedForStaff(c *gin.Context, student masterdata.Student) bool {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser || principal.Role != identity.UserRoleTeacher {
		return true
	}
	return h.assignments != nil && h.classAllowed(c, principal.SubjectID, student.SchoolClassID)
}

func (h *Handler) classAllowed(c *gin.Context, teacherID, schoolClassID uint64) bool {
	item, err := h.assignments.FindByPair(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), teacherID, schoolClassID)
	return err == nil && item.Status == assignment.AssignmentStatusActive
}

func (h *Handler) staffClassAllowed(c *gin.Context, schoolClassID uint64) bool {
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindUser || p.Role != identity.UserRoleTeacher {
		return true
	}
	return h.assignments != nil && h.classAllowed(c, p.SubjectID, schoolClassID)
}
func (h *Handler) operationAllowed(c *gin.Context, op pickup.Operation) bool {
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindUser || p.Role != identity.UserRoleTeacher || h.assignments == nil {
		return true
	}
	item, e := h.assignments.FindByPair(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), p.SubjectID, op.SchoolClassID)
	return e == nil && item.Status == assignment.AssignmentStatusActive
}
func canWrite(c *gin.Context) bool {
	p, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || p.Kind != identity.PrincipalKindUser || p.Role == identity.UserRoleViewer {
		response.Error(c, response.Forbidden())
		return false
	}
	return true
}
func toView(item DailySummary, hideChildren bool) view {
	child := map[uint64]string{}
	if !hideChildren {
		child = item.ChildUpdates
	}
	return view{ID: item.ID, SummaryDate: item.SummaryDate.Format("2006-01-02"), Content: item.Content, ChildUpdates: child, Status: item.Status, Version: item.Version, CreatedByName: item.CreatedByName, GeneratedAt: format(item.GeneratedAt), PublishedAt: format(item.PublishedAt), ClosedAt: format(item.ClosedAt), WithdrawnAt: format(item.WithdrawnAt), WithdrawalReason: item.WithdrawalReason, CorrectionReason: item.CorrectionReason, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}
func toViewWithChildren(item DailySummary, children map[uint64]string, readAt *time.Time) view {
	return view{ID: item.ID, SummaryDate: item.SummaryDate.Format("2006-01-02"), Content: item.Content, ChildUpdates: children, Status: item.Status, Version: item.Version, CreatedByName: item.CreatedByName, PublishedAt: format(item.PublishedAt), ReadAt: format(readAt), CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}
func format(v *time.Time) *string {
	if v == nil {
		return nil
	}
	x := v.Format(time.RFC3339)
	return &x
}
func parseDate(v string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(v), time.UTC)
}
func parseID(c *gin.Context) (uint64, bool) {
	id, e := strconv.ParseUint(c.Param("id"), 10, 64)
	if e != nil || id == 0 {
		response.Error(c, response.BadRequest("id不合法", e))
		return 0, false
	}
	return id, true
}
func pickupStatusText(v string) string {
	return map[string]string{"picked_up": "已在校门口接到", "self_arrived": "自行到班", "parent_picked_up": "家长接走", "leave": "请假", "absent": "暂未找到", "arrived": "已到托管班", "left": "已离班", "midway_left": "中途离班", "abnormal": "异常"}[v]
}
func (h *Handler) respond(c *gin.Context, e error) {
	switch {
	case errors.Is(e, ErrNotFound):
		response.Error(c, response.NotFound())
	case errors.Is(e, ErrInvalidState):
		response.Error(c, response.BadRequest("当前总结状态不允许此操作", e))
	default:
		response.Error(c, response.Internal(e))
	}
}
