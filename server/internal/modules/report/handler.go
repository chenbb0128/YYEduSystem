package report

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/homework"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/meal"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/summary"
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
	orgID       uint64
}

func NewHandler(pickupStore pickup.Store, homeworkStore homework.Store, mealStore meal.Store, parentStore parent.Store, masterDataStore masterdata.Store, summaryStore summary.Store, assignmentStore assignment.Store) *Handler {
	return &Handler{pickup: pickupStore, homework: homeworkStore, meals: mealStore, parents: parentStore, masterData: masterDataStore, summaries: summaryStore, assignments: assignmentStore, orgID: masterdata.DefaultOrganizationID}
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/reports/daily-overview", h.dailyOverview)
}

func (h *Handler) dailyOverview(c *gin.Context) {
	date := time.Now().UTC()
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
