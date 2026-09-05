package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	auditmodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/audit"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/homework"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/meal"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
)

func TestBuildAnomaliesIncludesOnlyActionableItems(t *testing.T) {
	items := buildAnomalies(DailyOverview{
		Pickup: PickupOverview{
			Operations: 1,
			Statuses: map[string]int{
				pickup.MemberStatusPlanned:    1,
				pickup.MemberStatusAbsent:     2,
				pickup.MemberStatusNotArrived: 1,
				pickup.MemberStatusAbnormal:   1,
			},
			PhotoMissing: 3,
		},
		Homework:             HomeworkOverview{Incomplete: 1, NotSubmitted: 2},
		MealRecorded:         false,
		PendingApplications:  2,
		PendingLeaveRequests: 1,
	})
	if len(items) != 9 {
		t.Fatalf("anomalies = %+v, want 9 actionable items", items)
	}
	if items[0].Code != "pickup_pending" || items[0].Count != 1 {
		t.Fatalf("first anomaly = %+v", items[0])
	}
}

func TestBuildAnomaliesDoesNotFlagMealWithoutPickup(t *testing.T) {
	items := buildAnomalies(DailyOverview{Pickup: PickupOverview{Statuses: map[string]int{}}, MealRecorded: false})
	if len(items) != 0 {
		t.Fatalf("anomalies = %+v, want empty without active pickup", items)
	}
}

func TestDailyOverviewHTTPScopesTeacherToAssignedClasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: reportDate("2026-09-01"), EndsOn: reportDate("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	classOne, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	classTwo, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "2班"})
	if err != nil {
		t.Fatal(err)
	}
	studentOne, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: classOne.ID, Name: "小明"})
	if err != nil {
		t.Fatal(err)
	}
	studentTwo, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: classTwo.ID, Name: "小雨"})
	if err != nil {
		t.Fatal(err)
	}
	pickupStore := pickup.NewMemoryStore()
	if _, err := pickupStore.CreateOperation(ctx, masterdata.DefaultOrganizationID, pickup.CreateOperationParams{OperationDate: reportDate("2026-09-02"), SchoolID: school.ID, SchoolClassID: classOne.ID, TeacherName: "王老师"}, []pickup.StudentRef{{ID: studentOne.ID, Name: studentOne.Name}}); err != nil {
		t.Fatal(err)
	}
	if _, err := pickupStore.CreateOperation(ctx, masterdata.DefaultOrganizationID, pickup.CreateOperationParams{OperationDate: reportDate("2026-09-02"), SchoolID: school.ID, SchoolClassID: classTwo.ID, TeacherName: "李老师"}, []pickup.StudentRef{{ID: studentTwo.ID, Name: studentTwo.Name}}); err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 31, Role: identity.UserRoleTeacher}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: classOne.ID}); err != nil {
		t.Fatal(err)
	}
	otherTeacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 32, Role: identity.UserRoleTeacher}
	handler := NewHandler(pickupStore, nil, nil, nil, master, nil, assignments)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	teacherResponse := reportHTTPAs(t, router, teacher, http.MethodGet, "/api/v1/reports/daily-overview?date=2026-09-02", "")
	var teacherOverview DailyOverview
	decodeReportData(t, teacherResponse, &teacherOverview)
	if teacherOverview.Pickup.Operations != 1 || len(teacherOverview.Classes) != 1 || teacherOverview.Classes[0].SchoolClassID != classOne.ID {
		t.Fatalf("teacher overview = %+v", teacherOverview)
	}

	otherResponse := reportHTTPAs(t, router, otherTeacher, http.MethodGet, "/api/v1/reports/daily-overview?date=2026-09-02", "")
	var otherOverview DailyOverview
	decodeReportData(t, otherResponse, &otherOverview)
	if otherOverview.Pickup.Operations != 0 || len(otherOverview.Classes) != 0 {
		t.Fatalf("unassigned teacher overview = %+v", otherOverview)
	}

	admin := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 1, Role: identity.UserRoleAdmin}
	adminResponse := reportHTTPAs(t, router, admin, http.MethodGet, "/api/v1/reports/daily-overview?date=2026-09-02", "")
	var adminOverview DailyOverview
	decodeReportData(t, adminResponse, &adminOverview)
	if adminOverview.Pickup.Operations != 2 || len(adminOverview.Classes) != 2 {
		t.Fatalf("admin overview = %+v", adminOverview)
	}
}

func TestDailyExceptionsHTTPScopesTeacherAndIncludesActionableRecords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: reportDate("2026-09-01"), EndsOn: reportDate("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	classOne, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	classTwo, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "2班"})
	if err != nil {
		t.Fatal(err)
	}
	studentOne, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: classOne.ID, Name: "小明"})
	if err != nil {
		t.Fatal(err)
	}
	studentTwo, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: classTwo.ID, Name: "小雨"})
	if err != nil {
		t.Fatal(err)
	}
	pickupStore := pickup.NewMemoryStore()
	operationOne, err := pickupStore.CreateOperation(ctx, masterdata.DefaultOrganizationID, pickup.CreateOperationParams{OperationDate: reportDate("2026-09-02"), SchoolID: school.ID, SchoolClassID: classOne.ID, TeacherName: "王老师"}, []pickup.StudentRef{{ID: studentOne.ID, Name: studentOne.Name}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pickupStore.CreateOperation(ctx, masterdata.DefaultOrganizationID, pickup.CreateOperationParams{OperationDate: reportDate("2026-09-02"), SchoolID: school.ID, SchoolClassID: classTwo.ID, TeacherName: "李老师"}, []pickup.StudentRef{{ID: studentTwo.ID, Name: studentTwo.Name}}); err != nil {
		t.Fatal(err)
	}
	homeworkStore := homework.NewMemoryStore()
	if _, err := homeworkStore.CreateTask(ctx, masterdata.DefaultOrganizationID, homework.CreateTaskParams{HomeworkDate: reportDate("2026-09-02"), SchoolID: school.ID, SchoolClassID: classOne.ID, Subject: "数学", Content: "练习册"}, []homework.StudentRef{{ID: studentOne.ID, Name: studentOne.Name}}); err != nil {
		t.Fatal(err)
	}
	if _, err := homeworkStore.CreateTask(ctx, masterdata.DefaultOrganizationID, homework.CreateTaskParams{HomeworkDate: reportDate("2026-09-02"), SchoolID: school.ID, SchoolClassID: classTwo.ID, Subject: "语文", Content: "阅读"}, []homework.StudentRef{{ID: studentTwo.ID, Name: studentTwo.Name}}); err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 31, Role: identity.UserRoleTeacher}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: classOne.ID}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(pickupStore, homeworkStore, meal.NewMemoryStore(), nil, master, nil, assignments)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	record := reportHTTPAs(t, router, teacher, http.MethodGet, "/api/v1/reports/daily-exceptions?date=2026-09-02", "")
	var result DailyExceptions
	decodeReportData(t, record, &result)
	if len(result.Items) != 3 || result.Counts["pickup"] != 1 || result.Counts["homework"] != 1 || result.Counts["meal"] != 1 {
		t.Fatalf("daily exceptions = %+v", result)
	}
	for _, item := range result.Items {
		if item.SchoolClassID == classTwo.ID || item.StudentID == studentTwo.ID {
			t.Fatalf("teacher can see unassigned class exception: %+v", item)
		}
	}

	// The close-check view is narrower than the teacher's whole workbench:
	// class-level warnings remain visible, while pickup warnings from another
	// operation are excluded.
	admin := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 1, Role: identity.UserRoleAdmin}
	filteredRecord := reportHTTPAs(t, router, admin, http.MethodGet, fmt.Sprintf("/api/v1/reports/daily-exceptions?date=2026-09-02&school_class_id=%d&operation_id=%d", classOne.ID, operationOne.ID), "")
	var filtered DailyExceptions
	decodeReportData(t, filteredRecord, &filtered)
	if len(filtered.Items) != 3 || filtered.Counts["pickup"] != 1 || filtered.Counts["homework"] != 1 || filtered.Counts["meal"] != 1 {
		t.Fatalf("filtered daily exceptions = %+v", filtered)
	}
	for _, item := range filtered.Items {
		if item.SchoolClassID == classTwo.ID || (item.Category == "pickup" && item.OperationID != operationOne.ID) {
			t.Fatalf("filtered exceptions contain another class or operation: %+v", item)
		}
	}
}

func TestDailyExceptionAcknowledgementIsScopedVisibleAndIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: reportDate("2026-09-01"), EndsOn: reportDate("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	schoolClass, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	student, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: schoolClass.ID, Name: "小明"})
	if err != nil {
		t.Fatal(err)
	}
	pickupStore := pickup.NewMemoryStore()
	if _, err := pickupStore.CreateOperation(ctx, masterdata.DefaultOrganizationID, pickup.CreateOperationParams{OperationDate: reportDate("2026-09-02"), SchoolID: school.ID, SchoolClassID: schoolClass.ID, TeacherName: "王老师"}, []pickup.StudentRef{{ID: student.ID, Name: student.Name}}); err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 31, Role: identity.UserRoleTeacher}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: schoolClass.ID}); err != nil {
		t.Fatal(err)
	}
	otherTeacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 32, Role: identity.UserRoleTeacher}
	auditStore := auditmodule.NewMemoryStore()
	handler := NewHandler(pickupStore, nil, meal.NewMemoryStore(), nil, master, nil, assignments)
	handler.SetAuditStore(auditStore)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	initial := reportHTTPAs(t, router, teacher, http.MethodGet, "/api/v1/reports/daily-exceptions?date=2026-09-02", "")
	var exceptions DailyExceptions
	decodeReportData(t, initial, &exceptions)
	var exceptionID string
	for _, item := range exceptions.Items {
		if item.Code == "pickup_pending" {
			exceptionID = item.ID
			break
		}
	}
	if exceptionID == "" {
		t.Fatalf("pickup exception not found: %+v", exceptions.Items)
	}

	other := reportHTTPAs(t, router, otherTeacher, http.MethodPost, "/api/v1/reports/daily-exceptions/"+exceptionID+"/acknowledge?date=2026-09-02", `{"note":"越权测试"}`)
	if other.Code != http.StatusNotFound {
		t.Fatalf("unassigned teacher acknowledge status = %d, want %d: %s", other.Code, http.StatusNotFound, other.Body.String())
	}

	acknowledge := reportHTTPAs(t, router, teacher, http.MethodPost, "/api/v1/reports/daily-exceptions/"+exceptionID+"/acknowledge?date=2026-09-02", `{"note":"已电话联系家长"}`)
	if acknowledge.Code != http.StatusOK {
		t.Fatalf("acknowledge status = %d: %s", acknowledge.Code, acknowledge.Body.String())
	}
	acknowledgeAgain := reportHTTPAs(t, router, teacher, http.MethodPost, "/api/v1/reports/daily-exceptions/"+exceptionID+"/acknowledge?date=2026-09-02", `{"note":"重复点击"}`)
	if acknowledgeAgain.Code != http.StatusOK {
		t.Fatalf("repeated acknowledge status = %d: %s", acknowledgeAgain.Code, acknowledgeAgain.Body.String())
	}

	pending := reportHTTPAs(t, router, teacher, http.MethodGet, "/api/v1/reports/daily-exceptions?date=2026-09-02", "")
	var pendingResult DailyExceptions
	decodeReportData(t, pending, &pendingResult)
	for _, item := range pendingResult.Items {
		if item.ID == exceptionID {
			t.Fatalf("acknowledged exception still in pending list: %+v", item)
		}
	}

	history := reportHTTPAs(t, router, teacher, http.MethodGet, "/api/v1/reports/daily-exceptions?date=2026-09-02&include_acknowledged=true", "")
	var historyResult DailyExceptions
	decodeReportData(t, history, &historyResult)
	var found *DailyException
	for index := range historyResult.Items {
		if historyResult.Items[index].ID == exceptionID {
			found = &historyResult.Items[index]
			break
		}
	}
	if found == nil || !found.Acknowledged || found.AcknowledgedBy == "" {
		t.Fatalf("acknowledged exception history = %+v", historyResult.Items)
	}
	entries, err := auditStore.List(ctx, masterdata.DefaultOrganizationID, auditmodule.ListFilter{Action: "exception.acknowledge", ResourceType: "daily_exception", Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want idempotent single entry", len(entries))
	}
}

func reportHTTPAs(t *testing.T, router http.Handler, principal identity.Principal, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body)).WithContext(identity.WithPrincipal(context.Background(), principal))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(record, req)
	return record
}

func decodeReportData(t *testing.T, record *httptest.ResponseRecorder, target any) {
	t.Helper()
	if record.Code < 200 || record.Code >= 300 {
		t.Fatalf("unexpected report status %d: %s", record.Code, record.Body.String())
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(record.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode report envelope: %v", err)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatalf("decode report data: %v", err)
	}
}

func reportDate(value string) time.Time {
	parsed, _ := time.ParseInLocation("2006-01-02", fmt.Sprint(value), time.UTC)
	return parsed
}
