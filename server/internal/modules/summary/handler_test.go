package summary

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
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
)

func TestSummaryHTTPWorkflowAndScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{
		Name: "2026 秋季", StartsOn: testDate("2026-09-01"), EndsOn: testDate("2027-01-31"), IsCurrent: true,
	})
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
	studentOne, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: classOne.ID, Name: "小明", Gender: "male"})
	if err != nil {
		t.Fatal(err)
	}
	studentTwo, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: classTwo.ID, Name: "小雨", Gender: "female"})
	if err != nil {
		t.Fatal(err)
	}

	assignments := assignment.NewMemoryStore()
	teacherOne := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 41, Role: identity.UserRoleTeacher}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacherOne.SubjectID, SchoolClassID: classOne.ID}); err != nil {
		t.Fatal(err)
	}
	teacherTwo := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 42, Role: identity.UserRoleTeacher}
	admin := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 7, Role: identity.UserRoleAdmin}

	parents := parent.NewMemoryStore()
	parentOne, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, parent.CreateAccountParams{OpenID: "summary-parent-one", Nickname: "小明家长"})
	if err != nil {
		t.Fatal(err)
	}
	parentTwo, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, parent.CreateAccountParams{OpenID: "summary-parent-two", Nickname: "小雨家长"})
	if err != nil {
		t.Fatal(err)
	}
	unboundParent, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, parent.CreateAccountParams{OpenID: "summary-parent-unbound", Nickname: "未绑定家长"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parents.CreateBinding(ctx, masterdata.DefaultOrganizationID, parent.BindStudentParams{ParentAccountID: parentOne.ID, StudentID: studentOne.ID, Relationship: "妈妈", IsPrimary: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := parents.CreateBinding(ctx, masterdata.DefaultOrganizationID, parent.BindStudentParams{ParentAccountID: parentTwo.ID, StudentID: studentTwo.ID, Relationship: "爸爸", IsPrimary: true}); err != nil {
		t.Fatal(err)
	}

	store := NewMemoryStore()
	handler := NewHandler(store, pickup.NewMemoryStore(), nil, nil, master)
	handler.SetStaffScope(assignments)
	handler.SetParentStore(parents)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterStaffRoutes(api)
	handler.RegisterParentRoutes(api)

	generated := summaryHTTPAs(t, router, admin, http.MethodPost, "/api/v1/daily-summaries/generate", `{"summary_date":"2026-09-02"}`)
	var current view
	decodeSummaryData(t, generated, &current)
	if current.Status != StatusDraft || current.Version != 1 {
		t.Fatalf("generated summary = %+v", current)
	}

	updated := summaryHTTPAs(t, router, admin, http.MethodPut, fmt.Sprintf("/api/v1/daily-summaries/%d", current.ID), fmt.Sprintf(`{"content":"今日托管正常","child_updates":{"%d":"作业已完成","%d":"状态良好"}}`, studentOne.ID, studentTwo.ID))
	decodeSummaryData(t, updated, &current)
	if current.Version != 2 || current.ChildUpdates[studentOne.ID] != "作业已完成" || current.ChildUpdates[studentTwo.ID] != "状态良好" {
		t.Fatalf("updated summary = %+v", current)
	}

	published := summaryHTTPAs(t, router, admin, http.MethodPost, fmt.Sprintf("/api/v1/daily-summaries/%d/publish", current.ID), `{}`)
	decodeSummaryData(t, published, &current)
	if current.Status != StatusPublished {
		t.Fatalf("published summary = %+v", current)
	}

	parentOneSummary := summaryHTTPAs(t, router, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: parentOne.ID}, http.MethodGet, "/api/v1/parent/daily-summary?date=2026-09-02", "")
	var parentView view
	decodeSummaryData(t, parentOneSummary, &parentView)
	if parentView.ChildUpdates[studentOne.ID] != "作业已完成" || len(parentView.ChildUpdates) != 1 {
		t.Fatalf("parent one child updates = %+v", parentView.ChildUpdates)
	}
	parentTwoSummary := summaryHTTPAs(t, router, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: parentTwo.ID}, http.MethodGet, "/api/v1/parent/daily-summary?date=2026-09-02", "")
	parentView = view{}
	decodeSummaryData(t, parentTwoSummary, &parentView)
	if parentView.ChildUpdates[studentTwo.ID] != "状态良好" || len(parentView.ChildUpdates) != 1 {
		t.Fatalf("parent two child updates = %+v", parentView.ChildUpdates)
	}
	unbound := summaryHTTPAs(t, router, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: unboundParent.ID}, http.MethodGet, "/api/v1/parent/daily-summary?date=2026-09-02", "")
	if unbound.Code != http.StatusNotFound {
		t.Fatalf("unbound parent summary status = %d: %s", unbound.Code, unbound.Body.String())
	}

	read := summaryHTTPAs(t, router, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: parentOne.ID}, http.MethodPost, fmt.Sprintf("/api/v1/parent/daily-summary/%d/read", current.ID), `{}`)
	if read.Code != http.StatusOK {
		t.Fatalf("parent read status = %d: %s", read.Code, read.Body.String())
	}
	withdrawn := summaryHTTPAs(t, router, admin, http.MethodPost, fmt.Sprintf("/api/v1/daily-summaries/%d/withdraw", current.ID), `{"reason":"需要补充现场情况"}`)
	decodeSummaryData(t, withdrawn, &current)
	if current.Status != StatusWithdrawn {
		t.Fatalf("withdrawn summary = %+v", current)
	}

	corrected := summaryHTTPAs(t, router, admin, http.MethodPost, fmt.Sprintf("/api/v1/daily-summaries/%d/correct", current.ID), fmt.Sprintf(`{"content":"补充后的总结","child_updates":{"%d":"已补充作业反馈"},"reason":"补充孩子作业反馈"}`, studentOne.ID))
	decodeSummaryData(t, corrected, &current)
	if current.Status != StatusPublished || current.Version != 3 || current.ChildUpdates[studentOne.ID] != "已补充作业反馈" {
		t.Fatalf("corrected summary = %+v", current)
	}
	correctedParentView := summaryHTTPAs(t, router, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: parentOne.ID}, http.MethodGet, "/api/v1/parent/daily-summary?date=2026-09-02", "")
	parentView = view{}
	decodeSummaryData(t, correctedParentView, &parentView)
	if parentView.ReadAt != nil {
		t.Fatalf("read state after correction = %v, want unread", parentView.ReadAt)
	}
	otherParentRead := summaryHTTPAs(t, router, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: parentTwo.ID}, http.MethodPost, fmt.Sprintf("/api/v1/parent/daily-summary/%d/read", current.ID), `{}`)
	if otherParentRead.Code != http.StatusNotFound {
		t.Fatalf("other parent read status = %d: %s", otherParentRead.Code, otherParentRead.Body.String())
	}

	closed := summaryHTTPAs(t, router, admin, http.MethodPost, fmt.Sprintf("/api/v1/daily-summaries/%d/close", current.ID), `{}`)
	decodeSummaryData(t, closed, &current)
	if current.Status != StatusClosed {
		t.Fatalf("closed summary = %+v", current)
	}
	versions := summaryHTTPAs(t, router, admin, http.MethodGet, fmt.Sprintf("/api/v1/daily-summaries/%d/versions", current.ID), "")
	var versionPage struct {
		Items []versionView `json:"items"`
		Total int           `json:"total"`
	}
	decodeSummaryData(t, versions, &versionPage)
	if versionPage.Total != 6 {
		t.Fatalf("summary versions = %+v", versionPage)
	}

	teacherVersions := summaryHTTPAs(t, router, teacherOne, http.MethodGet, fmt.Sprintf("/api/v1/daily-summaries/%d/versions", current.ID), "")
	if teacherVersions.Code != http.StatusOK {
		t.Fatalf("assigned teacher versions status = %d: %s", teacherVersions.Code, teacherVersions.Body.String())
	}
	unassignedTeacherVersions := summaryHTTPAs(t, router, teacherTwo, http.MethodGet, fmt.Sprintf("/api/v1/daily-summaries/%d/versions", current.ID), "")
	if unassignedTeacherVersions.Code != http.StatusForbidden {
		t.Fatalf("unassigned teacher versions status = %d: %s", unassignedTeacherVersions.Code, unassignedTeacherVersions.Body.String())
	}
}

func TestSummaryNotificationsScopeToTeacherClass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: testDate("2026-09-01"), EndsOn: testDate("2027-01-31"), IsCurrent: true})
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
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 51, Role: identity.UserRoleTeacher}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: classOne.ID}); err != nil {
		t.Fatal(err)
	}
	pickupStore := pickup.NewMemoryStore()
	handler := NewHandler(NewMemoryStore(), pickupStore, nil, nil, master)
	handler.SetStaffScope(assignments)
	handler.SetNotificationWriter(pickupStore)
	router := gin.New()
	handler.RegisterStaffRoutes(router.Group("/api/v1"))

	generated := summaryHTTPAs(t, router, teacher, http.MethodPost, "/api/v1/daily-summaries/generate", `{"summary_date":"2026-09-02"}`)
	var current view
	decodeSummaryData(t, generated, &current)
	published := summaryHTTPAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/daily-summaries/%d/publish", current.ID), `{}`)
	if published.Code != http.StatusOK {
		t.Fatalf("publish summary status = %d: %s", published.Code, published.Body.String())
	}
	notifications, err := pickupStore.ListNotifications(ctx, masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notifications) != 1 || notifications[0].StudentID != studentOne.ID || notifications[0].StudentID == studentTwo.ID {
		t.Fatalf("teacher summary notifications = %+v", notifications)
	}
}

func summaryHTTPAs(t *testing.T, router http.Handler, principal identity.Principal, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body)).WithContext(identity.WithPrincipal(context.Background(), principal))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(record, req)
	return record
}

func decodeSummaryData(t *testing.T, record *httptest.ResponseRecorder, target any) {
	t.Helper()
	if record.Code < 200 || record.Code >= 300 {
		t.Fatalf("unexpected status %d: %s", record.Code, record.Body.String())
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(record.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode summary envelope: %v", err)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatalf("decode summary data: %v", err)
	}
}

func testDate(value string) time.Time {
	parsed, _ := time.ParseInLocation("2006-01-02", value, time.UTC)
	return parsed
}
