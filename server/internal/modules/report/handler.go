package report

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
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
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/summary"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/businessdate"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	pickup      pickup.Store
	homework    homework.Store
	meals       meal.Store
	parents     parent.Store
	masterData  masterdata.Store
	summaries   summary.Store
	assignments assignment.Store
	audit       auditmodule.Store
	orgID       uint64
}

// dailyExceptionFilter narrows the actionable checklist to the workflow the
// caller is currently completing. A class filter applies to every module;
// an operation filter applies only to pickup records because homework,
// leave, meal and summary records do not belong to one pickup operation.
type dailyExceptionFilter struct {
	SchoolClassID uint64
	OperationID   uint64
}

func NewHandler(pickupStore pickup.Store, homeworkStore homework.Store, mealStore meal.Store, parentStore parent.Store, masterDataStore masterdata.Store, summaryStore summary.Store, assignmentStore assignment.Store) *Handler {
	return &Handler{pickup: pickupStore, homework: homeworkStore, meals: mealStore, parents: parentStore, masterData: masterDataStore, summaries: summaryStore, assignments: assignmentStore, orgID: masterdata.DefaultOrganizationID}
}

func (h *Handler) SetAuditStore(store auditmodule.Store) { h.audit = store }

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/reports/daily-overview", h.dailyOverview)
	api.GET("/reports/daily-exceptions", h.dailyExceptions)
	api.POST("/reports/daily-exceptions/:id/acknowledge", h.acknowledgeDailyException)
}

func (h *Handler) dailyOverview(c *gin.Context) {
	date := businessdate.Today(time.Now())
	if value := strings.TrimSpace(c.Query("date")); value != "" {
		parsed, err := time.ParseInLocation("2006-01-02", value, time.UTC)
		if err != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
		date = parsed
	}
	out, err := h.build(c, date)
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	response.OK(c, out)
}

func (h *Handler) dailyExceptions(c *gin.Context) {
	date := businessdate.Today(time.Now())
	if value := strings.TrimSpace(c.Query("date")); value != "" {
		parsed, err := time.ParseInLocation("2006-01-02", value, time.UTC)
		if err != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
		date = parsed
	}
	filter, filterErr := parseDailyExceptionFilter(c)
	if filterErr != nil {
		response.Error(c, filterErr)
		return
	}
	out, err := h.buildDailyExceptions(c, date, filter)
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	includeAcknowledged := strings.EqualFold(strings.TrimSpace(c.Query("include_acknowledged")), "true") || strings.TrimSpace(c.Query("include_acknowledged")) == "1"
	h.applyAcknowledgements(c, date, &out, includeAcknowledged)
	response.OK(c, out)
}

func parseDailyExceptionFilter(c *gin.Context) (dailyExceptionFilter, *response.AppError) {
	filter := dailyExceptionFilter{}
	for key, target := range map[string]*uint64{
		"school_class_id": &filter.SchoolClassID,
		"operation_id":    &filter.OperationID,
	} {
		value := strings.TrimSpace(c.Query(key))
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			return dailyExceptionFilter{}, response.ValidationFailed([]response.ValidationDetail{{Field: key, Reason: "invalid_value"}})
		}
		*target = parsed
	}
	return filter, nil
}

type acknowledgeDailyExceptionRequest struct {
	Note string `json:"note"`
}

func (r acknowledgeDailyExceptionRequest) Validate() []response.ValidationDetail {
	if len([]rune(strings.TrimSpace(r.Note))) > 200 {
		return []response.ValidationDetail{{Field: "note", Reason: "too_long"}}
	}
	return nil
}

func (h *Handler) acknowledgeDailyException(c *gin.Context) {
	date, err := parseBusinessDate(c.Query("date"))
	if err != nil {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		response.Error(c, response.BadRequest("异常 ID 不能为空", nil))
		return
	}
	current, err := h.buildDailyExceptions(c, date, dailyExceptionFilter{})
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	found := false
	for _, item := range current.Items {
		if item.ID == id {
			found = true
			break
		}
	}
	if !found {
		response.Error(c, response.NotFound())
		return
	}
	var req acknowledgeDailyExceptionRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if h.audit == nil {
		response.Error(c, response.Internal(fmt.Errorf("异常审计存储未配置")))
		return
	}
	// A repeated tap or client retry must be safe. The audit log is the
	// source of truth for acknowledgement state, so do not append another
	// record when this exception was already acknowledged for this date.
	if acknowledged, err := h.isAcknowledged(c, date, id); err != nil {
		response.Error(c, response.Internal(err))
		return
	} else if acknowledged {
		response.OK(c, gin.H{"id": id, "acknowledged": true})
		return
	}
	metadata, _ := json.Marshal(map[string]string{"exception_id": id, "business_date": date.Format("2006-01-02"), "note": strings.TrimSpace(req.Note)})
	resourceID := exceptionResourceID(id)
	if err := auditmodule.RecordForContextWithError(c.Request.Context(), h.audit, identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), "exception.acknowledge", "daily_exception", &resourceID, string(metadata), c.GetHeader("X-Request-ID")); err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	response.OK(c, gin.H{"id": id, "acknowledged": true})
}

func (h *Handler) isAcknowledged(c *gin.Context, date time.Time, exceptionID string) (bool, error) {
	entries, err := h.audit.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), auditmodule.ListFilter{Action: "exception.acknowledge", ResourceType: "daily_exception", Limit: 200})
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		var metadata struct {
			ExceptionID  string `json:"exception_id"`
			BusinessDate string `json:"business_date"`
		}
		if json.Unmarshal([]byte(entry.MetadataJSON), &metadata) == nil && metadata.ExceptionID == exceptionID && metadata.BusinessDate == date.Format("2006-01-02") {
			return true, nil
		}
	}
	return false, nil
}

func (h *Handler) applyAcknowledgements(c *gin.Context, date time.Time, out *DailyExceptions, includeAcknowledged bool) {
	if h.audit == nil || out == nil {
		return
	}
	entries, err := h.audit.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), auditmodule.ListFilter{Action: "exception.acknowledge", ResourceType: "daily_exception", Limit: 200})
	if err != nil {
		return
	}
	acknowledged := make(map[string]auditmodule.Entry, len(entries))
	for _, entry := range entries {
		var metadata struct {
			ExceptionID  string `json:"exception_id"`
			BusinessDate string `json:"business_date"`
		}
		if json.Unmarshal([]byte(entry.MetadataJSON), &metadata) != nil || metadata.BusinessDate != out.Date || metadata.ExceptionID == "" {
			continue
		}
		if _, exists := acknowledged[metadata.ExceptionID]; !exists {
			acknowledged[metadata.ExceptionID] = entry
		}
	}
	filtered := make([]DailyException, 0, len(out.Items))
	for _, item := range out.Items {
		entry, ok := acknowledged[item.ID]
		if ok {
			item.Acknowledged = true
			item.AcknowledgedAt = entry.CreatedAt
			if entry.ActorID != nil {
				item.AcknowledgedBy = fmt.Sprintf("工作人员 #%d", *entry.ActorID)
			} else {
				item.AcknowledgedBy = entry.ActorType
			}
		}
		if !item.Acknowledged || includeAcknowledged {
			filtered = append(filtered, item)
		}
	}
	out.Items = filtered
	out.Counts = map[string]int{}
	for _, item := range out.Items {
		out.Counts[item.Category]++
	}
}

func parseBusinessDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return businessdate.Today(time.Now()), nil
	}
	return time.ParseInLocation("2006-01-02", value, time.UTC)
}

func exceptionResourceID(value string) uint64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(value))
	result := hasher.Sum64()
	if result == 0 {
		return 1
	}
	return result
}

func (h *Handler) buildDailyExceptions(c *gin.Context, date time.Time, filter dailyExceptionFilter) (DailyExceptions, error) {
	orgID := identity.OrganizationIDFromContext(c.Request.Context(), h.orgID)
	out := DailyExceptions{Date: date.Format("2006-01-02"), Items: []DailyException{}, Counts: map[string]int{}}
	classNames := map[uint64]string{}
	studentClasses := map[uint64]uint64{}
	studentNames := map[uint64]string{}
	if h.masterData != nil {
		classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), orgID)
		if err != nil {
			return out, err
		}
		for _, item := range classes {
			classNames[item.ID] = strings.TrimSpace(item.Grade + item.Name)
		}
		students, err := h.masterData.ListStudents(c.Request.Context(), orgID)
		if err != nil {
			return out, err
		}
		for _, item := range students {
			studentClasses[item.ID] = item.SchoolClassID
			studentNames[item.ID] = item.Name
		}
	}

	classLabel := func(classID uint64) string {
		if value := strings.TrimSpace(classNames[classID]); value != "" {
			return value
		}
		return fmt.Sprintf("班级 #%d", classID)
	}
	add := func(item DailyException) {
		if item.ID == "" {
			item.ID = fmt.Sprintf("%s:%d:%d:%d:%d", item.Code, item.SchoolClassID, item.StudentID, item.OperationID, item.TaskID)
		}
		out.Items = append(out.Items, item)
		out.Counts[item.Category]++
	}

	activeOperations := 0
	if h.pickup != nil {
		operations, err := h.pickup.ListOperations(c.Request.Context(), orgID)
		if err != nil {
			return out, err
		}
		for _, operation := range operations {
			if operation.Status == pickup.OperationStatusCancelled || !sameDay(operation.OperationDate, date) || !h.classAllowed(c, operation.SchoolClassID) {
				continue
			}
			if filter.SchoolClassID != 0 && operation.SchoolClassID != filter.SchoolClassID {
				continue
			}
			if filter.OperationID != 0 && operation.ID != filter.OperationID {
				continue
			}
			activeOperations++
			members, err := h.pickup.ListOperationStudents(c.Request.Context(), orgID, operation.ID)
			if err != nil {
				return out, err
			}
			for _, member := range members {
				base := DailyException{Category: "pickup", Severity: "danger", SchoolClassID: operation.SchoolClassID, ClassName: classLabel(operation.SchoolClassID), StudentID: member.StudentID, StudentName: member.StudentName, OperationID: operation.ID, Action: "/pages/pickup/index"}
				switch member.Status {
				case pickup.MemberStatusPlanned:
					base.Code, base.Label, base.Message = "pickup_pending", "接送待确认", fmt.Sprintf("%s 尚未完成今日接送登记", member.StudentName)
				case pickup.MemberStatusAbsent:
					base.Code, base.Label, base.Message = "pickup_absent", "未找到孩子", fmt.Sprintf("%s 记录为未找到，请核对家长和学校情况", member.StudentName)
				case pickup.MemberStatusNotArrived:
					base.Code, base.Label, base.Message = "pickup_not_arrived", "到班异常", fmt.Sprintf("%s 已接到但尚未确认到班", member.StudentName)
				case pickup.MemberStatusAbnormal:
					base.Code, base.Label, base.Message = "pickup_abnormal", "接送异常", fmt.Sprintf("%s 存在接送异常，请补充处理说明", member.StudentName)
				case pickup.MemberStatusPickedUp:
					if strings.TrimSpace(member.PhotoURL) == "" {
						base.Code, base.Label, base.Message = "pickup_photo_missing", "待补接送照片", fmt.Sprintf("%s 已登记接到但还没有接送照片", member.StudentName)
					}
				default:
					if !member.ProfilePending {
						continue
					}
				}
				if base.Code != "" {
					add(base)
				}
				if member.ProfilePending {
					add(DailyException{Code: "student_profile_pending", Category: "student", Severity: "warning", Label: "临时档案待补", Message: fmt.Sprintf("%s 是临时学生，档案还未补充完整", member.StudentName), SchoolClassID: operation.SchoolClassID, ClassName: classLabel(operation.SchoolClassID), StudentID: member.StudentID, StudentName: member.StudentName, OperationID: operation.ID, Action: "/pages/pickup/index"})
				}
			}
		}
	}

	if h.homework != nil {
		tasks, err := h.homework.ListTasks(c.Request.Context(), orgID)
		if err != nil {
			return out, err
		}
		for _, task := range tasks {
			if task.Status != homework.TaskStatusActive || !sameDay(task.HomeworkDate, date) || !h.classAllowed(c, task.SchoolClassID) {
				continue
			}
			students, err := h.homework.ListTaskStudents(c.Request.Context(), orgID, task.ID)
			if err != nil {
				return out, err
			}
			for _, student := range students {
				label := "作业待反馈"
				switch student.Status {
				case homework.StudentStatusIncomplete:
					label = "作业未完成"
				case homework.StudentStatusNotSubmitted:
					label = "作业未提交"
				case homework.StudentStatusPending:
					label = "作业待批改"
				default:
					continue
				}
				add(DailyException{Code: "homework_pending", Category: "homework", Severity: "warning", Label: label, Message: fmt.Sprintf("%s 的%s还没有完成处理", student.StudentName, task.Subject), SchoolClassID: task.SchoolClassID, ClassName: classLabel(task.SchoolClassID), StudentID: student.StudentID, StudentName: student.StudentName, TaskID: task.ID, Action: "/pages/homework/index"})
			}
		}
	}

	if h.meals != nil && activeOperations > 0 {
		from, to := date, date
		plans, err := h.meals.ListPlans(c.Request.Context(), orgID, &from, &to)
		if err != nil {
			return out, err
		}
		if len(plans) == 0 {
			add(DailyException{Code: "meal_missing", Category: "meal", Severity: "warning", Label: "今日餐食未登记", Message: "今天有托管接送任务，但还没有登记餐食安排", Action: "/pages/meals/index"})
		}
	}

	if h.parents != nil {
		leaves, err := h.parents.ListLeaveRequests(c.Request.Context(), orgID, nil)
		if err != nil {
			return out, err
		}
		for _, leave := range leaves {
			classID := studentClasses[leave.StudentID]
			if leave.Status != parent.LeaveStatusPending || !sameDay(leave.LeaveDate, date) || classID == 0 || !h.classAllowed(c, classID) {
				continue
			}
			name := studentNames[leave.StudentID]
			if name == "" {
				name = fmt.Sprintf("学生 #%d", leave.StudentID)
			}
			add(DailyException{Code: "leave_pending", Category: "leave", Severity: "warning", Label: "请假待处理", Message: fmt.Sprintf("%s 的请假申请还未处理：%s", name, strings.TrimSpace(leave.Reason)), SchoolClassID: classID, ClassName: classLabel(classID), StudentID: leave.StudentID, StudentName: name, Action: "/pages/pickup/index"})
		}
		applications, err := h.parents.ListChildApplications(c.Request.Context(), orgID, nil)
		if err != nil {
			return out, err
		}
		for _, application := range applications {
			if (application.Status != parent.ChildApplicationStatusPending && application.Status != parent.ChildApplicationStatusNeedsInfo) || application.SchoolClassID == nil || !h.classAllowed(c, *application.SchoolClassID) {
				continue
			}
			add(DailyException{Code: "application_pending", Category: "application", Severity: "warning", Label: "入班申请待处理", Message: fmt.Sprintf("%s 的入班申请还需要教师确认", application.StudentName), SchoolClassID: *application.SchoolClassID, ClassName: classLabel(*application.SchoolClassID), StudentName: application.StudentName, Action: "/pages/child-applications/index"})
		}
	}

	if h.summaries != nil && activeOperations > 0 {
		summaries, err := h.summaries.List(c.Request.Context(), orgID, &date)
		if err != nil {
			return out, err
		}
		published := false
		for _, item := range summaries {
			if !h.summaryAllowed(c, item) {
				continue
			}
			if item.Status == summary.StatusPublished || item.Status == summary.StatusClosed {
				published = true
				break
			}
		}
		if !published {
			add(DailyException{Code: "summary_pending", Category: "summary", Severity: "warning", Label: "每日总结待发布", Message: "今天的托管每日总结还没有发布给家长", Action: "/pages/summary/index"})
		}
	}
	if filter.SchoolClassID != 0 && !h.classAllowed(c, filter.SchoolClassID) {
		return DailyExceptions{Date: out.Date, Items: []DailyException{}, Counts: map[string]int{}}, nil
	}
	if filter.SchoolClassID != 0 || filter.OperationID != 0 {
		filtered := make([]DailyException, 0, len(out.Items))
		counts := map[string]int{}
		for _, item := range out.Items {
			if filter.SchoolClassID != 0 && item.SchoolClassID != 0 && item.SchoolClassID != filter.SchoolClassID {
				continue
			}
			if filter.OperationID != 0 && item.Category == "pickup" && item.OperationID != filter.OperationID {
				continue
			}
			filtered = append(filtered, item)
			counts[item.Category]++
		}
		out.Items = filtered
		out.Counts = counts
	}
	return out, nil
}

func (h *Handler) build(c *gin.Context, date time.Time) (DailyOverview, error) {
	out := DailyOverview{Date: date.Format("2006-01-02"), Pickup: PickupOverview{Statuses: map[string]int{}}, Homework: HomeworkOverview{Statuses: map[string]int{}}, Anomalies: []Anomaly{}, Classes: []ClassOverview{}}
	classNames := map[uint64]string{}
	studentClasses := map[uint64]uint64{}
	if h.masterData != nil {
		classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		if err != nil {
			return out, err
		}
		for _, item := range classes {
			classNames[item.ID] = strings.TrimSpace(item.Grade + item.Name)
		}
		students, err := h.masterData.ListStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		if err != nil {
			return out, err
		}
		for _, item := range students {
			studentClasses[item.ID] = item.SchoolClassID
		}
	}
	classIndex := map[uint64]int{}
	if h.pickup != nil {
		operations, err := h.pickup.ListOperations(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		if err != nil {
			return out, err
		}
		for _, operation := range operations {
			if !sameDay(operation.OperationDate, date) || !h.classAllowed(c, operation.SchoolClassID) {
				continue
			}
			out.Pickup.Operations++
			index, ok := classIndex[operation.SchoolClassID]
			if !ok {
				out.Classes = append(out.Classes, ClassOverview{SchoolClassID: operation.SchoolClassID, ClassName: classNames[operation.SchoolClassID]})
				index = len(out.Classes) - 1
				classIndex[operation.SchoolClassID] = index
			}
			out.Classes[index].Operations++
			members, err := h.pickup.ListOperationStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), operation.ID)
			if err != nil {
				return out, err
			}
			for _, member := range members {
				out.Pickup.Students++
				out.Classes[index].Students++
				out.Pickup.Statuses[member.Status]++
				if pickup.IsReadyToFinish(member.Status) {
					out.Pickup.Resolved++
					out.Classes[index].Resolved++
				}
				if member.Status == pickup.MemberStatusAbsent || member.Status == pickup.MemberStatusNotArrived || member.Status == pickup.MemberStatusAbnormal {
					out.Classes[index].Abnormal++
				}
				if (member.Status == pickup.MemberStatusPickedUp || member.Status == pickup.MemberStatusAbnormal) && strings.TrimSpace(member.PhotoURL) == "" {
					out.Pickup.PhotoMissing++
				}
			}
		}
	}
	if h.homework != nil {
		tasks, err := h.homework.ListTasks(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		if err != nil {
			return out, err
		}
		for _, task := range tasks {
			if !sameDay(task.HomeworkDate, date) || task.Status != homework.TaskStatusActive || !h.classAllowed(c, task.SchoolClassID) {
				continue
			}
			out.Homework.Tasks++
			members, err := h.homework.ListTaskStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), task.ID)
			if err != nil {
				return out, err
			}
			for _, member := range members {
				out.Homework.Students++
				out.Homework.Statuses[member.Status]++
				switch member.Status {
				case homework.StudentStatusCompleted:
					out.Homework.Completed++
				case homework.StudentStatusIncomplete:
					out.Homework.Incomplete++
				case homework.StudentStatusNotSubmitted:
					out.Homework.NotSubmitted++
				}
			}
		}
	}
	if h.meals != nil {
		from, to := date, date
		plans, err := h.meals.ListPlans(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), &from, &to)
		if err != nil {
			return out, err
		}
		out.MealPlans = len(plans)
		out.MealRecorded = len(plans) > 0
	}
	if h.parents != nil {
		applications, err := h.parents.ListChildApplications(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), nil)
		if err != nil {
			return out, err
		}
		for _, item := range applications {
			if !h.applicationAllowed(c, item.SchoolClassID) {
				continue
			}
			if item.Status == parent.ChildApplicationStatusPending || item.Status == parent.ChildApplicationStatusNeedsInfo {
				out.PendingApplications++
			}
		}
		leaves, err := h.parents.ListLeaveRequests(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), nil)
		if err != nil {
			return out, err
		}
		for _, item := range leaves {
			if sameDay(item.LeaveDate, date) && item.Status == parent.LeaveStatusPending && h.classAllowed(c, studentClasses[item.StudentID]) {
				out.PendingLeaveRequests++
			}
		}
	}
	if h.summaries != nil {
		items, err := h.summaries.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), &date)
		if err != nil {
			return out, err
		}
		for _, item := range items {
			if h.summaryAllowed(c, item) {
				out.SummaryStatus = item.Status
				break
			}
		}
	}
	out.Anomalies = buildAnomalies(out)
	return out, nil
}

func (h *Handler) applicationAllowed(c *gin.Context, schoolClassID *uint64) bool {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser || principal.Role != identity.UserRoleTeacher {
		return true
	}
	return schoolClassID != nil && h.classAllowed(c, *schoolClassID)
}

func buildAnomalies(value DailyOverview) []Anomaly {
	items := make([]Anomaly, 0, 6)
	add := func(code, label string, count int) {
		if count > 0 {
			items = append(items, Anomaly{Code: code, Label: label, Count: count})
		}
	}
	add("pickup_pending", "接送待确认", value.Pickup.Statuses[pickup.MemberStatusPlanned])
	add("pickup_absent", "未找到孩子", value.Pickup.Statuses[pickup.MemberStatusAbsent])
	add("pickup_not_arrived", "到班异常", value.Pickup.Statuses[pickup.MemberStatusNotArrived])
	add("pickup_abnormal", "接送异常", value.Pickup.Statuses[pickup.MemberStatusAbnormal])
	add("pickup_photo_missing", "待补接送照片", value.Pickup.PhotoMissing)
	add("homework_pending", "作业未完成", value.Homework.Incomplete+value.Homework.NotSubmitted)
	add("meal_missing", "今日餐食未登记", boolCount(value.MealRecorded, value.Pickup.Operations > 0))
	add("leave_pending", "请假待处理", value.PendingLeaveRequests)
	add("application_pending", "入班申请待处理", value.PendingApplications)
	return items
}

func boolCount(recorded, active bool) int {
	if active && !recorded {
		return 1
	}
	return 0
}

func (h *Handler) classAllowed(c *gin.Context, schoolClassID uint64) bool {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser || principal.Role != identity.UserRoleTeacher {
		return true
	}
	if h.assignments == nil {
		return false
	}
	item, err := h.assignments.FindByPair(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), principal.SubjectID, schoolClassID)
	return err == nil && item.Status == assignment.AssignmentStatusActive
}

func (h *Handler) summaryAllowed(c *gin.Context, item summary.DailySummary) bool {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser || principal.Role != identity.UserRoleTeacher {
		return true
	}
	if h.pickup == nil {
		return h.assignments != nil && h.teacherHasAssignment(c, principal.SubjectID)
	}
	operations, err := h.pickup.ListOperations(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return false
	}
	for _, operation := range operations {
		if sameDay(operation.OperationDate, item.SummaryDate) && h.classAllowed(c, operation.SchoolClassID) {
			return true
		}
	}
	return h.teacherHasAssignment(c, principal.SubjectID)
}

func (h *Handler) teacherHasAssignment(c *gin.Context, teacherID uint64) bool {
	if h.assignments == nil {
		return false
	}
	items, err := h.assignments.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), teacherID, 0)
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

func sameDay(a, b time.Time) bool {
	a, b = a.UTC(), b.UTC()
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}
