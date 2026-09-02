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
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"

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
	admin := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 1, Role: identity.UserRoleAdmin}
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

	adminResponse := reportHTTPAs(t, router, admin, http.MethodGet, "/api/v1/reports/daily-overview?date=2026-09-02", "")
	var adminOverview DailyOverview
	decodeReportData(t, adminResponse, &adminOverview)
	if adminOverview.Pickup.Operations != 2 || len(adminOverview.Classes) != 2 {
		t.Fatalf("admin overview = %+v", adminOverview)
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
