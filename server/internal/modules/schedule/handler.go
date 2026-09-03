package schedule

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	store       Store
	masterData  masterdata.Store
	assignments assignment.Store
	users       identity.UserStore
	generator   *Generator
	orgID       uint64
}

func NewHandler(store Store, masterData masterdata.Store, assignments assignment.Store, users identity.UserStore, pickupStore pickup.Store) *Handler {
	return &Handler{store: store, masterData: masterData, assignments: assignments, users: users, generator: NewGenerator(store, masterData, pickupStore), orgID: masterdata.DefaultOrganizationID}
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/pickup-schedules", h.list)
	api.POST("/pickup-schedules", h.create)
	api.POST("/pickup-schedules/generate", h.generate)
	api.PUT("/pickup-schedules/:id", h.update)
}

type scheduleRequest struct {
	SchoolID           uint64  `json:"school_id"`
	SchoolClassID      uint64  `json:"school_class_id"`
	CareClassID        *uint64 `json:"care_class_id"`
	Weekday            int     `json:"weekday"`
	PickupMode         string  `json:"pickup_mode"`
	TeacherUserID      *uint64 `json:"teacher_user_id"`
	TeacherName        string  `json:"teacher_name"`
	ExpectedPickupTime string  `json:"expected_pickup_time"`
	EffectiveFrom      string  `json:"effective_from"`
	EffectiveTo        string  `json:"effective_to"`
	Enabled            *bool   `json:"enabled"`
	Notes              string  `json:"notes"`
}

type generateRequest struct {
	Date string `json:"date"`
}

type scheduleView struct {
	ID                 uint64  `json:"id"`
	SchoolID           uint64  `json:"school_id"`
	SchoolClassID      uint64  `json:"school_class_id"`
	CareClassID        *uint64 `json:"care_class_id,omitempty"`
	Weekday            int     `json:"weekday"`
	WeekdayLabel       string  `json:"weekday_label"`
	PickupMode         string  `json:"pickup_mode"`
	TeacherUserID      *uint64 `json:"teacher_user_id,omitempty"`
	TeacherName        string  `json:"teacher_name"`
	ExpectedPickupTime string  `json:"expected_pickup_time"`
	EffectiveFrom      string  `json:"effective_from"`
	EffectiveTo        string  `json:"effective_to,omitempty"`
	Enabled            bool    `json:"enabled"`
	Notes              string  `json:"notes"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	SchoolName         string  `json:"school_name,omitempty"`
	Grade              string  `json:"grade,omitempty"`
	ClassName          string  `json:"class_name,omitempty"`
}

func (h *Handler) list(c *gin.Context) {
	items, err := h.store.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	items, err = h.filterForPrincipal(c, items)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if value := strings.TrimSpace(c.Query("date")); value != "" {
		date, parseErr := parseDate(value)
		if parseErr != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
		filtered := make([]PickupSchedule, 0, len(items))
		for _, item := range items {
			if item.Enabled && item.Weekday == date.Weekday() && !date.Before(day(item.EffectiveFrom)) && (item.EffectiveTo == nil || !date.After(day(*item.EffectiveTo))) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	classes, classErr := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if classErr != nil {
		respondStoreError(c, classErr)
		return
	}
	schools, schoolErr := h.masterData.ListSchools(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if schoolErr != nil {
		respondStoreError(c, schoolErr)
		return
	}
	classMap := make(map[uint64]masterdata.SchoolClass, len(classes))
	for _, item := range classes {
		classMap[item.ID] = item
	}
	schoolMap := make(map[uint64]string, len(schools))
	for _, item := range schools {
		schoolMap[item.ID] = item.Name
	}
	views := make([]scheduleView, 0, len(items))
	for _, item := range items {
		views = append(views, toView(item, classMap, schoolMap))
	}
	response.OK(c, gin.H{"items": views, "total": len(views)})
}

func (h *Handler) create(c *gin.Context) { h.save(c, 0) }

func (h *Handler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	h.save(c, id)
}

func (h *Handler) save(c *gin.Context, id uint64) {
	principal, ok := staffPrincipal(c)
	if !ok || !canWrite(principal) {
		return
	}
	var req scheduleRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	params, err := h.toParams(c, principal, req)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	var item PickupSchedule
	if id == 0 {
		item, err = h.store.Create(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), params)
	} else {
		item, err = h.store.Update(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), UpdateParams{ID: id, CreateParams: params})
	}
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if id == 0 {
		response.Created(c, "/api/v1/pickup-schedules/"+strconv.FormatUint(item.ID, 10), h.viewForRequest(c, item))
	} else {
		response.OK(c, h.viewForRequest(c, item))
	}
}

func (h *Handler) generate(c *gin.Context) {
	principal, ok := staffPrincipal(c)
	if !ok || !canWrite(principal) {
		return
	}
	var req generateRequest
	if c.Request.ContentLength != 0 {
		if err := request.BindJSON(c, &req); err != nil {
			response.Error(c, err)
			return
		}
	}
	date := time.Now().UTC()
	if strings.TrimSpace(req.Date) != "" {
		parsed, err := parseDate(req.Date)
		if err != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
		date = parsed
	}
	items, err := h.store.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	items, err = h.filterForPrincipal(c, items)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	result, err := h.generator.Generate(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), date, items)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) toParams(c *gin.Context, principal identity.Principal, req scheduleRequest) (CreateParams, error) {
	if req.Weekday < 1 || req.Weekday > 7 || req.SchoolID == 0 || req.SchoolClassID == 0 {
		return CreateParams{}, ErrInvalid
	}
	if req.PickupMode == "" {
		req.PickupMode = PickupModeSchool
	}
	if req.PickupMode != PickupModeSchool && req.PickupMode != PickupModeSelf {
		return CreateParams{}, ErrInvalid
	}
	from, err := parseDate(req.EffectiveFrom)
	if err != nil {
		return CreateParams{}, ErrInvalid
	}
	to, err := optionalDate(req.EffectiveTo)
	if err != nil {
		return CreateParams{}, ErrInvalid
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	classItems, err := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return CreateParams{}, err
	}
	var class masterdata.SchoolClass
	for _, item := range classItems {
		if item.ID == req.SchoolClassID {
			class = item
			break
		}
	}
	if class.ID == 0 || class.Status != "active" || class.SchoolID != req.SchoolID {
		return CreateParams{}, ErrInvalid
	}
	if principal.Role == identity.UserRoleTeacher {
		if err := h.requireAssignment(c, principal.SubjectID, req.SchoolClassID); err != nil {
			return CreateParams{}, err
		}
		req.TeacherUserID = &principal.SubjectID
		req.TeacherName = h.teacherName(c, principal.SubjectID)
	} else if req.TeacherUserID != nil {
		if err := h.requireAssignment(c, *req.TeacherUserID, req.SchoolClassID); err != nil {
			return CreateParams{}, err
		}
		req.TeacherName = h.teacherName(c, *req.TeacherUserID)
	}
	if len([]rune(strings.TrimSpace(req.ExpectedPickupTime))) > 16 || len([]rune(strings.TrimSpace(req.Notes))) > 500 {
		return CreateParams{}, ErrInvalid
	}
	return CreateParams{SchoolID: req.SchoolID, SchoolClassID: req.SchoolClassID, CareClassID: req.CareClassID, Weekday: time.Weekday(req.Weekday % 7), PickupMode: req.PickupMode, TeacherUserID: req.TeacherUserID, TeacherName: strings.TrimSpace(req.TeacherName), ExpectedPickupTime: strings.TrimSpace(req.ExpectedPickupTime), EffectiveFrom: from, EffectiveTo: to, Enabled: enabled, Notes: strings.TrimSpace(req.Notes)}, nil
}

func (h *Handler) filterForPrincipal(c *gin.Context, items []PickupSchedule) ([]PickupSchedule, error) {
	principal, ok := staffPrincipal(c)
	if !ok || principal.Role != identity.UserRoleTeacher {
		return items, nil
	}
	if h.assignments == nil {
		return nil, ErrUnauthorized
	}
	assigned, err := h.assignments.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), principal.SubjectID, 0)
	if err != nil {
		return nil, err
	}
	classes := map[uint64]struct{}{}
	for _, item := range assigned {
		if item.Status == assignment.AssignmentStatusActive {
			classes[item.SchoolClassID] = struct{}{}
		}
	}
	out := make([]PickupSchedule, 0, len(items))
	for _, item := range items {
		if _, allowed := classes[item.SchoolClassID]; allowed {
			out = append(out, item)
		}
	}
	return out, nil
}

func (h *Handler) requireAssignment(c *gin.Context, teacherID, classID uint64) error {
	if h.assignments == nil {
		return ErrUnauthorized
	}
	item, err := h.assignments.FindByPair(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), teacherID, classID)
	if err != nil {
		if errors.Is(err, assignment.ErrNotFound) {
			return ErrUnauthorized
		}
		return err
	}
	if item.Status != assignment.AssignmentStatusActive {
		return ErrUnauthorized
	}
	return nil
}

func (h *Handler) teacherName(c *gin.Context, id uint64) string {
	if h.users != nil {
		if item, err := h.users.FindUserByID(c.Request.Context(), id); err == nil && strings.TrimSpace(item.Nickname) != "" {
			return strings.TrimSpace(item.Nickname)
		}
	}
	return "老师"
}
func (h *Handler) viewForRequest(c *gin.Context, item PickupSchedule) scheduleView {
	classes, _ := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	schools, _ := h.masterData.ListSchools(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	classMap := map[uint64]masterdata.SchoolClass{}
	schoolMap := map[uint64]string{}
	for _, v := range classes {
		classMap[v.ID] = v
	}
	for _, v := range schools {
		schoolMap[v.ID] = v.Name
	}
	return toView(item, classMap, schoolMap)
}

func toView(item PickupSchedule, classes map[uint64]masterdata.SchoolClass, schools map[uint64]string) scheduleView {
	view := scheduleView{ID: item.ID, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, CareClassID: item.CareClassID, Weekday: int(item.Weekday), WeekdayLabel: weekdayLabel(item.Weekday), PickupMode: item.PickupMode, TeacherUserID: item.TeacherUserID, TeacherName: item.TeacherName, ExpectedPickupTime: item.ExpectedPickupTime, EffectiveFrom: item.EffectiveFrom.Format("2006-01-02"), Enabled: item.Enabled, Notes: item.Notes, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339), SchoolName: schools[item.SchoolID]}
	if item.EffectiveTo != nil {
		view.EffectiveTo = item.EffectiveTo.Format("2006-01-02")
	}
	if class, ok := classes[item.SchoolClassID]; ok {
		view.Grade, view.ClassName = class.Grade, class.Name
	}
	return view
}

func weekdayLabel(value time.Weekday) string {
	return map[time.Weekday]string{time.Monday: "周一", time.Tuesday: "周二", time.Wednesday: "周三", time.Thursday: "周四", time.Friday: "周五", time.Saturday: "周六", time.Sunday: "周日"}[value]
}
func parseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.UTC)
}
func optionalDate(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := parseDate(value)
	return &parsed, err
}
func day(value time.Time) time.Time { return value.UTC().Truncate(24 * time.Hour) }
func parseID(c *gin.Context, key string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || value == 0 {
		response.Error(c, response.BadRequest(fmt.Sprintf("%s 不合法", key), err))
		return 0, false
	}
	return value, true
}
func staffPrincipal(c *gin.Context) (identity.Principal, bool) {
	item, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || item.Kind != identity.PrincipalKindUser {
		response.Error(c, response.Unauthorized())
		return identity.Principal{}, false
	}
	return item, true
}
func canWrite(item identity.Principal) bool {
	if item.Role == identity.UserRoleViewer {
		return false
	}
	return true
}
func respondStoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		response.Error(c, response.Forbidden())
	case errors.Is(err, ErrConflict):
		response.Error(c, response.BadRequest("同一班级同一星期的排班已经存在", err))
	case errors.Is(err, ErrInvalid):
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "schedule", Reason: "invalid_value"}}))
	case errors.Is(err, masterdata.ErrNotFound):
		response.Error(c, response.NotFound())
	default:
		response.Error(c, response.Internal(err))
	}
}
