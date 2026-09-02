package assignment

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	store      Store
	users      identity.UserStore
	masterData masterdata.Store
	orgID      uint64
}

func NewHandler(store Store, users identity.UserStore, masterData masterdata.Store) *Handler {
	return &Handler{store: store, users: users, masterData: masterData, orgID: masterdata.DefaultOrganizationID}
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/teacher-assignments", h.list)
	api.POST("/teacher-assignments", h.create)
	api.PUT("/teacher-assignments/:id", h.update)
}

type assignmentView struct {
	ID            uint64 `json:"id"`
	TeacherUserID uint64 `json:"teacher_user_id"`
	TeacherName   string `json:"teacher_name"`
	Username      string `json:"username"`
	SchoolClassID uint64 `json:"school_class_id"`
	SchoolID      uint64 `json:"school_id"`
	Grade         string `json:"grade"`
	ClassName     string `json:"class_name"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type assignmentRequest struct {
	TeacherUserID uint64 `json:"teacher_user_id"`
	SchoolClassID uint64 `json:"school_class_id"`
	Status        string `json:"status"`
}

type assignmentStatusRequest struct {
	Status string `json:"status"`
}

func (r assignmentRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 3)
	if r.TeacherUserID == 0 {
		details = append(details, response.ValidationDetail{Field: "teacher_user_id", Reason: "required"})
	}
	if r.SchoolClassID == 0 {
		details = append(details, response.ValidationDetail{Field: "school_class_id", Reason: "required"})
	}
	if r.Status != "" && r.Status != AssignmentStatusActive && r.Status != AssignmentStatusDisabled {
		details = append(details, response.ValidationDetail{Field: "status", Reason: "invalid_value"})
	}
	return details
}

func (h *Handler) list(c *gin.Context) {
	principal, ok := staffPrincipal(c)
	if !ok {
		return
	}
	var teacherUserID uint64
	if principal.Role == identity.UserRoleTeacher {
		teacherUserID = principal.SubjectID
	} else if value := strings.TrimSpace(c.Query("teacher_user_id")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "teacher_user_id", Reason: "invalid_value"}}))
			return
		}
		teacherUserID = parsed
	}
	var schoolClassID uint64
	if value := strings.TrimSpace(c.Query("school_class_id")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "school_class_id", Reason: "invalid_value"}}))
			return
		}
		schoolClassID = parsed
	}
	items, err := h.store.List(c.Request.Context(), h.orgID, teacherUserID, schoolClassID)
	if err != nil {
		respondError(c, err)
		return
	}
	views := make([]assignmentView, 0, len(items))
	for _, item := range items {
		view, enrichErr := h.toView(c.Request.Context(), item)
		if enrichErr != nil {
			respondError(c, enrichErr)
			return
		}
		views = append(views, view)
	}
	response.OK(c, gin.H{"items": views, "total": len(views)})
}

func (h *Handler) create(c *gin.Context) {
	if !canManageAssignments(c) {
		return
	}
	var req assignmentRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if err := h.validateRelations(c.Request.Context(), req.TeacherUserID, req.SchoolClassID); err != nil {
		respondError(c, err)
		return
	}
	item, err := h.store.Create(c.Request.Context(), h.orgID, CreateParams{TeacherUserID: req.TeacherUserID, SchoolClassID: req.SchoolClassID})
	if err != nil {
		respondError(c, err)
		return
	}
	view, err := h.toView(c.Request.Context(), item)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Created(c, "/api/v1/teacher-assignments/"+strconv.FormatUint(item.ID, 10), view)
}

func (h *Handler) update(c *gin.Context) {
	if !canManageAssignments(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, response.BadRequest("分配 ID 不合法", err))
		return
	}
	var req assignmentStatusRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if req.Status == "" {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "status", Reason: "required"}}))
		return
	}
	item, err := h.store.SetStatus(c.Request.Context(), h.orgID, SetStatusParams{ID: id, Status: req.Status})
	if err != nil {
		respondError(c, err)
		return
	}
	view, err := h.toView(c.Request.Context(), item)
	if err != nil {
		respondError(c, err)
		return
	}
	response.OK(c, view)
}

func (h *Handler) validateRelations(ctx context.Context, teacherUserID, schoolClassID uint64) error {
	user, err := h.users.FindUserByID(ctx, teacherUserID)
	if err != nil {
		return err
	}
	if user.Role != identity.UserRoleTeacher || user.Status != identity.UserStatusActive {
		return errors.New("assignment: user is not an active teacher")
	}
	class, err := h.masterData.ListSchoolClasses(ctx, h.orgID)
	if err != nil {
		return err
	}
	for _, item := range class {
		if item.ID == schoolClassID && item.Status == "active" {
			return nil
		}
	}
	return ErrNotFound
}

func (h *Handler) toView(ctx context.Context, item TeacherClassAssignment) (assignmentView, error) {
	user, err := h.users.FindUserByID(ctx, item.TeacherUserID)
	if err != nil {
		return assignmentView{}, err
	}
	classes, err := h.masterData.ListSchoolClasses(ctx, h.orgID)
	if err != nil {
		return assignmentView{}, err
	}
	view := assignmentView{ID: item.ID, TeacherUserID: item.TeacherUserID, TeacherName: user.Nickname, Username: user.Username, SchoolClassID: item.SchoolClassID, Status: item.Status, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
	for _, schoolClass := range classes {
		if schoolClass.ID == item.SchoolClassID {
			view.SchoolID, view.Grade, view.ClassName = schoolClass.SchoolID, schoolClass.Grade, schoolClass.Name
			break
		}
	}
	return view, nil
}

func staffPrincipal(c *gin.Context) (identity.Principal, bool) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser {
		response.Error(c, response.Unauthorized())
		return identity.Principal{}, false
	}
	return principal, true
}

func canManageAssignments(c *gin.Context) bool {
	principal, ok := staffPrincipal(c)
	if !ok {
		return false
	}
	if principal.Role != identity.UserRoleAdmin && principal.Role != identity.UserRoleEditor {
		response.Error(c, response.BadRequest("没有配置教师负责班级的权限", nil))
		return false
	}
	return true
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(c, response.NotFound())
	case errors.Is(err, ErrConflict):
		response.Error(c, response.BadRequest("该教师与学校班级已经分配过", err))
	case errors.Is(err, ErrInvalidStatus):
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}))
	case errors.Is(err, identity.ErrUserNotFound):
		response.Error(c, response.NotFound())
	default:
		if strings.Contains(err.Error(), "active teacher") {
			response.Error(c, response.BadRequest("只能给启用中的教师账号分配班级", err))
			return
		}
		response.Error(c, response.Internal(err))
	}
}
