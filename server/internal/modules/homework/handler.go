package homework

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
	assignments        assignment.Store
	users              identity.UserStore
	parents            parent.Store
	notifications      pickup.NotificationWriter
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

func (h *Handler) SetStaffScope(assignments assignment.Store, users identity.UserStore) {
	h.assignments = assignments
	h.users = users
}

func (h *Handler) SetParentStore(parents parent.Store) { h.parents = parents }

func (h *Handler) SetNotificationWriter(notifications pickup.NotificationWriter) {
	h.notifications = notifications
}

func (h *Handler) SetPhotoStore(photos storage.Store) { h.photos = photos }

func (h *Handler) SetAssetStore(store mediamodule.Store) { h.assets = store }

func (h *Handler) SetAssetRetentionDays(days int) { h.assetRetentionDays = days }

func (h *Handler) SetPhotoSigner(signer *storage.URLSigner) { h.photoSigner = signer }

func (h *Handler) SetAuditWriter(writer auditmodule.Writer) { h.audit = writer }

func (h *Handler) recordAudit(c *gin.Context, action, resourceType string, resourceID uint64) {
	auditmodule.RecordForContext(c.Request.Context(), h.audit, h.orgID, action, resourceType, &resourceID, "{}", c.GetHeader("X-Request-ID"))
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	h.RegisterStaffRoutes(api)
	h.RegisterParentRoutes(api)
}

func (h *Handler) RegisterStaffRoutes(api *gin.RouterGroup) {
	api.GET("/homework-tasks", h.listTasks)
	api.POST("/homework-tasks", h.createTask)
	api.POST("/uploads/homework", h.uploadHomeworkPhoto)
	api.GET("/homework-tasks/:id/students", h.listTaskStudents)
	api.POST("/homework-tasks/:id/students/:student_id/review", h.reviewStudent)
}

func (h *Handler) RegisterParentRoutes(api *gin.RouterGroup) {
	api.GET("/parent/students/:student_id/homework", h.listParentHomework)
}

type taskView struct {
	ID              uint64   `json:"id"`
	HomeworkDate    string   `json:"homework_date"`
	SchoolID        uint64   `json:"school_id"`
	SchoolClassID   uint64   `json:"school_class_id"`
	Subject         string   `json:"subject"`
	Content         string   `json:"content"`
	AttachmentURLs  []string `json:"attachment_urls"`
	CreatedByUserID *uint64  `json:"created_by_user_id,omitempty"`
	CreatorName     string   `json:"creator_name"`
	Status          string   `json:"status"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type taskStudentView struct {
	ID               uint64  `json:"id"`
	TaskID           uint64  `json:"task_id"`
	StudentID        uint64  `json:"student_id"`
	StudentName      string  `json:"student_name"`
	Status           string  `json:"status"`
	CorrectionNote   string  `json:"correction_note"`
	ReviewedByUserID *uint64 `json:"reviewed_by_user_id,omitempty"`
	ReviewedAt       *string `json:"reviewed_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type parentHomeworkView struct {
	TaskID         uint64   `json:"task_id"`
	HomeworkDate   string   `json:"homework_date"`
	SchoolID       uint64   `json:"school_id"`
	SchoolClassID  uint64   `json:"school_class_id"`
	StudentID      uint64   `json:"student_id"`
	StudentName    string   `json:"student_name"`
	Subject        string   `json:"subject"`
	Content        string   `json:"content"`
	Status         string   `json:"status"`
	CorrectionNote string   `json:"correction_note"`
	CreatorName    string   `json:"creator_name"`
	AttachmentURLs []string `json:"attachment_urls"`
	ReviewedAt     *string  `json:"reviewed_at,omitempty"`
	CreatedAt      string   `json:"created_at"`
}

type createTaskRequest struct {
	HomeworkDate   string   `json:"homework_date"`
	SchoolClassID  uint64   `json:"school_class_id"`
	Subject        string   `json:"subject"`
	Content        string   `json:"content"`
	AttachmentURLs []string `json:"attachment_urls"`
	StudentIDs     []uint64 `json:"student_ids"`
}

func (r createTaskRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 3)
	if _, err := parseDate(r.HomeworkDate); err != nil {
		details = append(details, response.ValidationDetail{Field: "homework_date", Reason: "date_format"})
	}
	if r.SchoolClassID == 0 {
		details = append(details, response.ValidationDetail{Field: "school_class_id", Reason: "required"})
	}
	if strings.TrimSpace(r.Content) == "" {
		details = append(details, response.ValidationDetail{Field: "content", Reason: "required"})
	}
	if len(r.AttachmentURLs) > 9 {
		details = append(details, response.ValidationDetail{Field: "attachment_urls", Reason: "too_many"})
	}
	for _, value := range r.AttachmentURLs {
		value = strings.TrimSpace(value)
		if value == "" || !strings.HasPrefix(value, "/uploads/") || len(value) > 512 {
			details = append(details, response.ValidationDetail{Field: "attachment_urls", Reason: "invalid_value"})
			break
		}
	}
	return details
}

type reviewStudentRequest struct {
	Status         string `json:"status"`
	CorrectionNote string `json:"correction_note"`
}

func (r reviewStudentRequest) Validate() []response.ValidationDetail {
	if !validStudentStatus(r.Status) {
		return []response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}
	}
	return nil
}

func (h *Handler) listTasks(c *gin.Context) {
	items, err := h.store.ListTasks(c.Request.Context(), h.orgID)
	if err != nil {
		respondError(c, err)
		return
	}
	items, err = h.filterTasksForPrincipal(c, items)
	if err != nil {
		respondError(c, err)
		return
	}
	if date := strings.TrimSpace(c.Query("date")); date != "" {
		parsed, parseErr := parseDate(date)
		if parseErr != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
		filtered := make([]Task, 0, len(items))
		for _, item := range items {
			if sameDay(item.HomeworkDate, parsed) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	if classID := strings.TrimSpace(c.Query("school_class_id")); classID != "" {
		parsed, parseErr := strconv.ParseUint(classID, 10, 64)
		if parseErr != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "school_class_id", Reason: "invalid_value"}}))
			return
		}
		filtered := make([]Task, 0, len(items))
		for _, item := range items {
			if item.SchoolClassID == parsed {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filtered := make([]Task, 0, len(items))
		for _, item := range items {
			if item.Status == status {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	out := make([]taskView, 0, len(items))
	for _, item := range items {
		out = append(out, toTaskView(item))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func (h *Handler) createTask(c *gin.Context) {
	if !canWriteHomework(c) {
		return
	}
	var req createTaskRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	homeworkDate, _ := parseDate(req.HomeworkDate)
	classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), h.orgID)
	if err != nil {
		respondMasterDataError(c, err)
		return
	}
	var selectedClass masterdata.SchoolClass
	for _, item := range classes {
		if item.ID == req.SchoolClassID && item.Status == "active" {
			selectedClass = item
			break
		}
	}
	if selectedClass.ID == 0 {
		response.Error(c, response.NotFound())
		return
	}
	if err := h.ensureClassAccess(c, selectedClass.ID); err != nil {
		respondAccessError(c, err)
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
		if student.Status != "active" || student.SchoolClassID != selectedClass.ID {
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
		response.Error(c, response.BadRequest("没有可布置作业的在托学生", ErrNotFound))
		return
	}
	createdBy, creatorName, creatorErr := h.currentCreator(c)
	if creatorErr != nil {
		respondAccessError(c, creatorErr)
		return
	}
	item, err := h.store.CreateTask(c.Request.Context(), h.orgID, CreateTaskParams{HomeworkDate: homeworkDate, SchoolID: selectedClass.SchoolID, SchoolClassID: selectedClass.ID, Subject: defaultSubject(req.Subject), Content: strings.TrimSpace(req.Content), AttachmentURLs: normalizeAttachmentURLs(req.AttachmentURLs), CreatedByUserID: createdBy, CreatorName: creatorName}, roster)
	if err != nil {
		respondError(c, err)
		return
	}
	_ = h.notifyHomeworkPublished(c, item, roster)
	h.recordAudit(c, "homework.task.create", "homework_task", item.ID)
	response.Created(c, "/api/v1/homework-tasks/"+strconv.FormatUint(item.ID, 10), toTaskView(item))
}

func (h *Handler) uploadHomeworkPhoto(c *gin.Context) {
	if !canWriteHomework(c) {
		return
	}
	if h.photos == nil {
		response.Error(c, response.Internal(errors.New("photo storage is not configured")))
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, response.BadRequest("请上传作业图片", err))
		return
	}
	file, err := header.Open()
	if err != nil {
		response.Error(c, response.BadRequest("无法读取作业图片", err))
		return
	}
	defer func() { _ = file.Close() }()
	asset, err := saveHomeworkPhoto(c, h.photos, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrFileTooLarge):
			response.Error(c, response.PayloadTooLarge(err))
		case errors.Is(err, storage.ErrUnsupportedContentType):
			response.Error(c, response.UnsupportedMediaType())
		default:
			response.Error(c, response.BadRequest("作业图片上传失败", err))
		}
		return
	}
	if err := h.recordAsset(c, asset, "homework_photo", formUint64(c, "task_id")); err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	response.Created(c, asset.URL, gin.H{"url": asset.URL, "key": asset.Key, "sha256": asset.SHA256, "content_type": asset.ContentType, "size": asset.Size})
}

type categorizedPhotoStore interface {
	SaveIn(context.Context, string, string, string, io.Reader) (storage.Asset, error)
}

func saveHomeworkPhoto(c *gin.Context, photos storage.Store, filename, contentType string, reader io.Reader) (storage.Asset, error) {
	if store, ok := photos.(categorizedPhotoStore); ok {
		return store.SaveIn(c.Request.Context(), "homework", filename, contentType, reader)
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

func (h *Handler) listTaskStudents(c *gin.Context) {
	taskID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.taskForPrincipal(c, taskID); !ok {
		return
	}
	items, err := h.store.ListTaskStudents(c.Request.Context(), h.orgID, taskID)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]taskStudentView, 0, len(items))
	for _, item := range items {
		out = append(out, toTaskStudentView(item))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func (h *Handler) reviewStudent(c *gin.Context) {
	if !canWriteHomework(c) {
		return
	}
	taskID, ok := parsePathID(c, "id")
	if !ok {
		return
	}
	if _, ok := h.taskForPrincipal(c, taskID); !ok {
		return
	}
	studentID, ok := parsePathID(c, "student_id")
	if !ok {
		return
	}
	var req reviewStudentRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	var reviewerID *uint64
	if principal, exists := staffPrincipal(c); exists {
		reviewerID = &principal.SubjectID
	}
	var previousStatus string
	students, studentsErr := h.store.ListTaskStudents(c.Request.Context(), h.orgID, taskID)
	if studentsErr != nil {
		respondError(c, studentsErr)
		return
	}
	for _, student := range students {
		if student.StudentID == studentID {
			previousStatus = student.Status
			break
		}
	}
	item, err := h.store.ReviewStudent(c.Request.Context(), h.orgID, ReviewStudentParams{TaskID: taskID, StudentID: studentID, Status: req.Status, CorrectionNote: strings.TrimSpace(req.CorrectionNote), ReviewedByUserID: reviewerID})
	if err != nil {
		respondError(c, err)
		return
	}
	if previousStatus != req.Status {
		_ = h.notifyHomeworkReview(c, item, req.Status)
	}
	h.recordAudit(c, "homework.student.review", "homework_student", item.ID)
	response.OK(c, toTaskStudentView(item))
}

func (h *Handler) notifyHomeworkPublished(c *gin.Context, task Task, roster []StudentRef) error {
	if h.notifications == nil {
		return nil
	}
	for _, student := range roster {
		if _, err := h.notifications.CreateNotification(c.Request.Context(), h.orgID, pickup.CreateNotificationParams{StudentID: student.ID, Kind: "homework_published", Title: "今日作业已发布", Content: fmt.Sprintf("%s：%s", task.Subject, task.Content)}); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) notifyHomeworkReview(c *gin.Context, student TaskStudent, status string) error {
	if h.notifications == nil {
		return nil
	}
	title := "作业已批改"
	content := fmt.Sprintf("%s 的作业已完成", student.StudentName)
	switch status {
	case StudentStatusIncomplete:
		title = "作业需要订正"
		content = fmt.Sprintf("%s 的作业需要订正", student.StudentName)
	case StudentStatusNotSubmitted:
		title = "作业尚未提交"
		content = fmt.Sprintf("%s 的作业尚未提交", student.StudentName)
	}
	if strings.TrimSpace(student.CorrectionNote) != "" {
		content += "；老师意见：" + strings.TrimSpace(student.CorrectionNote)
	}
	_, err := h.notifications.CreateNotification(c.Request.Context(), h.orgID, pickup.CreateNotificationParams{StudentID: student.StudentID, Kind: "homework_review", Title: title, Content: content})
	return err
}

func (h *Handler) listParentHomework(c *gin.Context) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindParent {
		response.Error(c, response.Unauthorized())
		return
	}
	studentID, ok := parsePathID(c, "student_id")
	if !ok {
		return
	}
	if h.parents == nil {
		response.Error(c, response.Internal(errors.New("家长绑定服务未配置")))
		return
	}
	bindings, err := h.parents.ListBindings(c.Request.Context(), h.orgID, principal.SubjectID)
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	bound := false
	for _, item := range bindings {
		if item.StudentID == studentID {
			bound = true
			break
		}
	}
	if !bound {
		response.Error(c, response.NotFound())
		return
	}
	items, err := h.store.ListStudentHomework(c.Request.Context(), h.orgID, studentID)
	if err != nil {
		respondError(c, err)
		return
	}
	if date := strings.TrimSpace(c.Query("date")); date != "" {
		parsed, parseErr := parseDate(date)
		if parseErr != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
		filtered := make([]StudentHomework, 0, len(items))
		for _, item := range items {
			if sameDay(item.HomeworkDate, parsed) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	out := make([]parentHomeworkView, 0, len(items))
	for _, item := range items {
		out = append(out, parentHomeworkView{TaskID: item.Task.ID, HomeworkDate: item.Task.HomeworkDate.Format("2006-01-02"), SchoolID: item.Task.SchoolID, SchoolClassID: item.Task.SchoolClassID, StudentID: item.TaskStudent.StudentID, StudentName: item.TaskStudent.StudentName, Subject: item.Task.Subject, Content: item.Task.Content, Status: item.TaskStudent.Status, CorrectionNote: item.TaskStudent.CorrectionNote, CreatorName: item.Task.CreatorName, AttachmentURLs: h.signedPhotoURLs(item.Task.AttachmentURLs), ReviewedAt: formatTime(item.TaskStudent.ReviewedAt), CreatedAt: item.TaskStudent.CreatedAt.Format(time.RFC3339)})
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func (h *Handler) filterTasksForPrincipal(c *gin.Context, items []Task) ([]Task, error) {
	principal, scoped := staffPrincipal(c)
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
	filtered := make([]Task, 0, len(items))
	for _, item := range items {
		if _, ok := classes[item.SchoolClassID]; ok {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (h *Handler) taskForPrincipal(c *gin.Context, taskID uint64) (Task, bool) {
	item, err := h.store.FindTask(c.Request.Context(), h.orgID, taskID)
	if err != nil {
		respondError(c, err)
		return Task{}, false
	}
	if err := h.ensureClassAccess(c, item.SchoolClassID); err != nil {
		respondAccessError(c, err)
		return Task{}, false
	}
	return item, true
}

func (h *Handler) ensureClassAccess(c *gin.Context, schoolClassID uint64) error {
	principal, scoped := staffPrincipal(c)
	if !scoped || h.assignments == nil || principal.Role != identity.UserRoleTeacher {
		return nil
	}
	item, err := h.assignments.FindByPair(c.Request.Context(), h.orgID, principal.SubjectID, schoolClassID)
	if err != nil {
		if errors.Is(err, assignment.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if item.Status != assignment.AssignmentStatusActive {
		return ErrNotFound
	}
	return nil
}

func (h *Handler) currentCreator(c *gin.Context) (*uint64, string, error) {
	principal, scoped := staffPrincipal(c)
	if !scoped || h.users == nil {
		return nil, "", nil
	}
	user, err := h.users.FindUserByID(c.Request.Context(), principal.SubjectID)
	if err != nil {
		return nil, "", err
	}
	if user.Status != identity.UserStatusActive {
		return nil, "", errors.New("homework: current user is disabled")
	}
	return &principal.SubjectID, user.Nickname, nil
}

func staffPrincipal(c *gin.Context) (identity.Principal, bool) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser {
		return identity.Principal{}, false
	}
	return principal, true
}

func parsePathID(c *gin.Context, key string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || value == 0 {
		response.Error(c, response.BadRequest(key+" 不合法", err))
		return 0, false
	}
	return value, true
}

func parseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.UTC)
}

func sameDay(left, right time.Time) bool {
	left, right = left.UTC(), right.UTC()
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}

func defaultSubject(value string) string {
	if strings.TrimSpace(value) == "" {
		return "综合作业"
	}
	return strings.TrimSpace(value)
}

func normalizeAttachmentURLs(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (h *Handler) signedPhotoURLs(values []string) []string {
	if h.photoSigner == nil {
		return append([]string(nil), values...)
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, h.photoSigner.Sign(value, 15*time.Minute))
	}
	return out
}

func canWriteHomework(c *gin.Context) bool {
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

func toTaskView(item Task) taskView {
	return taskView{ID: item.ID, HomeworkDate: item.HomeworkDate.Format("2006-01-02"), SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, Subject: item.Subject, Content: item.Content, AttachmentURLs: item.AttachmentURLs, CreatedByUserID: item.CreatedByUserID, CreatorName: item.CreatorName, Status: item.Status, CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}

func toTaskStudentView(item TaskStudent) taskStudentView {
	return taskStudentView{ID: item.ID, TaskID: item.TaskID, StudentID: item.StudentID, StudentName: item.StudentName, Status: item.Status, CorrectionNote: item.CorrectionNote, ReviewedByUserID: item.ReviewedByUserID, ReviewedAt: formatTime(item.ReviewedAt), CreatedAt: item.CreatedAt.Format(time.RFC3339), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)}
}

func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(c, response.NotFound())
	case errors.Is(err, ErrConflict):
		response.Error(c, response.BadRequest("作业没有可关联的学生", err))
	case errors.Is(err, ErrInvalidStatus):
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}))
	default:
		response.Error(c, response.Internal(err))
	}
}

func respondAccessError(c *gin.Context, err error) {
	if errors.Is(err, ErrNotFound) {
		response.Error(c, response.BadRequest("当前教师没有负责该学校班级", err))
		return
	}
	response.Error(c, response.Internal(err))
}

func respondMasterDataError(c *gin.Context, err error) {
	if errors.Is(err, masterdata.ErrNotFound) {
		response.Error(c, response.NotFound())
		return
	}
	response.Error(c, response.Internal(err))
}
