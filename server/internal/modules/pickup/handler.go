package pickup

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
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/storage"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	store              Store
	masterData         masterdata.Store
	photos             storage.Store
	photoSigner        *storage.URLSigner
	assets             mediamodule.Store
	assetRetentionDays int
	leaveReader        LeaveReader
	assignments        assignment.Store
	users              identity.UserStore
	audit              auditmodule.Writer
	orgID              uint64
}

type LeaveReader interface {
	ListApprovedLeaveStudentIDs(context.Context, uint64, time.Time) (map[uint64]struct{}, error)
}

func NewHandler(store Store, masterData masterdata.Store, photos storage.Store, leaveReaders ...LeaveReader) *Handler {
	var leaveReader LeaveReader
	if len(leaveReaders) > 0 {
		leaveReader = leaveReaders[0]
	}
	return &Handler{store: store, masterData: masterData, photos: photos, leaveReader: leaveReader, orgID: masterdata.DefaultOrganizationID}
}

// SetStaffScope enables identity-aware access control while keeping the focused
// handler tests and other embedders compatible with the original constructor.
func (h *Handler) SetStaffScope(assignments assignment.Store, users identity.UserStore) {
	h.assignments = assignments
	h.users = users
}

func (h *Handler) SetPhotoSigner(signer *storage.URLSigner) { h.photoSigner = signer }

func (h *Handler) SetAssetStore(store mediamodule.Store) { h.assets = store }

func (h *Handler) SetAssetRetentionDays(days int) { h.assetRetentionDays = days }

func (h *Handler) SetAuditWriter(writer auditmodule.Writer) { h.audit = writer }

func (h *Handler) recordAudit(c *gin.Context, action, resourceType string, resourceID uint64) {
	auditmodule.RecordForContext(c.Request.Context(), h.audit, h.orgID, action, resourceType, &resourceID, "{}", c.GetHeader("X-Request-ID"))
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/pickup-operations", h.listOperations)
	api.POST("/pickup-operations", h.createOperation)
	api.GET("/pickup-operations/:id/students", h.listOperationStudents)
	api.GET("/pickup-operations/:id/events", h.listEvents)
	api.GET("/pickup-operations/:id/handoffs", h.listHandoffs)
	api.GET("/pickup-operations/:id/handoff-teachers", h.listHandoffTeachers)
	api.POST("/pickup-operations/:id/events/:event_id/correct", h.correctEvent)
	api.GET("/pickup-operations/:id/close-check", h.closeCheck)
	api.GET("/pickup-workbench", h.workbench)
	api.POST("/pickup-operations/:id/confirm", h.confirmOperation)
	api.POST("/pickup-operations/:id/handover", h.handoverOperation)
	api.POST("/pickup-operations/:id/start", h.startOperation)
	api.POST("/pickup-operations/:id/finish", h.finishOperation)
	api.POST("/pickup-operations/:id/students/:student_id/status", h.markOperationStudent)
	api.POST("/pickup-operations/:id/students/:student_id/profile", h.completeOperationStudentProfile)
	api.POST("/pickup-operations/:id/students", h.addOperationStudent)
	api.GET("/pickup-change-requests", h.listChangeRequests)
	api.POST("/pickup-change-requests/:id/review", h.reviewChangeRequest)
	api.GET("/notifications", h.listNotifications)
	api.POST("/notifications/:id/read", h.markNotificationRead)
	api.GET("/notifications/delivery-logs", h.listNotificationDeliveryLogs)
	api.POST("/notifications/:id/retry", h.retryNotification)
	api.POST("/uploads/pickup", h.uploadPickupPhoto)
}

type listResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type operationView struct {
	ID                     uint64  `json:"id"`
	OperationDate          string  `json:"operation_date"`
	PickupMode             string  `json:"pickup_mode"`
	SchoolID               uint64  `json:"school_id"`
	SchoolClassID          uint64  `json:"school_class_id"`
	CareClassID            *uint64 `json:"care_class_id,omitempty"`
	TeacherUserID          *uint64 `json:"teacher_user_id,omitempty"`
	TeacherName            string  `json:"teacher_name"`
	Status                 string  `json:"status"`
	StartedAt              *string `json:"started_at,omitempty"`
	FinishedAt             *string `json:"finished_at,omitempty"`
	ConfirmedAt            *string `json:"confirmed_at,omitempty"`
	ConfirmedByName        string  `json:"confirmed_by_name,omitempty"`
	ExecutingTeacherUserID *uint64 `json:"executing_teacher_user_id,omitempty"`
	ExecutingTeacherName   string  `json:"executing_teacher_name,omitempty"`
	TeacherRole            string  `json:"teacher_role"`
	ExpectedPickupTime     string  `json:"expected_pickup_time,omitempty"`
	Notes                  string  `json:"notes"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
}

type operationStudentView struct {
	ID             uint64  `json:"id"`
	OperationID    uint64  `json:"operation_id"`
	StudentID      uint64  `json:"student_id"`
	StudentName    string  `json:"student_name"`
	Status         string  `json:"status"`
	PhotoURL       string  `json:"photo_url,omitempty"`
	CheckedAt      *string `json:"checked_at,omitempty"`
	Note           string  `json:"note"`
	IsTemporary    bool    `json:"is_temporary"`
	ProfilePending bool    `json:"profile_pending"`
	PickupMode     string  `json:"pickup_mode,omitempty"`
}

type eventView struct {
	ID                 uint64 `json:"id"`
	OperationStudentID uint64 `json:"operation_student_id"`
	StudentID          uint64 `json:"student_id"`
	EventType          string `json:"event_type"`
	EventAt            string `json:"event_at"`
	OperatorName       string `json:"operator_name"`
	PhotoURL           string `json:"photo_url,omitempty"`
	Note               string `json:"note"`
}

type handoffView struct {
	ID                uint64  `json:"id"`
	OperationID       uint64  `json:"operation_id"`
	FromTeacherUserID *uint64 `json:"from_teacher_user_id,omitempty"`
	FromTeacherName   string  `json:"from_teacher_name"`
	ToTeacherUserID   *uint64 `json:"to_teacher_user_id,omitempty"`
	ToTeacherName     string  `json:"to_teacher_name"`
	TeacherRole       string  `json:"teacher_role"`
	Note              string  `json:"note"`
	HandoffAt         string  `json:"handoff_at"`
	CreatedByName     string  `json:"created_by_name"`
}

type handoffTeacherView struct {
	TeacherUserID uint64 `json:"teacher_user_id"`
	TeacherName   string `json:"teacher_name"`
	Username      string `json:"username"`
}

type notificationView struct {
	ID               uint64  `json:"id"`
	StudentID        uint64  `json:"student_id"`
	OperationID      *uint64 `json:"operation_id,omitempty"`
	EventID          *uint64 `json:"event_id,omitempty"`
	RecipientType    string  `json:"recipient_type"`
	Kind             string  `json:"kind"`
	Title            string  `json:"title"`
	Content          string  `json:"content"`
	Status           string  `json:"status"`
	DeliveryAttempts int     `json:"delivery_attempts"`
	LastAttemptAt    *string `json:"last_attempt_at,omitempty"`
	DeliveryError    string  `json:"delivery_error,omitempty"`
	NextRetryAt      *string `json:"next_retry_at,omitempty"`
	ReadAt           *string `json:"read_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

type notificationDeliveryLogView struct {
	ID                 uint64  `json:"id"`
	NotificationID     uint64  `json:"notification_id"`
	StudentID          uint64  `json:"student_id"`
	StudentName        string  `json:"student_name"`
	ParentAccountID    uint64  `json:"parent_account_id"`
	MessageKind        string  `json:"message_kind"`
	TemplateID         string  `json:"template_id"`
	NotificationStatus string  `json:"notification_status"`
	NotificationTitle  string  `json:"notification_title"`
	Status             string  `json:"status"`
	Attempts           int     `json:"attempts"`
	LastAttemptAt      *string `json:"last_attempt_at,omitempty"`
	SentAt             *string `json:"sent_at,omitempty"`
	NextRetryAt        *string `json:"next_retry_at,omitempty"`
	DeliveryError      string  `json:"delivery_error,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type createOperationRequest struct {
	OperationDate string   `json:"operation_date"`
	PickupMode    string   `json:"pickup_mode"`
	SchoolClassID uint64   `json:"school_class_id"`
	CareClassID   *uint64  `json:"care_class_id"`
	TeacherUserID *uint64  `json:"teacher_user_id"`
	TeacherName   string   `json:"teacher_name"`
	StudentIDs    []uint64 `json:"student_ids"`
	Notes         string   `json:"notes"`
}

func (r createOperationRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 3)
	if _, err := parseDate(r.OperationDate); err != nil {
		details = append(details, response.ValidationDetail{Field: "operation_date", Reason: "date_format"})
	}
	if r.SchoolClassID == 0 {
		details = append(details, response.ValidationDetail{Field: "school_class_id", Reason: "required"})
	}
	if r.PickupMode != "" && r.PickupMode != "school_pickup" && r.PickupMode != "self_arrival" {
		details = append(details, response.ValidationDetail{Field: "pickup_mode", Reason: "invalid_value"})
	}
	return details
}

type markOperationStudentRequest struct {
	Status       string `json:"status"`
	PhotoURL     string `json:"photo_url"`
	OperatorName string `json:"operator_name"`
	Note         string `json:"note"`
}

type correctEventRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (r correctEventRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if !IsValidMemberStatus(r.Status) || r.Status == MemberStatusPlanned {
		details = append(details, response.ValidationDetail{Field: "status", Reason: "invalid_value"})
	}
	if strings.TrimSpace(r.Reason) == "" {
		details = append(details, response.ValidationDetail{Field: "reason", Reason: "required"})
	} else if len([]rune(strings.TrimSpace(r.Reason))) > 500 {
		details = append(details, response.ValidationDetail{Field: "reason", Reason: "too_long"})
	}
	return details
}

func (r markOperationStudentRequest) Validate() []response.ValidationDetail {
	if !IsValidMemberStatus(r.Status) || r.Status == MemberStatusPlanned {
		return []response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}
	}
	if r.Status == MemberStatusPickedUp && strings.TrimSpace(r.PhotoURL) == "" {
		return []response.ValidationDetail{{Field: "photo_url", Reason: "required_for_school_pickup"}}
	}
	return nil
}

type confirmOperationRequest struct {
	ExecutingTeacherUserID *uint64 `json:"executing_teacher_user_id"`
	ExecutingTeacherName   string  `json:"executing_teacher_name"`
	TeacherRole            string  `json:"teacher_role"`
	ExpectedPickupTime     string  `json:"expected_pickup_time"`
	Notes                  string  `json:"notes"`
}

type handoverOperationRequest struct {
	ToTeacherUserID uint64 `json:"to_teacher_user_id"`
	ToTeacherName   string `json:"to_teacher_name"`
	TeacherRole     string `json:"teacher_role"`
	Note            string `json:"note"`
}

func (r handoverOperationRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 3)
	if r.ToTeacherUserID == 0 {
		details = append(details, response.ValidationDetail{Field: "to_teacher_user_id", Reason: "required"})
	}
	if r.TeacherRole != "" && r.TeacherRole != "lead" && r.TeacherRole != "collaborator" && r.TeacherRole != "substitute" {
		details = append(details, response.ValidationDetail{Field: "teacher_role", Reason: "invalid_value"})
	}
	if len([]rune(strings.TrimSpace(r.Note))) > 500 {
		details = append(details, response.ValidationDetail{Field: "note", Reason: "too_long"})
	}
	return details
}

func (r confirmOperationRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if r.TeacherRole != "" && r.TeacherRole != "lead" && r.TeacherRole != "collaborator" && r.TeacherRole != "substitute" {
		details = append(details, response.ValidationDetail{Field: "teacher_role", Reason: "invalid_value"})
	}
	if len([]rune(strings.TrimSpace(r.ExpectedPickupTime))) > 16 {
		details = append(details, response.ValidationDetail{Field: "expected_pickup_time", Reason: "too_long"})
	}
	return details
}

type addOperationStudentRequest struct {
	Name          string `json:"name"`
	GuardianPhone string `json:"guardian_phone"`
	Gender        string `json:"gender"`
	StudentNo     string `json:"student_no"`
	PickupMode    string `json:"pickup_mode"`
	Note          string `json:"note"`
}

func (r addOperationStudentRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if strings.TrimSpace(r.Name) == "" {
		details = append(details, response.ValidationDetail{Field: "name", Reason: "required"})
	}
	if r.PickupMode != "" && r.PickupMode != "school_pickup" && r.PickupMode != "self_arrival" && r.PickupMode != "parent_picked_up" {
		details = append(details, response.ValidationDetail{Field: "pickup_mode", Reason: "invalid_value"})
	}
	return details
}

type completeOperationStudentProfileRequest struct {
	GuardianPhone    *string `json:"guardian_phone"`
	Gender           *string `json:"gender"`
	StudentNo        *string `json:"student_no"`
	EmergencyContact *string `json:"emergency_contact"`
	EmergencyPhone   *string `json:"emergency_phone"`
	Notes            *string `json:"notes"`
}

func (r completeOperationStudentProfileRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if r.GuardianPhone == nil && r.Gender == nil && r.StudentNo == nil && r.EmergencyContact == nil && r.EmergencyPhone == nil && r.Notes == nil {
		details = append(details, response.ValidationDetail{Field: "profile", Reason: "required"})
	}
	if r.Gender != nil {
		gender := strings.TrimSpace(*r.Gender)
		if gender != "" && gender != "unknown" && gender != "male" && gender != "female" {
			details = append(details, response.ValidationDetail{Field: "gender", Reason: "invalid_value"})
		}
	}
	for field, value := range map[string]struct {
		value     *string
		maxLength int
	}{
		"guardian_phone":    {r.GuardianPhone, 32},
		"student_no":        {r.StudentNo, 64},
		"emergency_contact": {r.EmergencyContact, 64},
		"emergency_phone":   {r.EmergencyPhone, 32},
		"notes":             {r.Notes, 500},
	} {
		if value.value != nil && len([]rune(strings.TrimSpace(*value.value))) > value.maxLength {
			details = append(details, response.ValidationDetail{Field: field, Reason: "too_long"})
		}
	}
	return details
}

type operationStudentProfileView struct {
	ID               uint64  `json:"id"`
	SchoolID         uint64  `json:"school_id"`
	TermID           uint64  `json:"term_id"`
	SchoolClassID    uint64  `json:"school_class_id"`
	CareClassID      *uint64 `json:"care_class_id,omitempty"`
	Name             string  `json:"name"`
	Gender           string  `json:"gender"`
	StudentNo        string  `json:"student_no"`
	GuardianPhone    string  `json:"guardian_phone"`
	EmergencyContact string  `json:"emergency_contact"`
	EmergencyPhone   string  `json:"emergency_phone"`
	Status           string  `json:"status"`
	Notes            string  `json:"notes"`
	UpdatedAt        string  `json:"updated_at"`
}

type pickupChangeRequestView struct {
	ID              uint64  `json:"id"`
	StudentID       uint64  `json:"student_id"`
	StudentName     string  `json:"student_name"`
	OperationID     *uint64 `json:"operation_id,omitempty"`
	ChangeDate      string  `json:"change_date"`
	RequestedStatus string  `json:"requested_status"`
	Note            string  `json:"note"`
	SubmittedBy     string  `json:"submitted_by"`
	Status          string  `json:"status"`
	ReviewedAt      *string `json:"reviewed_at,omitempty"`
	ReviewNote      string  `json:"review_note"`
	CreatedAt       string  `json:"created_at"`
}

type reviewChangeRequest struct {
	Status     string `json:"status"`
	ReviewNote string `json:"review_note"`
}

func (r reviewChangeRequest) Validate() []response.ValidationDetail {
	if r.Status != ChangeRequestStatusApproved && r.Status != ChangeRequestStatusRejected {
		return []response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}
	}
	return nil
}

func (h *Handler) listOperations(c *gin.Context) {
	items, err := h.store.ListOperations(c.Request.Context(), h.orgID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if date := strings.TrimSpace(c.Query("date")); date != "" {
		parsed, parseErr := parseDate(date)
		if parseErr != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
		filtered := make([]Operation, 0, len(items))
		for _, item := range items {
			if sameDay(item.OperationDate, parsed) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filtered := make([]Operation, 0, len(items))
		for _, item := range items {
			if item.Status == status {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	items, err = h.filterOperationsForPrincipal(c, items)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]operationView, 0, len(items))
	for _, item := range items {
		out = append(out, toOperationView(item))
	}
	response.OK(c, listResponse[operationView]{Items: out, Total: len(out)})
}

func (h *Handler) createOperation(c *gin.Context) {
	if !canWritePickup(c) {
		return
	}
	var req createOperationRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	operationDate, _ := parseDate(req.OperationDate)
	classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), h.orgID)
	if err != nil {
		respondMasterDataError(c, err)
		return
	}
	var selectedClass masterdata.SchoolClass
	for _, item := range classes {
		if item.ID == req.SchoolClassID {
			selectedClass = item
			break
		}
	}
	if selectedClass.ID == 0 {
		response.Error(c, response.NotFound())
		return
	}
	students, err := h.masterData.ListStudents(c.Request.Context(), h.orgID)
	if err != nil {
		respondMasterDataError(c, err)
		return
	}
	selectedIDs := make(map[uint64]struct{}, len(req.StudentIDs))
	for _, id := range req.StudentIDs {
		selectedIDs[id] = struct{}{}
	}
	roster := make([]StudentRef, 0)
	for _, student := range students {
		if student.Status != "active" || student.SchoolClassID != req.SchoolClassID {
			continue
		}
		if req.CareClassID != nil && (student.CareClassID == nil || *student.CareClassID != *req.CareClassID) {
			continue
		}
		if len(selectedIDs) > 0 {
			if _, ok := selectedIDs[student.ID]; !ok {
				continue
			}
		}
		roster = append(roster, StudentRef{ID: student.ID, Name: student.Name})
	}
	if len(roster) == 0 {
		response.Error(c, response.BadRequest("没有可加入接送任务的在托学生", ErrInvalidState))
		return
	}
	teacherUserID, teacherName, accessErr := h.resolveOperationTeacher(c, selectedClass.ID, req.TeacherUserID, req.TeacherName)
	if accessErr != nil {
		respondAccessError(c, accessErr)
		return
	}
	if h.leaveReader != nil {
		approvedLeaveIDs, leaveErr := h.leaveReader.ListApprovedLeaveStudentIDs(c.Request.Context(), h.orgID, operationDate)
		if leaveErr != nil {
			response.Error(c, response.Internal(leaveErr))
			return
		}
		filteredRoster := roster[:0]
		for _, student := range roster {
			if _, onLeave := approvedLeaveIDs[student.ID]; !onLeave {
				filteredRoster = append(filteredRoster, student)
			}
		}
		roster = filteredRoster
		if len(roster) == 0 {
			response.Error(c, response.BadRequest("当天在托学生均已请假，无需创建接送任务", ErrInvalidState))
			return
		}
	}
	item, err := h.store.CreateOperation(c.Request.Context(), h.orgID, CreateOperationParams{OperationDate: operationDate, PickupMode: defaultString(req.PickupMode, "school_pickup"), SchoolID: selectedClass.SchoolID, SchoolClassID: selectedClass.ID, CareClassID: req.CareClassID, TeacherUserID: teacherUserID, TeacherName: teacherName, Notes: strings.TrimSpace(req.Notes)}, roster)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "pickup.operation.create", "pickup_operation", item.ID)
	response.Created(c, "/api/v1/pickup-operations/"+strconv.FormatUint(item.ID, 10), toOperationView(item))
}

func (h *Handler) confirmOperation(c *gin.Context) {
	if !canWritePickup(c) {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	operation, ok := h.operationForPrincipal(c, id)
	if !ok {
		return
	}
	var req confirmOperationRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	teacherID := req.ExecutingTeacherUserID
	teacherName := strings.TrimSpace(req.ExecutingTeacherName)
	if teacherID == nil {
		teacherID = operation.TeacherUserID
	}
	if teacherName == "" {
		teacherName = operation.TeacherName
	}
	resolvedID, resolvedName, resolveErr := h.resolveOperationTeacher(c, operation.SchoolClassID, teacherID, teacherName)
	if resolveErr != nil {
		respondAccessError(c, resolveErr)
		return
	}
	confirmedByID, confirmedByName := h.currentStaff(c)
	if confirmedByID == nil {
		confirmedByID = resolvedID
	}
	item, err := h.store.ConfirmOperation(c.Request.Context(), h.orgID, ConfirmOperationParams{ID: id, ExecutingTeacherUserID: resolvedID, ExecutingTeacherName: resolvedName, TeacherRole: defaultString(req.TeacherRole, "lead"), ExpectedPickupTime: strings.TrimSpace(req.ExpectedPickupTime), ConfirmedByUserID: confirmedByID, ConfirmedByName: confirmedByName, Notes: strings.TrimSpace(req.Notes)})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	students, err := h.store.ListOperationStudents(c.Request.Context(), h.orgID, id)
	if err == nil {
		h.notifyOperationConfirmed(c, item, students)
	}
	h.recordAudit(c, "pickup.operation.confirm", "pickup_operation", item.ID)
	response.OK(c, toOperationView(item))
}

func (h *Handler) handoverOperation(c *gin.Context) {
	if !canWritePickup(c) {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	operation, ok := h.operationForPrincipal(c, id)
	if !ok {
		return
	}
	var req handoverOperationRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if operation.ExecutingTeacherUserID != nil && *operation.ExecutingTeacherUserID == req.ToTeacherUserID {
		response.Error(c, response.BadRequest("交接目标不能是当前执行老师", ErrInvalidState))
		return
	}
	teacherName, resolveErr := h.resolveHandoverTeacher(c, operation.SchoolClassID, req.ToTeacherUserID, req.ToTeacherName)
	if resolveErr != nil {
		respondAccessError(c, resolveErr)
		return
	}
	createdByID, createdByName := h.currentStaff(c)
	item, err := h.store.HandoffOperation(c.Request.Context(), h.orgID, HandoffOperationParams{ID: id, ToTeacherUserID: req.ToTeacherUserID, ToTeacherName: teacherName, TeacherRole: defaultString(req.TeacherRole, "collaborator"), Note: strings.TrimSpace(req.Note), CreatedByUserID: createdByID, CreatedByName: createdByName})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if students, listErr := h.store.ListOperationStudents(c.Request.Context(), h.orgID, id); listErr == nil {
		h.notifyOperationHandover(c, item, students)
	}
	h.recordAudit(c, "pickup.operation.handover", "pickup_operation", id)
	response.OK(c, toOperationView(item))
}

func (h *Handler) resolveHandoverTeacher(c *gin.Context, schoolClassID, teacherUserID uint64, requestedName string) (string, error) {
	if teacherUserID == 0 {
		return "", ErrUnauthorizedOperation
	}
	principal, scoped := h.staffPrincipal(c)
	if scoped && h.assignments != nil {
		if principal.Role == identity.UserRoleTeacher {
			if err := h.requireAssignment(c, principal.SubjectID, schoolClassID); err != nil {
				return "", err
			}
		}
		if err := h.requireAssignment(c, teacherUserID, schoolClassID); err != nil {
			return "", err
		}
	}
	name := strings.TrimSpace(requestedName)
	if h.users != nil {
		user, err := h.users.FindUserByID(c.Request.Context(), teacherUserID)
		if err != nil {
			return "", err
		}
		if user.Role != identity.UserRoleTeacher || user.Status != identity.UserStatusActive {
			return "", ErrUnauthorizedOperation
		}
		name = user.Nickname
	}
	if name == "" {
		name = "老师"
	}
	return name, nil
}

func (h *Handler) notifyOperationHandover(c *gin.Context, operation Operation, students []OperationStudent) {
	if h.store == nil {
		return
	}
	content := "今日接送执行老师已变更为" + strings.TrimSpace(operation.ExecutingTeacherName)
	if strings.TrimSpace(operation.ExpectedPickupTime) != "" {
		content += "，预计" + strings.TrimSpace(operation.ExpectedPickupTime)
	}
	for _, student := range students {
		_, _ = h.store.CreateNotification(c.Request.Context(), h.orgID, CreateNotificationParams{StudentID: student.StudentID, OperationID: &operation.ID, Kind: "pickup_plan_confirmed", Title: "今日接送老师已变更", Content: student.StudentName + "：" + content})
	}
}

func (h *Handler) listHandoffs(c *gin.Context) {
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.operationForPrincipal(c, id); !ok {
		return
	}
	items, err := h.store.ListHandoffs(c.Request.Context(), h.orgID, id)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]handoffView, 0, len(items))
	for _, item := range items {
		out = append(out, handoffView{ID: item.ID, OperationID: item.OperationID, FromTeacherUserID: item.FromTeacherUserID, FromTeacherName: item.FromTeacherName, ToTeacherUserID: item.ToTeacherUserID, ToTeacherName: item.ToTeacherName, TeacherRole: item.TeacherRole, Note: item.Note, HandoffAt: item.HandoffAt.Format(time.RFC3339), CreatedByName: item.CreatedByName})
	}
	response.OK(c, listResponse[handoffView]{Items: out, Total: len(out)})
}

func (h *Handler) listHandoffTeachers(c *gin.Context) {
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	operation, ok := h.operationForPrincipal(c, id)
	if !ok {
		return
	}
	if h.assignments == nil {
		response.OK(c, listResponse[handoffTeacherView]{Items: []handoffTeacherView{}, Total: 0})
		return
	}
	items, err := h.assignments.List(c.Request.Context(), h.orgID, 0, operation.SchoolClassID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]handoffTeacherView, 0, len(items))
	seen := make(map[uint64]struct{}, len(items))
	for _, item := range items {
		if item.Status != assignment.AssignmentStatusActive {
			continue
		}
		if _, exists := seen[item.TeacherUserID]; exists {
			continue
		}
		teacher := handoffTeacherView{TeacherUserID: item.TeacherUserID, TeacherName: "老师"}
		if h.users != nil {
			user, findErr := h.users.FindUserByID(c.Request.Context(), item.TeacherUserID)
			if findErr != nil {
				continue
			}
			if user.Role != identity.UserRoleTeacher || user.Status != identity.UserStatusActive {
				continue
			}
			teacher.TeacherName = strings.TrimSpace(user.Nickname)
			teacher.Username = user.Username
			if teacher.TeacherName == "" {
				teacher.TeacherName = user.Username
			}
		}
		seen[item.TeacherUserID] = struct{}{}
		out = append(out, teacher)
	}
	response.OK(c, listResponse[handoffTeacherView]{Items: out, Total: len(out)})
}

func (h *Handler) currentStaff(c *gin.Context) (*uint64, string) {
	principal, ok := h.staffPrincipal(c)
	if !ok {
		return nil, ""
	}
	name := ""
	if h.users != nil {
		if user, err := h.users.FindUserByID(c.Request.Context(), principal.SubjectID); err == nil {
			name = user.Nickname
		}
	}
	return &principal.SubjectID, name
}

func (h *Handler) notifyOperationConfirmed(c *gin.Context, operation Operation, students []OperationStudent) {
	teacher := strings.TrimSpace(operation.ExecutingTeacherName)
	if teacher == "" {
		teacher = "待安排"
	}
	content := "今日接送老师：" + teacher
	if strings.TrimSpace(operation.ExpectedPickupTime) != "" {
		content += "；预计" + strings.TrimSpace(operation.ExpectedPickupTime) + "开始接送"
	}
	content += "。如接送方式有变化，请及时提交临时变更。"
	for _, student := range students {
		_, _ = h.store.CreateNotification(c.Request.Context(), h.orgID, CreateNotificationParams{StudentID: student.StudentID, OperationID: &operation.ID, Kind: "pickup_plan_confirmed", Title: "今日接送安排已确认", Content: student.StudentName + "：" + content})
	}
}

type workbenchOperationView struct {
	Operation operationView          `json:"operation"`
	Students  []operationStudentView `json:"students"`
	Counts    map[string]int         `json:"counts"`
}

type workbenchAlert struct {
	Kind        string `json:"kind"`
	OperationID uint64 `json:"operation_id"`
	StudentID   uint64 `json:"student_id,omitempty"`
	StudentName string `json:"student_name,omitempty"`
	Message     string `json:"message"`
}

type closeCheckView struct {
	OperationID         uint64                 `json:"operation_id"`
	CanFinish           bool                   `json:"can_finish"`
	Pending             []operationStudentView `json:"pending"`
	Exceptions          []workbenchAlert       `json:"exceptions"`
	PendingPhotoCount   int                    `json:"pending_photo_count"`
	ProfilePendingCount int                    `json:"profile_pending_count"`
}

func (h *Handler) workbench(c *gin.Context) {
	dateValue := strings.TrimSpace(c.Query("date"))
	if dateValue == "" {
		dateValue = time.Now().UTC().Format("2006-01-02")
	}
	date, err := parseDate(dateValue)
	if err != nil {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
		return
	}
	items, err := h.store.ListOperations(c.Request.Context(), h.orgID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	items, err = h.filterOperationsForPrincipal(c, items)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	totals := map[string]int{"tasks": 0, "students": 0, "planned": 0, "picked_up": 0, "arrived": 0, "finished": 0, "abnormal": 0, "pending_photo": 0, "profile_pending": 0}
	operations := make([]workbenchOperationView, 0)
	alerts := make([]workbenchAlert, 0)
	for _, operation := range items {
		if !sameDay(operation.OperationDate, date) {
			continue
		}
		students, listErr := h.store.ListOperationStudents(c.Request.Context(), h.orgID, operation.ID)
		if listErr != nil {
			respondStoreError(c, listErr)
			return
		}
		counts := make(map[string]int)
		views := make([]operationStudentView, 0, len(students))
		for _, student := range students {
			counts[student.Status]++
			totals[student.Status]++
			totals["students"]++
			view := toOperationStudentView(student)
			view.PhotoURL = h.signedPhotoURL(student.PhotoURL)
			views = append(views, view)
			if student.Status == MemberStatusPickedUp && strings.TrimSpace(student.PhotoURL) == "" {
				totals["pending_photo"]++
				alerts = append(alerts, workbenchAlert{Kind: "photo_pending", OperationID: operation.ID, StudentID: student.StudentID, StudentName: student.StudentName, Message: student.StudentName + "已登记接到但照片待补"})
			}
			if student.ProfilePending {
				totals["profile_pending"]++
				alerts = append(alerts, workbenchAlert{Kind: "profile_pending", OperationID: operation.ID, StudentID: student.StudentID, StudentName: student.StudentName, Message: student.StudentName + "是临时学生，档案待补充"})
			}
			if student.Status == MemberStatusNotArrived || student.Status == MemberStatusAbnormal || student.Status == MemberStatusAbsent {
				alerts = append(alerts, workbenchAlert{Kind: "exception", OperationID: operation.ID, StudentID: student.StudentID, StudentName: student.StudentName, Message: student.StudentName + "存在" + pickupStatusLabel(student.Status) + "，请完成收班核对"})
			}
		}
		totals["tasks"]++
		if operation.Status == OperationStatusFinished {
			totals["finished"]++
		}
		if operation.Status == OperationStatusDraft {
			alerts = append(alerts, workbenchAlert{Kind: "task_unconfirmed", OperationID: operation.ID, Message: "接送任务尚未确认，家长还不会收到今日老师安排"})
		}
		operations = append(operations, workbenchOperationView{Operation: toOperationView(operation), Students: views, Counts: counts})
	}
	response.OK(c, gin.H{"date": dateValue, "operations": operations, "totals": totals, "alerts": alerts})
}

func (h *Handler) closeCheck(c *gin.Context) {
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	operation, ok := h.operationForPrincipal(c, id)
	if !ok {
		return
	}
	students, err := h.store.ListOperationStudents(c.Request.Context(), h.orgID, id)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	view := closeCheckView{OperationID: id, CanFinish: operation.Status == OperationStatusStarted}
	for _, student := range students {
		if !IsReadyToFinish(student.Status) {
			view.CanFinish = false
			view.Pending = append(view.Pending, toOperationStudentView(student))
		}
		if student.Status == MemberStatusPickedUp && strings.TrimSpace(student.PhotoURL) == "" {
			view.PendingPhotoCount++
			view.CanFinish = false
		}
		if student.ProfilePending {
			view.ProfilePendingCount++
		}
		if student.Status == MemberStatusNotArrived || student.Status == MemberStatusAbnormal || student.Status == MemberStatusAbsent {
			view.Exceptions = append(view.Exceptions, workbenchAlert{Kind: "exception", OperationID: id, StudentID: student.StudentID, StudentName: student.StudentName, Message: student.StudentName + "：" + pickupStatusLabel(student.Status)})
		}
	}
	response.OK(c, view)
}

func (h *Handler) addOperationStudent(c *gin.Context) {
	if !canWritePickup(c) {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	operation, ok := h.operationForPrincipal(c, id)
	if !ok {
		return
	}
	var req addOperationStudentRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), h.orgID)
	if err != nil {
		respondMasterDataError(c, err)
		return
	}
	var selectedClass masterdata.SchoolClass
	for _, item := range classes {
		if item.ID == operation.SchoolClassID {
			selectedClass = item
			break
		}
	}
	if selectedClass.ID == 0 {
		response.Error(c, response.NotFound())
		return
	}
	student, err := h.masterData.CreateStudent(c.Request.Context(), h.orgID, masterdata.CreateStudentParams{SchoolID: selectedClass.SchoolID, TermID: selectedClass.TermID, SchoolClassID: selectedClass.ID, CareClassID: operation.CareClassID, Name: strings.TrimSpace(req.Name), Gender: defaultString(req.Gender, "unknown"), StudentNo: strings.TrimSpace(req.StudentNo), GuardianPhone: strings.TrimSpace(req.GuardianPhone), Notes: strings.TrimSpace(req.Note)})
	if err != nil {
		respondMasterDataError(c, err)
		return
	}
	item, err := h.store.AddOperationStudent(c.Request.Context(), h.orgID, AddOperationStudentParams{OperationID: id, StudentID: student.ID, StudentName: student.Name, IsTemporary: true, ProfilePending: strings.TrimSpace(req.GuardianPhone) == "", PickupMode: strings.TrimSpace(req.PickupMode), Note: strings.TrimSpace(req.Note)})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "pickup.student.add_temporary", "pickup_operation", id)
	response.Created(c, "/api/v1/pickup-operations/"+strconv.FormatUint(id, 10)+"/students/"+strconv.FormatUint(student.ID, 10), toOperationStudentView(item))
}

func (h *Handler) completeOperationStudentProfile(c *gin.Context) {
	if !canWritePickup(c) {
		return
	}
	operationID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.operationForPrincipal(c, operationID); !ok {
		return
	}
	studentID, ok := parsePathValue(c, "student_id")
	if !ok {
		return
	}
	var req completeOperationStudentProfileRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}

	members, err := h.store.ListOperationStudents(c.Request.Context(), h.orgID, operationID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	var operationStudent *OperationStudent
	for index := range members {
		if members[index].StudentID == studentID {
			operationStudent = &members[index]
			break
		}
	}
	if operationStudent == nil {
		response.Error(c, response.NotFound())
		return
	}
	if !operationStudent.IsTemporary {
		response.Error(c, response.BadRequest("只有临时学生可以补充临时档案", ErrInvalidState))
		return
	}

	student, err := h.masterData.FindStudent(c.Request.Context(), h.orgID, studentID)
	if err != nil {
		respondMasterDataError(c, err)
		return
	}
	params := masterdata.UpdateStudentParams{
		ID:               student.ID,
		SchoolID:         student.SchoolID,
		TermID:           student.TermID,
		SchoolClassID:    student.SchoolClassID,
		CareClassID:      student.CareClassID,
		Name:             student.Name,
		Gender:           student.Gender,
		BirthDate:        student.BirthDate,
		StudentNo:        student.StudentNo,
		GuardianPhone:    student.GuardianPhone,
		EmergencyContact: student.EmergencyContact,
		EmergencyPhone:   student.EmergencyPhone,
		Status:           student.Status,
		Notes:            student.Notes,
	}
	if req.GuardianPhone != nil {
		params.GuardianPhone = strings.TrimSpace(*req.GuardianPhone)
	}
	if req.Gender != nil {
		params.Gender = defaultString(*req.Gender, "unknown")
	}
	if req.StudentNo != nil {
		params.StudentNo = strings.TrimSpace(*req.StudentNo)
	}
	if req.EmergencyContact != nil {
		params.EmergencyContact = strings.TrimSpace(*req.EmergencyContact)
	}
	if req.EmergencyPhone != nil {
		params.EmergencyPhone = strings.TrimSpace(*req.EmergencyPhone)
	}
	if req.Notes != nil {
		params.Notes = strings.TrimSpace(*req.Notes)
	}
	if operationStudent.ProfilePending && strings.TrimSpace(params.GuardianPhone) == "" {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "guardian_phone", Reason: "required"}}))
		return
	}
	updatedStudent, err := h.masterData.UpdateStudent(c.Request.Context(), h.orgID, params)
	if err != nil {
		respondMasterDataError(c, err)
		return
	}
	if operationStudent.ProfilePending {
		if err := h.store.CompleteOperationStudentProfile(c.Request.Context(), h.orgID, operationID, studentID); err != nil {
			respondStoreError(c, err)
			return
		}
	}
	updatedMembers, err := h.store.ListOperationStudents(c.Request.Context(), h.orgID, operationID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	operationStudent = nil
	for index := range updatedMembers {
		if updatedMembers[index].StudentID == studentID {
			operationStudent = &updatedMembers[index]
			break
		}
	}
	if operationStudent == nil {
		response.Error(c, response.NotFound())
		return
	}
	h.recordAudit(c, "pickup.student.complete_profile", "student", studentID)
	response.OK(c, gin.H{
		"student":           toOperationStudentProfileView(updatedStudent),
		"operation_student": toOperationStudentView(*operationStudent),
	})
}

func (h *Handler) listOperationStudents(c *gin.Context) {
	operationID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.operationForPrincipal(c, operationID); !ok {
		return
	}
	items, err := h.store.ListOperationStudents(c.Request.Context(), h.orgID, operationID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]operationStudentView, 0, len(items))
	for _, item := range items {
		view := toOperationStudentView(item)
		view.PhotoURL = h.signedPhotoURL(item.PhotoURL)
		out = append(out, view)
	}
	response.OK(c, listResponse[operationStudentView]{Items: out, Total: len(out)})
}

func (h *Handler) startOperation(c *gin.Context)  { h.setOperationStatus(c, OperationStatusStarted) }
func (h *Handler) finishOperation(c *gin.Context) { h.setOperationStatus(c, OperationStatusFinished) }

func (h *Handler) setOperationStatus(c *gin.Context, status string) {
	if !canWritePickup(c) {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	operation, ok := h.operationForPrincipal(c, id)
	if !ok {
		return
	}
	item, err := h.store.SetOperationStatus(c.Request.Context(), h.orgID, SetOperationStatusParams{ID: id, Status: status})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if status == OperationStatusStarted {
		if err := h.applyApprovedPickupChanges(c, operation); err != nil {
			respondStoreError(c, err)
			return
		}
	}
	h.recordAudit(c, "pickup.operation."+status, "pickup_operation", item.ID)
	response.OK(c, toOperationView(item))
}

func (h *Handler) applyApprovedPickupChanges(c *gin.Context, operation Operation) error {
	changes, err := h.store.ListPickupChangeRequests(c.Request.Context(), h.orgID, &operation.OperationDate, ChangeRequestStatusApproved)
	if err != nil {
		return err
	}
	latestByStudent := make(map[uint64]PickupChangeRequest)
	for _, change := range changes {
		if change.OperationID == nil || *change.OperationID != operation.ID {
			continue
		}
		if _, exists := latestByStudent[change.StudentID]; exists {
			continue
		}
		latestByStudent[change.StudentID] = change
	}
	for _, change := range latestByStudent {
		if _, err := h.store.MarkOperationStudent(c.Request.Context(), h.orgID, MarkStudentParams{OperationID: operation.ID, StudentID: change.StudentID, Status: change.RequestedStatus, OperatorName: "老师", Note: change.Note}); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) markOperationStudent(c *gin.Context) {
	if !canWritePickup(c) {
		return
	}
	operationID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.operationForPrincipal(c, operationID); !ok {
		return
	}
	studentID, ok := parsePathValue(c, "student_id")
	if !ok {
		return
	}
	var req markOperationStudentRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.store.MarkOperationStudent(c.Request.Context(), h.orgID, MarkStudentParams{OperationID: operationID, StudentID: studentID, Status: req.Status, PhotoURL: strings.TrimSpace(req.PhotoURL), OperatorName: strings.TrimSpace(req.OperatorName), Note: strings.TrimSpace(req.Note)})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "pickup.student.status", "pickup_operation_student", item.ID)
	response.OK(c, toOperationStudentView(item))
}

func (h *Handler) listEvents(c *gin.Context) {
	operationID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.operationForPrincipal(c, operationID); !ok {
		return
	}
	items, err := h.store.ListEvents(c.Request.Context(), h.orgID, operationID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]eventView, 0, len(items))
	for _, item := range items {
		out = append(out, eventView{ID: item.ID, OperationStudentID: item.OperationStudentID, StudentID: item.StudentID, EventType: item.EventType, EventAt: item.EventAt.Format(time.RFC3339), OperatorName: item.OperatorName, PhotoURL: h.signedPhotoURL(item.PhotoURL), Note: item.Note})
	}
	response.OK(c, listResponse[eventView]{Items: out, Total: len(out)})
}

func (h *Handler) correctEvent(c *gin.Context) {
	if !canWritePickup(c) {
		return
	}
	operationID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.operationForPrincipal(c, operationID); !ok {
		return
	}
	eventID, ok := parsePathID(c, "event_id")
	if !ok {
		return
	}
	var req correctEventRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	_, operatorName := h.currentStaff(c)
	item, err := h.store.CorrectOperationEvent(c.Request.Context(), h.orgID, CorrectEventParams{OperationID: operationID, EventID: eventID, Status: strings.TrimSpace(req.Status), OperatorName: operatorName, Reason: strings.TrimSpace(req.Reason)})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "pickup.event.correct", "pickup_event", eventID)
	response.OK(c, toOperationStudentView(item))
}

func (h *Handler) listNotifications(c *gin.Context) {
	items, err := h.store.ListNotifications(c.Request.Context(), h.orgID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	items, err = h.filterNotificationsForPrincipal(c, items)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]notificationView, 0, len(items))
	for _, item := range items {
		out = append(out, notificationView{ID: item.ID, StudentID: item.StudentID, OperationID: item.OperationID, EventID: item.EventID, RecipientType: item.RecipientType, Kind: item.Kind, Title: item.Title, Content: item.Content, Status: item.Status, DeliveryAttempts: item.DeliveryAttempts, LastAttemptAt: formatTime(item.LastAttemptAt), DeliveryError: item.DeliveryError, NextRetryAt: formatTime(item.NextRetryAt), ReadAt: formatTime(item.ReadAt), CreatedAt: item.CreatedAt.Format(time.RFC3339)})
	}
	response.OK(c, listResponse[notificationView]{Items: out, Total: len(out)})
}

func (h *Handler) listNotificationDeliveryLogs(c *gin.Context) {
	principal, scoped := h.staffPrincipal(c)
	if scoped && principal.Role != identity.UserRoleAdmin && principal.Role != identity.UserRoleEditor {
		response.Error(c, response.Forbidden())
		return
	}
	var notificationID *uint64
	if value := strings.TrimSpace(c.Query("notification_id")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "notification_id", Reason: "invalid_value"}}))
			return
		}
		notificationID = &parsed
	}
	messageKind := strings.TrimSpace(c.Query("message_kind"))
	if messageKind != "" && !isNotificationMessageKind(messageKind) {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "message_kind", Reason: "invalid_value"}}))
		return
	}
	items, err := h.store.ListNotificationDeliveryLogs(c.Request.Context(), h.orgID, notificationID, strings.TrimSpace(c.Query("status")))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if messageKind != "" {
		filtered := items[:0]
		for _, item := range items {
			if item.MessageKind == messageKind {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	notifications, err := h.store.ListNotifications(c.Request.Context(), h.orgID)
	if err != nil {
		respondStoreError(c, err)
		return
	}
	notificationByID := make(map[uint64]Notification, len(notifications))
	for _, notification := range notifications {
		notificationByID[notification.ID] = notification
	}
	studentNames := make(map[uint64]string)
	if h.masterData != nil {
		students, studentErr := h.masterData.ListStudents(c.Request.Context(), h.orgID)
		if studentErr != nil {
			respondStoreError(c, studentErr)
			return
		}
		for _, student := range students {
			studentNames[student.ID] = student.Name
		}
	}
	out := make([]notificationDeliveryLogView, 0, len(items))
	for _, item := range items {
		notification := notificationByID[item.NotificationID]
		out = append(out, notificationDeliveryLogView{ID: item.ID, NotificationID: item.NotificationID, StudentID: notification.StudentID, StudentName: studentNames[notification.StudentID], ParentAccountID: item.ParentAccountID, MessageKind: item.MessageKind, TemplateID: item.TemplateID, NotificationStatus: notification.Status, NotificationTitle: notification.Title, Status: item.Status, Attempts: item.Attempts, LastAttemptAt: formatTime(item.LastAttemptAt), SentAt: formatTime(item.SentAt), NextRetryAt: formatTime(item.NextRetryAt), DeliveryError: item.DeliveryError, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)})
	}
	response.OK(c, listResponse[notificationDeliveryLogView]{Items: out, Total: len(out)})
}

func isNotificationMessageKind(value string) bool {
	switch value {
	case "pickup", "meal", "homework", "leave", "summary":
		return true
	default:
		return false
	}
}

func (h *Handler) retryNotification(c *gin.Context) {
	principal, scoped := h.staffPrincipal(c)
	if scoped && principal.Role != identity.UserRoleAdmin && principal.Role != identity.UserRoleEditor {
		response.Error(c, response.Forbidden())
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if err := h.store.RetryNotification(c.Request.Context(), h.orgID, id); err != nil {
		respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "notification.retry", "notification", id)
	response.OK(c, gin.H{"id": id, "status": "pending"})
}

func (h *Handler) markNotificationRead(c *gin.Context) {
	if !canReadNotification(c) {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if err := h.store.MarkNotificationRead(c.Request.Context(), h.orgID, id); err != nil {
		respondStoreError(c, err)
		return
	}
	response.OK(c, gin.H{"id": id, "read": true})
}

func canReadNotification(c *gin.Context) bool {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok {
		return true
	}
	if principal.Kind != identity.PrincipalKindUser && principal.Kind != identity.PrincipalKindParent {
		response.Error(c, response.Forbidden())
		return false
	}
	return true
}

func (h *Handler) listChangeRequests(c *gin.Context) {
	date, err := optionalDate(c.Query("date"))
	if err != nil {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
		return
	}
	items, err := h.store.ListPickupChangeRequests(c.Request.Context(), h.orgID, date, strings.TrimSpace(c.Query("status")))
	if err != nil {
		respondStoreError(c, err)
		return
	}
	out := make([]pickupChangeRequestView, 0, len(items))
	for _, item := range items {
		if item.OperationID != nil {
			if _, allowed := h.operationForPrincipal(c, *item.OperationID); !allowed {
				continue
			}
		} else if !h.changeRequestStudentAllowed(c, item.StudentID) {
			continue
		}
		out = append(out, toPickupChangeRequestView(item))
	}
	response.OK(c, listResponse[pickupChangeRequestView]{Items: out, Total: len(out)})
}

func (h *Handler) reviewChangeRequest(c *gin.Context) {
	if !canWritePickup(c) {
		return
	}
	id, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	var req reviewChangeRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	existingItems, err := h.store.ListPickupChangeRequests(c.Request.Context(), h.orgID, nil, "")
	if err != nil {
		respondStoreError(c, err)
		return
	}
	var existing *PickupChangeRequest
	for index := range existingItems {
		if existingItems[index].ID == id {
			existing = &existingItems[index]
			break
		}
	}
	if existing == nil {
		response.Error(c, response.NotFound())
		return
	}
	if existing.OperationID != nil {
		if _, allowed := h.operationForPrincipal(c, *existing.OperationID); !allowed {
			return
		}
	} else if !h.changeRequestStudentAllowed(c, existing.StudentID) {
		return
	}
	reviewerID, _ := h.currentStaff(c)
	item, err := h.store.ReviewPickupChangeRequest(c.Request.Context(), h.orgID, ReviewPickupChangeRequestParams{ID: id, Status: req.Status, ReviewedByUserID: reviewerID, ReviewNote: strings.TrimSpace(req.ReviewNote)})
	if err != nil {
		respondStoreError(c, err)
		return
	}
	if item.Status == ChangeRequestStatusApproved && item.OperationID != nil {
		operation, findErr := h.store.FindOperation(c.Request.Context(), h.orgID, *item.OperationID)
		if findErr == nil && operation.Status == OperationStatusStarted {
			_, _ = h.store.MarkOperationStudent(c.Request.Context(), h.orgID, MarkStudentParams{OperationID: *item.OperationID, StudentID: item.StudentID, Status: item.RequestedStatus, OperatorName: "老师", Note: item.Note})
		}
	}
	statusText := "已同意"
	if item.Status == ChangeRequestStatusRejected {
		statusText = "未同意"
	}
	content := item.ChangeDate.Format("2006-01-02") + "：临时接送变更" + statusText
	if strings.TrimSpace(item.ReviewNote) != "" {
		content += "；老师备注：" + strings.TrimSpace(item.ReviewNote)
	}
	_, _ = h.store.CreateNotification(c.Request.Context(), h.orgID, CreateNotificationParams{StudentID: item.StudentID, OperationID: item.OperationID, Kind: "pickup_change_review", Title: "临时接送变更" + statusText, Content: content})
	h.recordAudit(c, "pickup.change_request.review", "pickup_change_request", item.ID)
	response.OK(c, toPickupChangeRequestView(item))
}

func (h *Handler) changeRequestStudentAllowed(c *gin.Context, studentID uint64) bool {
	principal, scoped := h.staffPrincipal(c)
	if !scoped || principal.Role != identity.UserRoleTeacher || h.assignments == nil {
		return true
	}
	student, err := h.masterData.FindStudent(c.Request.Context(), h.orgID, studentID)
	if err != nil {
		return false
	}
	return h.requireAssignment(c, principal.SubjectID, student.SchoolClassID) == nil
}

func (h *Handler) uploadPickupPhoto(c *gin.Context) {
	if !canWritePickup(c) {
		return
	}
	if h.photos == nil {
		response.Error(c, response.Internal(errors.New("photo storage is not configured")))
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, response.BadRequest("请上传照片文件", err))
		return
	}
	file, err := header.Open()
	if err != nil {
		response.Error(c, response.BadRequest("无法读取照片文件", err))
		return
	}
	defer func() { _ = file.Close() }()
	asset, err := savePickupPhoto(c, h.photos, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrFileTooLarge):
			response.Error(c, response.PayloadTooLarge(err))
		case errors.Is(err, storage.ErrUnsupportedContentType):
			response.Error(c, response.UnsupportedMediaTypeMessage("仅支持 JPEG、PNG 或 WebP 图片", err))
		default:
			response.Error(c, response.BadRequest("照片上传失败", err))
		}
		return
	}
	if err := h.recordAsset(c, asset, "pickup_photo", formUint64(c, "operation_id")); err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	response.Created(c, asset.URL, gin.H{"url": asset.URL, "key": asset.Key, "sha256": asset.SHA256, "content_type": asset.ContentType, "size": asset.Size})
}

type pickupCategorizedPhotoStore interface {
	SaveIn(context.Context, string, string, string, io.Reader) (storage.Asset, error)
}

func savePickupPhoto(c *gin.Context, photos storage.Store, filename, contentType string, reader io.Reader) (storage.Asset, error) {
	if store, ok := photos.(pickupCategorizedPhotoStore); ok {
		return store.SaveIn(c.Request.Context(), "pickup", filename, contentType, reader)
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

func (h *Handler) filterOperationsForPrincipal(c *gin.Context, items []Operation) ([]Operation, error) {
	principal, scoped := h.staffPrincipal(c)
	if !scoped || principal.Role != identity.UserRoleTeacher || h.assignments == nil {
		return items, nil
	}
	assigned, err := h.assignments.List(c.Request.Context(), h.orgID, principal.SubjectID, 0)
	if err != nil {
		return nil, err
	}
	classes := make(map[uint64]struct{}, len(assigned))
	for _, item := range assigned {
		if item.Status == assignment.AssignmentStatusActive {
			classes[item.SchoolClassID] = struct{}{}
		}
	}
	filtered := make([]Operation, 0, len(items))
	for _, item := range items {
		if _, ok := classes[item.SchoolClassID]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (h *Handler) filterNotificationsForPrincipal(c *gin.Context, items []Notification) ([]Notification, error) {
	principal, scoped := h.staffPrincipal(c)
	if !scoped || principal.Role != identity.UserRoleTeacher || h.assignments == nil {
		return items, nil
	}
	operations, err := h.store.ListOperations(c.Request.Context(), h.orgID)
	if err != nil {
		return nil, err
	}
	operations, err = h.filterOperationsForPrincipal(c, operations)
	if err != nil {
		return nil, err
	}
	allowed := make(map[uint64]struct{}, len(operations))
	for _, item := range operations {
		allowed[item.ID] = struct{}{}
	}
	// Meal, homework, leave and daily-summary notifications are not tied to a
	// pickup operation. Build the same class scope from student records so the
	// teacher notification center does not silently hide those daily messages.
	allowedStudents := make(map[uint64]struct{})
	if h.masterData != nil {
		assigned, assignmentErr := h.assignments.List(c.Request.Context(), h.orgID, principal.SubjectID, 0)
		if assignmentErr != nil {
			return nil, assignmentErr
		}
		assignedClasses := make(map[uint64]struct{}, len(assigned))
		for _, item := range assigned {
			if item.Status == assignment.AssignmentStatusActive {
				assignedClasses[item.SchoolClassID] = struct{}{}
			}
		}
		students, studentErr := h.masterData.ListStudents(c.Request.Context(), h.orgID)
		if studentErr != nil {
			return nil, studentErr
		}
		for _, student := range students {
			if student.Status != "active" {
				continue
			}
			if _, ok := assignedClasses[student.SchoolClassID]; ok {
				allowedStudents[student.ID] = struct{}{}
			}
		}
	}
	filtered := make([]Notification, 0, len(items))
	for _, item := range items {
		if item.OperationID != nil {
			if _, ok := allowed[*item.OperationID]; ok {
				filtered = append(filtered, item)
			}
			continue
		}
		if _, ok := allowedStudents[item.StudentID]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (h *Handler) operationForPrincipal(c *gin.Context, operationID uint64) (Operation, bool) {
	item, err := h.store.FindOperation(c.Request.Context(), h.orgID, operationID)
	if err != nil {
		respondStoreError(c, err)
		return Operation{}, false
	}
	if !h.operationAllowed(c, item) {
		respondAccessError(c, ErrUnauthorizedOperation)
		return Operation{}, false
	}
	return item, true
}

func (h *Handler) operationAllowed(c *gin.Context, operation Operation) bool {
	principal, scoped := h.staffPrincipal(c)
	if !scoped || h.assignments == nil || principal.Role != identity.UserRoleTeacher {
		return true
	}
	assigned, err := h.assignments.FindByPair(c.Request.Context(), h.orgID, principal.SubjectID, operation.SchoolClassID)
	return err == nil && assigned.Status == assignment.AssignmentStatusActive
}

func (h *Handler) resolveOperationTeacher(c *gin.Context, schoolClassID uint64, requestedID *uint64, requestedName string) (*uint64, string, error) {
	principal, scoped := h.staffPrincipal(c)
	if !scoped || h.assignments == nil {
		return requestedID, strings.TrimSpace(requestedName), nil
	}
	if principal.Role == identity.UserRoleTeacher {
		if err := h.requireAssignment(c, principal.SubjectID, schoolClassID); err != nil {
			return nil, "", err
		}
		name := strings.TrimSpace(requestedName)
		if h.users != nil {
			user, err := h.users.FindUserByID(c.Request.Context(), principal.SubjectID)
			if err != nil {
				return nil, "", err
			}
			if user.Role != identity.UserRoleTeacher || user.Status != identity.UserStatusActive {
				return nil, "", ErrUnauthorizedOperation
			}
			name = user.Nickname
		}
		teacherID := principal.SubjectID
		return &teacherID, name, nil
	}
	if requestedID == nil {
		return nil, strings.TrimSpace(requestedName), nil
	}
	if err := h.requireAssignment(c, *requestedID, schoolClassID); err != nil {
		return nil, "", err
	}
	name := strings.TrimSpace(requestedName)
	if h.users != nil {
		user, err := h.users.FindUserByID(c.Request.Context(), *requestedID)
		if err != nil {
			return nil, "", err
		}
		if user.Role != identity.UserRoleTeacher || user.Status != identity.UserStatusActive {
			return nil, "", errors.New("pickup: requested user is not an active teacher")
		}
		name = user.Nickname
	}
	return requestedID, name, nil
}

func (h *Handler) requireAssignment(c *gin.Context, teacherUserID, schoolClassID uint64) error {
	item, err := h.assignments.FindByPair(c.Request.Context(), h.orgID, teacherUserID, schoolClassID)
	if err != nil {
		if errors.Is(err, assignment.ErrNotFound) {
			return ErrUnauthorizedOperation
		}
		return err
	}
	if item.Status != assignment.AssignmentStatusActive {
		return ErrUnauthorizedOperation
	}
	return nil
}

func (h *Handler) staffPrincipal(c *gin.Context) (identity.Principal, bool) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser {
		return identity.Principal{}, false
	}
	return principal, true
}

func parsePathID(c *gin.Context, key string) (uint64, bool) { return parsePathValue(c, key) }
func parsePathValue(c *gin.Context, key string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || value == 0 {
		response.Error(c, response.BadRequest(fmt.Sprintf("%s 不合法", key), err))
		return 0, false
	}
	return value, true
}
func parseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.UTC)
}
func optionalDate(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := parseDate(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
func pickupStatusLabel(status string) string {
	return map[string]string{
		MemberStatusAbsent:         "未到",
		MemberStatusAbnormal:       "异常",
		MemberStatusNotArrived:     "未到班",
		MemberStatusPickedUp:       "校门口接到",
		MemberStatusSelfArrived:    "自行到班",
		MemberStatusParentPickedUp: "家长接走",
		MemberStatusLeave:          "请假",
		MemberStatusArrived:        "已到班",
		MemberStatusLeft:           "已离班",
		MemberStatusMidwayLeft:     "中途离班",
	}[status]
}
func toOperationView(item Operation) operationView {
	teacherName := item.ExecutingTeacherName
	if strings.TrimSpace(teacherName) == "" {
		teacherName = item.TeacherName
	}
	return operationView{ID: item.ID, OperationDate: item.OperationDate.Format("2006-01-02"), PickupMode: item.PickupMode, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, CareClassID: item.CareClassID, TeacherUserID: item.TeacherUserID, TeacherName: teacherName, Status: item.Status, StartedAt: formatTime(item.StartedAt), FinishedAt: formatTime(item.FinishedAt), ConfirmedAt: formatTime(item.ConfirmedAt), ConfirmedByName: item.ConfirmedByName, ExecutingTeacherUserID: item.ExecutingTeacherUserID, ExecutingTeacherName: item.ExecutingTeacherName, TeacherRole: item.TeacherRole, ExpectedPickupTime: item.ExpectedPickupTime, Notes: item.Notes, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}
func toOperationStudentView(item OperationStudent) operationStudentView {
	return operationStudentView{ID: item.ID, OperationID: item.OperationID, StudentID: item.StudentID, StudentName: item.StudentName, Status: item.Status, PhotoURL: item.PhotoURL, CheckedAt: formatTime(item.CheckedAt), Note: item.Note, IsTemporary: item.IsTemporary, ProfilePending: item.ProfilePending, PickupMode: item.PickupMode}
}
func toOperationStudentProfileView(item masterdata.Student) operationStudentProfileView {
	return operationStudentProfileView{ID: item.ID, SchoolID: item.SchoolID, TermID: item.TermID, SchoolClassID: item.SchoolClassID, CareClassID: item.CareClassID, Name: item.Name, Gender: item.Gender, StudentNo: item.StudentNo, GuardianPhone: item.GuardianPhone, EmergencyContact: item.EmergencyContact, EmergencyPhone: item.EmergencyPhone, Status: item.Status, Notes: item.Notes, UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}
func toPickupChangeRequestView(item PickupChangeRequest) pickupChangeRequestView {
	return pickupChangeRequestView{ID: item.ID, StudentID: item.StudentID, StudentName: item.StudentName, OperationID: item.OperationID, ChangeDate: item.ChangeDate.Format("2006-01-02"), RequestedStatus: item.RequestedStatus, Note: item.Note, SubmittedBy: item.SubmittedBy, Status: item.Status, ReviewedAt: formatTime(item.ReviewedAt), ReviewNote: item.ReviewNote, CreatedAt: item.CreatedAt.Format(time.RFC3339)}
}
func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func (h *Handler) signedPhotoURL(value string) string {
	if h.photoSigner == nil || strings.TrimSpace(value) == "" {
		return value
	}
	return h.photoSigner.Sign(value, 15*time.Minute)
}

func canWritePickup(c *gin.Context) bool {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok {
		return true
	}
	if principal.Kind != identity.PrincipalKindUser || principal.Role == identity.UserRoleViewer {
		response.Error(c, response.Forbidden())
		return false
	}
	return true
}
func respondStoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnauthorizedOperation):
		respondAccessError(c, err)
	case errors.Is(err, ErrConflict):
		response.Error(c, response.BadRequest("今天该学校班级已经创建过接送任务", err))
	case errors.Is(err, ErrInvalidState):
		response.Error(c, response.BadRequest("当前接送任务状态不允许此操作", err))
	case errors.Is(err, ErrInvalidStatus):
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}))
	case errors.Is(err, ErrNotFound):
		response.Error(c, response.NotFound())
	default:
		response.Error(c, response.Internal(err))
	}
}

func respondAccessError(c *gin.Context, err error) {
	response.Error(c, response.BadRequest("当前教师没有负责该学校班级", err))
}
func respondMasterDataError(c *gin.Context, err error) {
	if errors.Is(err, masterdata.ErrNotFound) {
		response.Error(c, response.NotFound())
		return
	}
	response.Error(c, response.Internal(err))
}
