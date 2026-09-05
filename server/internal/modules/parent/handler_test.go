package parent

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
	wechat "github.com/chenbb0128/tuoguan-system-server/internal/platform/wechat"
)

type testMiniProgramCodeGenerator struct {
	params wechat.MiniProgramCodeParams
}

func (g *testMiniProgramCodeGenerator) GenerateMiniProgramCode(_ context.Context, params wechat.MiniProgramCodeParams) ([]byte, error) {
	g.params = params
	return []byte("test-png"), nil
}

func TestParentBindingLeaveAndPickupReadModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(context.Background(), masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(context.Background(), masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026-2027 第一学期", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	schoolClass, err := master.CreateSchoolClass(context.Background(), masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	student, err := master.CreateStudent(context.Background(), masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: schoolClass.ID, Name: "小明", Gender: "male", GuardianPhone: "13800000000"})
	if err != nil {
		t.Fatal(err)
	}

	pickupStore := pickup.NewMemoryStore()
	operation, err := pickupStore.CreateOperation(context.Background(), masterdata.DefaultOrganizationID, pickup.CreateOperationParams{OperationDate: date("2026-09-01"), PickupMode: "school_pickup", SchoolID: school.ID, SchoolClassID: schoolClass.ID, TeacherName: "王老师"}, []pickup.StudentRef{{ID: student.ID, Name: student.Name}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pickupStore.SetOperationStatus(context.Background(), masterdata.DefaultOrganizationID, pickup.SetOperationStatusParams{ID: operation.ID, Status: pickup.OperationStatusStarted}); err != nil {
		t.Fatal(err)
	}
	if _, err := pickupStore.MarkOperationStudent(context.Background(), masterdata.DefaultOrganizationID, pickup.MarkStudentParams{OperationID: operation.ID, StudentID: student.ID, Status: pickup.MemberStatusPickedUp, PhotoURL: "/uploads/pickup/today.jpg", OperatorName: "王老师"}); err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(NewMemoryStore(), master, pickupStore)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	bind := parentRequest(t, router, http.MethodPost, "/api/v1/parent/bindings", `{"openid":"local-parent-1","student_id":4,"guardian_phone":"13800000000","relationship":"妈妈","is_primary":true}`, "")
	if bind.Code != http.StatusCreated {
		t.Fatalf("bind status = %d: %s", bind.Code, bind.Body.String())
	}

	me := parentRequest(t, router, http.MethodGet, "/api/v1/parent/me", "", "local-parent-1")
	var meData parentMeView
	decodeParentData(t, me, &meData)
	if len(meData.Children) != 1 || meData.Children[0].StudentName != "小明" {
		t.Fatalf("me children = %+v", meData.Children)
	}

	subscriptions := parentRequest(t, router, http.MethodPost, "/api/v1/parent/subscriptions", `{"subscriptions":[{"kind":"pickup","status":"accept","template_version":"pickup-v1"},{"kind":"meal","status":"reject","template_version":"meal-v1"}]}`, "local-parent-1")
	if subscriptions.Code != http.StatusOK {
		t.Fatalf("subscription update status = %d: %s", subscriptions.Code, subscriptions.Body.String())
	}
	var subscriptionPage struct {
		Items []subscriptionView `json:"items"`
	}
	decodeParentData(t, subscriptions, &subscriptionPage)
	if len(subscriptionPage.Items) != 2 || subscriptionPage.Items[0].Kind != MessageKindMeal || subscriptionPage.Items[1].Status != "accept" || subscriptionPage.Items[1].TemplateVersion != "pickup-v1" || subscriptionPage.Items[1].AuthorizedAt == nil {
		t.Fatalf("subscriptions = %+v", subscriptionPage.Items)
	}
	authorizedAt := subscriptionPage.Items[1].AuthorizedAt
	rejected := parentRequest(t, router, http.MethodPost, "/api/v1/parent/subscriptions", `{"subscriptions":[{"kind":"pickup","status":"reject","template_version":"pickup-v2"}]}`, "local-parent-1")
	if rejected.Code != http.StatusOK {
		t.Fatalf("subscription rejection status = %d: %s", rejected.Code, rejected.Body.String())
	}
	decodeParentData(t, rejected, &subscriptionPage)
	if len(subscriptionPage.Items) != 2 || subscriptionPage.Items[1].Status != "reject" || subscriptionPage.Items[1].TemplateVersion != "pickup-v2" || subscriptionPage.Items[1].AuthorizedAt == nil || *authorizedAt != *subscriptionPage.Items[1].AuthorizedAt {
		t.Fatalf("subscription history = %+v", subscriptionPage.Items)
	}
	duplicateKinds := parentRequest(t, router, http.MethodPost, "/api/v1/parent/subscriptions", `{"subscriptions":[{"kind":"meal","status":"accept"},{"kind":"meal","status":"reject"}]}`, "local-parent-1")
	if duplicateKinds.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate subscription kinds status = %d: %s", duplicateKinds.Code, duplicateKinds.Body.String())
	}

	leave := parentRequest(t, router, http.MethodPost, "/api/v1/parent/students/4/leave-requests", `{"leave_date":"2026-09-02","reason":"带孩子看医生"}`, "local-parent-1")
	if leave.Code != http.StatusCreated {
		t.Fatalf("leave status = %d: %s", leave.Code, leave.Body.String())
	}

	events := parentRequest(t, router, http.MethodGet, "/api/v1/parent/students/4/pickup-events?date=2026-09-01", "", "local-parent-1")
	var eventPage struct {
		Items []struct {
			PhotoURL  string `json:"photo_url"`
			EventType string `json:"event_type"`
		} `json:"items"`
		Total int `json:"total"`
	}
	decodeParentData(t, events, &eventPage)
	if eventPage.Total != 1 || eventPage.Items[0].EventType != pickup.MemberStatusPickedUp || eventPage.Items[0].PhotoURL != "/uploads/pickup/today.jpg" {
		t.Fatalf("events = %+v", eventPage)
	}

	notifications := parentRequest(t, router, http.MethodGet, "/api/v1/parent/notifications", "", "local-parent-1")
	var notificationPage struct {
		Total int `json:"total"`
	}
	decodeParentData(t, notifications, &notificationPage)
	if notificationPage.Total != 1 {
		t.Fatalf("notification total = %d, want 1", notificationPage.Total)
	}

	leaves := parentRequest(t, router, http.MethodGet, "/api/v1/parent/leave-requests", "", "local-parent-1")
	var leavePage struct {
		Items []leaveRequestView `json:"items"`
		Total int                `json:"total"`
	}
	decodeParentData(t, leaves, &leavePage)
	if leavePage.Total != 1 || leavePage.Items[0].Status != LeaveStatusPending {
		t.Fatalf("leaves = %+v", leavePage)
	}

	review := parentRequestWithHeaders(t, router, http.MethodPost, "/api/v1/leave-requests/3/review", `{"status":"approved","teacher_note":"已确认"}`, map[string]string{"X-Teacher-User-ID": "0"})
	if review.Code != http.StatusOK {
		t.Fatalf("review status = %d: %s", review.Code, review.Body.String())
	}
	leaves = parentRequest(t, router, http.MethodGet, "/api/v1/parent/leave-requests", "", "local-parent-1")
	decodeParentData(t, leaves, &leavePage)
	if leavePage.Items[0].Status != LeaveStatusApproved {
		t.Fatalf("reviewed leave = %+v", leavePage.Items[0])
	}
	parentNotifications, err := pickupStore.ListNotifications(context.Background(), masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(parentNotifications) != 2 || parentNotifications[0].Kind != "leave_review" {
		t.Fatalf("leave notifications = %+v", parentNotifications)
	}
}

func TestTeacherLeaveListAndReviewAreScopedToAssignedClass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", IsCurrent: true})
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
	parents := NewMemoryStore()
	account, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, CreateAccountParams{OpenID: "teacher-scope-parent"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := parents.CreateLeaveRequest(ctx, masterdata.DefaultOrganizationID, CreateLeaveRequestParams{StudentID: studentOne.ID, ParentAccountID: account.ID, LeaveDate: date("2026-09-02"), Reason: "发热"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := parents.CreateLeaveRequest(ctx, masterdata.DefaultOrganizationID, CreateLeaveRequestParams{StudentID: studentTwo.ID, ParentAccountID: account.ID, LeaveDate: date("2026-09-02"), Reason: "外出"})
	if err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 77, Role: identity.UserRoleTeacher}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: classOne.ID}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(parents, master, pickup.NewMemoryStore())
	handler.SetStaffScope(assignments)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	list := parentRequestAs(t, router, teacher, http.MethodGet, "/api/v1/leave-requests?date=2026-09-02&status=pending", "")
	var page struct {
		Items []leaveRequestView `json:"items"`
		Total int                `json:"total"`
	}
	decodeParentData(t, list, &page)
	if page.Total != 1 || page.Items[0].ID != first.ID || page.Items[0].StudentName != "小明" || page.Items[0].ClassName != "三年级1班" {
		t.Fatalf("teacher leave list = %+v", page)
	}

	forbidden := parentRequestAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/leave-requests/%d/review", second.ID), `{"status":"approved"}`)
	if forbidden.Code != http.StatusNotFound {
		t.Fatalf("unassigned leave review status = %d, want 404: %s", forbidden.Code, forbidden.Body.String())
	}
	approved := parentRequestAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/leave-requests/%d/review", first.ID), `{"status":"approved","teacher_note":"已确认"}`)
	if approved.Code != http.StatusOK {
		t.Fatalf("assigned leave review status = %d: %s", approved.Code, approved.Body.String())
	}
}

func TestParentPickupPlanChangeAndNotificationRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	schoolClass, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	student, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: schoolClass.ID, Name: "小明", GuardianPhone: "13800000000"})
	if err != nil {
		t.Fatal(err)
	}
	pickupStore := pickup.NewMemoryStore()
	operation, err := pickupStore.CreateOperation(ctx, masterdata.DefaultOrganizationID, pickup.CreateOperationParams{OperationDate: date("2026-09-01"), PickupMode: "school_pickup", SchoolID: school.ID, SchoolClassID: schoolClass.ID, TeacherName: "王老师"}, []pickup.StudentRef{{ID: student.ID, Name: student.Name}})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewMemoryStore(), master, pickupStore)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	bind := parentRequest(t, router, http.MethodPost, "/api/v1/parent/bindings", fmt.Sprintf(`{"openid":"parent-plan","student_id":%d,"guardian_phone":"13800000000"}`, student.ID), "")
	if bind.Code != http.StatusCreated {
		t.Fatalf("bind status = %d: %s", bind.Code, bind.Body.String())
	}
	plan := parentRequest(t, router, http.MethodGet, fmt.Sprintf("/api/v1/parent/students/%d/pickup-today?date=2026-09-01", student.ID), "", "parent-plan")
	var planData parentPickupTodayView
	decodeParentData(t, plan, &planData)
	if planData.OperationID != operation.ID || planData.TeacherName != "王老师" {
		t.Fatalf("pickup plan = %+v", planData)
	}
	change := parentRequest(t, router, http.MethodPost, fmt.Sprintf("/api/v1/parent/students/%d/pickup-changes", student.ID), `{"change_date":"2026-09-01","requested_status":"parent_picked_up","note":"今天妈妈到机构接走"}`, "parent-plan")
	if change.Code != http.StatusCreated {
		t.Fatalf("change status = %d: %s", change.Code, change.Body.String())
	}
	changes, err := pickupStore.ListPickupChangeRequests(ctx, masterdata.DefaultOrganizationID, nil, pickup.ChangeRequestStatusPending)
	if err != nil || len(changes) != 1 {
		t.Fatalf("change requests = %+v, err=%v", changes, err)
	}
	notification, err := pickupStore.CreateNotification(ctx, masterdata.DefaultOrganizationID, pickup.CreateNotificationParams{StudentID: student.ID, Title: "测试通知", Content: "请知晓"})
	if err != nil {
		t.Fatal(err)
	}
	read := parentRequest(t, router, http.MethodPost, fmt.Sprintf("/api/v1/parent/notifications/%d/read", notification.ID), `{}`, "parent-plan")
	if read.Code != http.StatusOK {
		t.Fatalf("read status = %d: %s", read.Code, read.Body.String())
	}
	items, err := pickupStore.ListNotifications(ctx, masterdata.DefaultOrganizationID)
	if err != nil || items[0].ReadAt == nil {
		t.Fatalf("notifications after read = %+v, err=%v", items, err)
	}
}

func TestTeacherCanRecordVerbalLeaveForAssignedStudent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026-2027 第一学期", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	schoolClass, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	student, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: schoolClass.ID, Name: "小明", Gender: "male"})
	if err != nil {
		t.Fatal(err)
	}
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 9, Role: identity.UserRoleTeacher}
	assignments := assignment.NewMemoryStore()
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: schoolClass.ID}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewMemoryStore(), master, pickup.NewMemoryStore())
	handler.SetStaffScope(assignments)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	created := parentRequestAs(t, router, teacher, http.MethodPost, "/api/v1/leave-requests/teacher", `{"student_id":`+fmt.Sprint(student.ID)+`,"leave_date":"2026-09-01","reason":"家长口头通知发烧"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("teacher leave status = %d: %s", created.Code, created.Body.String())
	}
	var leave leaveRequestView
	decodeParentData(t, created, &leave)
	if leave.Status != LeaveStatusApproved || leave.SubmittedByType != LeaveSubmittedByTeacher {
		t.Fatalf("teacher leave = %+v", leave)
	}

	duplicate := parentRequestAs(t, router, teacher, http.MethodPost, "/api/v1/leave-requests/teacher", `{"student_id":`+fmt.Sprint(student.ID)+`,"leave_date":"2026-09-01","reason":"重复登记"}`)
	if duplicate.Code != http.StatusCreated {
		t.Fatalf("idempotent teacher leave status = %d: %s", duplicate.Code, duplicate.Body.String())
	}
	var duplicateLeave leaveRequestView
	decodeParentData(t, duplicate, &duplicateLeave)
	if duplicateLeave.ID != leave.ID || duplicateLeave.Status != LeaveStatusApproved {
		t.Fatalf("idempotent teacher leave = %+v, original = %+v", duplicateLeave, leave)
	}

	other := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 10, Role: identity.UserRoleTeacher}
	forbidden := parentRequestAs(t, router, other, http.MethodPost, "/api/v1/leave-requests/teacher", `{"student_id":`+fmt.Sprint(student.ID)+`,"leave_date":"2026-09-02","reason":"无权限"}`)
	if forbidden.Code != http.StatusBadRequest {
		t.Fatalf("unassigned teacher status = %d: %s", forbidden.Code, forbidden.Body.String())
	}
}

func TestClassInviteQRCodeAndTokenFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026-2027 第一学期", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	schoolClass, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 21, OrganizationID: masterdata.DefaultOrganizationID, Role: identity.UserRoleTeacher}
	assignments := assignment.NewMemoryStore()
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: schoolClass.ID}); err != nil {
		t.Fatal(err)
	}
	generator := &testMiniProgramCodeGenerator{}
	handler := NewHandler(NewMemoryStore(), master, pickup.NewMemoryStore())
	handler.SetStaffScope(assignments)
	handler.SetMiniProgramCodeGenerator(generator)
	handler.SetMiniProgramCodeConfig("pages/parent/index", "trial")
	handler.SetClassInviteSecret("test-class-invite-secret")
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	qrcode := parentRequestAs(t, router, teacher, http.MethodGet, fmt.Sprintf("/api/v1/class-invites/qrcode?school_class_id=%d", schoolClass.ID), "")
	if qrcode.Code != http.StatusOK || qrcode.Header().Get("Content-Type") != "image/png" || qrcode.Body.String() != "test-png" {
		t.Fatalf("qrcode response = %d %q %q", qrcode.Code, qrcode.Header().Get("Content-Type"), qrcode.Body.String())
	}
	if generator.params.EnvVersion != "trial" || generator.params.Page != "pages/parent/index" || generator.params.Scene != handler.classInviteToken(masterdata.DefaultOrganizationID, schoolClass.ID) {
		t.Fatalf("qrcode params = %+v", generator.params)
	}

	token := handler.classInviteToken(masterdata.DefaultOrganizationID, schoolClass.ID)
	invite := parentRequestAs(t, router, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: 31, OrganizationID: masterdata.DefaultOrganizationID, Role: identity.UserRole("parent")}, http.MethodGet, "/api/v1/parent/class-invites/"+token, "")
	var inviteData classInviteView
	decodeParentData(t, invite, &inviteData)
	if inviteData.SchoolClassID != schoolClass.ID || inviteData.SchoolName != school.Name || inviteData.Label != "实验小学 · 三年级1班" {
		t.Fatalf("invite = %+v", inviteData)
	}

	invalid := parentRequestAs(t, router, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: 31, OrganizationID: masterdata.DefaultOrganizationID, Role: identity.UserRole("parent")}, http.MethodGet, "/api/v1/parent/class-invites/cinvalid.invalid", "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid invite status = %d: %s", invalid.Code, invalid.Body.String())
	}
}

func TestChildApplicationInviteReviewCreatesBindingAndPreventsCrossClassReview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
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

	parents := NewMemoryStore()
	parentAccount, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, CreateAccountParams{OpenID: "parent-application", Nickname: "小明妈妈"})
	if err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 42, Role: identity.UserRoleTeacher}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: classOne.ID}); err != nil {
		t.Fatal(err)
	}
	pickupStore := pickup.NewMemoryStore()
	handler := NewHandler(parents, master, pickupStore)
	handler.SetStaffScope(assignments)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	created := parentRequest(t, router, http.MethodPost, "/api/v1/parent/child-applications", fmt.Sprintf(`{"student_name":"小明","school_class_id":%d,"guardian_phone":"13800000000","relationship":"妈妈"}`, classOne.ID), parentAccount.OpenID)
	if created.Code != http.StatusCreated {
		t.Fatalf("create application status = %d: %s", created.Code, created.Body.String())
	}
	var application childApplicationView
	decodeParentData(t, created, &application)
	if application.Status != ChildApplicationStatusPending || application.SchoolClassID == nil || *application.SchoolClassID != classOne.ID {
		t.Fatalf("created application = %+v", application)
	}

	parentList := parentRequest(t, router, http.MethodGet, "/api/v1/parent/child-applications", "", parentAccount.OpenID)
	var parentPage listResponse[childApplicationView]
	decodeParentData(t, parentList, &parentPage)
	if parentPage.Total != 1 || parentPage.Items[0].Status != ChildApplicationStatusPending {
		t.Fatalf("parent applications = %+v", parentPage)
	}

	staffList := parentRequestAs(t, router, teacher, http.MethodGet, "/api/v1/child-applications", "")
	var staffPage listResponse[childApplicationView]
	decodeParentData(t, staffList, &staffPage)
	if staffPage.Total != 1 || staffPage.Items[0].ID != application.ID {
		t.Fatalf("teacher applications = %+v", staffPage)
	}

	needsInfo := parentRequestAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/child-applications/%d/review", application.ID), `{"status":"needs_info","review_note":"请补充接送说明"}`)
	if needsInfo.Code != http.StatusOK {
		t.Fatalf("needs info status = %d: %s", needsInfo.Code, needsInfo.Body.String())
	}
	decodeParentData(t, needsInfo, &application)
	if application.Status != ChildApplicationStatusNeedsInfo || application.ReviewNote != "请补充接送说明" {
		t.Fatalf("needs info application = %+v", application)
	}

	resubmitted := parentRequest(t, router, http.MethodPut, fmt.Sprintf("/api/v1/parent/child-applications/%d", application.ID), fmt.Sprintf(`{"student_name":"小明","school_class_id":%d,"guardian_phone":"13800000000","notes":"已补充接送说明"}`, classOne.ID), parentAccount.OpenID)
	if resubmitted.Code != http.StatusOK {
		t.Fatalf("resubmit application status = %d: %s", resubmitted.Code, resubmitted.Body.String())
	}
	decodeParentData(t, resubmitted, &application)
	if application.Status != ChildApplicationStatusPending || application.Notes != "已补充接送说明" || application.ReviewNote != "" {
		t.Fatalf("resubmitted application = %+v", application)
	}

	review := parentRequestAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/child-applications/%d/review", application.ID), `{"status":"approved"}`)
	if review.Code != http.StatusOK {
		t.Fatalf("review application status = %d: %s", review.Code, review.Body.String())
	}
	decodeParentData(t, review, &application)
	if application.Status != ChildApplicationStatusApproved || application.StudentID == nil {
		t.Fatalf("reviewed application = %+v", application)
	}
	students, err := master.ListStudents(ctx, masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(students) != 1 || students[0].SchoolClassID != classOne.ID || students[0].GuardianPhone != "13800000000" {
		t.Fatalf("students after review = %+v", students)
	}

	me := parentRequest(t, router, http.MethodGet, "/api/v1/parent/me", "", parentAccount.OpenID)
	var parentMe parentMeView
	decodeParentData(t, me, &parentMe)
	if len(parentMe.Children) != 1 || parentMe.Children[0].StudentID != *application.StudentID || parentMe.Children[0].StudentName != "小明" || parentMe.Children[0].SchoolName != "实验小学" || parentMe.Children[0].Grade != "三年级" || parentMe.Children[0].ClassName != "1班" {
		t.Fatalf("parent children after review = %+v", parentMe.Children)
	}
	notifications, err := pickupStore.ListNotifications(ctx, masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(notifications) != 1 || notifications[0].Kind != "child_application_approved" {
		t.Fatalf("application notifications = %+v", notifications)
	}

	duplicate := parentRequest(t, router, http.MethodPost, "/api/v1/parent/child-applications", fmt.Sprintf(`{"student_name":"小明","school_class_id":%d,"guardian_phone":"13800000000"}`, classOne.ID), parentAccount.OpenID)
	if duplicate.Code != http.StatusBadRequest {
		t.Fatalf("duplicate application status = %d: %s", duplicate.Code, duplicate.Body.String())
	}

	other := parentRequest(t, router, http.MethodPost, "/api/v1/parent/child-applications", fmt.Sprintf(`{"student_name":"小红","school_class_id":%d,"guardian_phone":"13900000000"}`, classTwo.ID), parentAccount.OpenID)
	if other.Code != http.StatusCreated {
		t.Fatalf("other-class application status = %d: %s", other.Code, other.Body.String())
	}
	staffList = parentRequestAs(t, router, teacher, http.MethodGet, "/api/v1/child-applications", "")
	decodeParentData(t, staffList, &staffPage)
	if staffPage.Total != 1 {
		t.Fatalf("teacher cross-class list = %+v", staffPage)
	}
	var otherApplication childApplicationView
	decodeParentData(t, other, &otherApplication)
	forbidden := parentRequestAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/child-applications/%d/review", otherApplication.ID), `{"status":"rejected","review_note":"不属于本班"}`)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("cross-class review status = %d: %s", forbidden.Code, forbidden.Body.String())
	}
}

func TestTeacherCanCreateClassFromUnmatchedApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026-2027 第一学期", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}

	parents := NewMemoryStore()
	parentAccount, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, CreateAccountParams{OpenID: "parent-unmatched", Nickname: "小明妈妈"})
	if err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 42, Role: identity.UserRoleTeacher}
	assignedClass, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "二年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: assignedClass.ID}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(parents, master, pickup.NewMemoryStore())
	handler.SetStaffScope(assignments)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	created := parentRequest(t, router, http.MethodPost, "/api/v1/parent/child-applications", `{"student_name":"小明","school_name":"实验小学","class_text":"1年纪二班","guardian_phone":"13800000000","relationship":"妈妈"}`, parentAccount.OpenID)
	if created.Code != http.StatusCreated {
		t.Fatalf("create unmatched application status = %d: %s", created.Code, created.Body.String())
	}
	var application childApplicationView
	decodeParentData(t, created, &application)
	if application.SchoolClassID != nil || application.SchoolID == nil {
		t.Fatalf("unmatched application = %+v", application)
	}

	staffList := parentRequestAs(t, router, teacher, http.MethodGet, "/api/v1/child-applications", "")
	var staffPage listResponse[childApplicationView]
	decodeParentData(t, staffList, &staffPage)
	if staffPage.Total != 1 || staffPage.Items[0].ID != application.ID {
		t.Fatalf("teacher unmatched intake = %+v", staffPage)
	}

	review := parentRequestAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/child-applications/%d/review", application.ID), `{"status":"approved","create_school_class":true}`)
	if review.Code != http.StatusOK {
		t.Fatalf("create class review status = %d: %s", review.Code, review.Body.String())
	}
	decodeParentData(t, review, &application)
	if application.Status != ChildApplicationStatusApproved || application.StudentID == nil || application.SchoolClassID == nil {
		t.Fatalf("approved unmatched application = %+v", application)
	}
	classes, err := master.ListSchoolClasses(ctx, masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	var createdClass masterdata.SchoolClass
	for _, item := range classes {
		if item.Grade == "一年级" && item.Name == "2班" {
			createdClass = item
			break
		}
	}
	if len(classes) != 2 || createdClass.ID == 0 {
		t.Fatalf("created class = %+v", classes)
	}
	assigned, err := assignments.List(ctx, masterdata.DefaultOrganizationID, teacher.SubjectID, 0)
	if err != nil {
		t.Fatal(err)
	}
	foundCreatedAssignment := false
	for _, item := range assigned {
		if item.SchoolClassID == createdClass.ID {
			foundCreatedAssignment = true
			break
		}
	}
	if len(assigned) != 2 || !foundCreatedAssignment {
		t.Fatalf("teacher assignment after review = %+v", assigned)
	}
}

func TestUnmatchedClassTextDoesNotBlockApproval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026-2027 第一学期", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	parents := NewMemoryStore()
	parentAccount, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, CreateAccountParams{OpenID: "parent-raw-class", Nickname: "家长"})
	if err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 43, Role: identity.UserRoleTeacher}
	assignedClass, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "二年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: assignedClass.ID}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(parents, master, pickup.NewMemoryStore())
	handler.SetStaffScope(assignments)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	created := parentRequest(t, router, http.MethodPost, "/api/v1/parent/child-applications", `{"student_name":"小雨","school_name":"实验小学","class_text":"家长填写的班级","guardian_phone":"13800000000"}`, parentAccount.OpenID)
	if created.Code != http.StatusCreated {
		t.Fatalf("create raw class application status = %d: %s", created.Code, created.Body.String())
	}
	var application childApplicationView
	decodeParentData(t, created, &application)
	if application.ClassNameInput != "家长填写的班级" {
		t.Fatalf("raw class input = %+v", application)
	}

	review := parentRequestAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/child-applications/%d/review", application.ID), `{"status":"approved","create_school_class":true}`)
	if review.Code != http.StatusOK {
		t.Fatalf("raw class review status = %d: %s", review.Code, review.Body.String())
	}
	decodeParentData(t, review, &application)
	if application.Status != ChildApplicationStatusApproved || application.SchoolClassID == nil {
		t.Fatalf("approved raw class application = %+v", application)
	}
}

func TestAdminCanApproveMinimalChildApplicationByCreatingFallbackClass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	parents := NewMemoryStore()
	parentAccount, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, CreateAccountParams{OpenID: "parent-minimal-application", Nickname: "家长"})
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(parents, master, pickup.NewMemoryStore())
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	created := parentRequest(t, router, http.MethodPost, "/api/v1/parent/child-applications", `{"student_name":"小星","grade":"一年级","guardian_phone":"15888231457","notes":"家长只填写年级，学校和班级由老师确认"}`, parentAccount.OpenID)
	if created.Code != http.StatusCreated {
		t.Fatalf("create minimal application status = %d: %s", created.Code, created.Body.String())
	}
	var application childApplicationView
	decodeParentData(t, created, &application)
	if application.SchoolID != nil || application.SchoolClassID != nil {
		t.Fatalf("minimal application should stay unresolved before review = %+v", application)
	}

	admin := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 1, Role: identity.UserRoleAdmin}
	review := parentRequestAs(t, router, admin, http.MethodPost, fmt.Sprintf("/api/v1/child-applications/%d/review", application.ID), `{"status":"approved","create_school_class":true}`)
	if review.Code != http.StatusOK {
		t.Fatalf("approve minimal application status = %d: %s", review.Code, review.Body.String())
	}
	decodeParentData(t, review, &application)
	if application.Status != ChildApplicationStatusApproved || application.StudentID == nil || application.SchoolID == nil || application.SchoolClassID == nil {
		t.Fatalf("approved minimal application = %+v", application)
	}

	schools, err := master.ListSchools(ctx, masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(schools) != 1 || schools[0].Name != "待确认学校" {
		t.Fatalf("fallback school = %+v", schools)
	}
	classes, err := master.ListSchoolClasses(ctx, masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 1 || classes[0].Grade != "一年级" || classes[0].Name != "待确认班级" {
		t.Fatalf("fallback class = %+v", classes)
	}
	students, err := master.ListStudents(ctx, masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(students) != 1 || students[0].Name != "小星" || students[0].GuardianPhone != "15888231457" {
		t.Fatalf("created student = %+v", students)
	}
}

func TestSameNameApplicationReturnsCandidatesAndRequiresSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	schoolClass, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "三年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: schoolClass.ID, Name: "小明", GuardianPhone: "13800000001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: schoolClass.ID, Name: "小明", GuardianPhone: "13800000002"}); err != nil {
		t.Fatal(err)
	}
	parents := NewMemoryStore()
	parentAccount, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, CreateAccountParams{OpenID: "parent-same-name", Nickname: "小明家长"})
	if err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 44, Role: identity.UserRoleTeacher}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: schoolClass.ID}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(parents, master, pickup.NewMemoryStore())
	handler.SetStaffScope(assignments)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	created := parentRequest(t, router, http.MethodPost, "/api/v1/parent/child-applications", fmt.Sprintf(`{"student_name":"小明","school_class_id":%d,"guardian_phone":"13900000000"}`, schoolClass.ID), parentAccount.OpenID)
	if created.Code != http.StatusCreated {
		t.Fatalf("create application status = %d: %s", created.Code, created.Body.String())
	}
	var application childApplicationView
	decodeParentData(t, created, &application)

	staffList := parentRequestAs(t, router, teacher, http.MethodGet, "/api/v1/child-applications", "")
	var page listResponse[childApplicationView]
	decodeParentData(t, staffList, &page)
	if page.Total != 1 || len(page.Items[0].StudentMatches) != 2 {
		t.Fatalf("same-name candidates = %+v", page.Items)
	}

	withoutSelection := parentRequestAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/child-applications/%d/review", application.ID), `{"status":"approved"}`)
	if withoutSelection.Code != http.StatusBadRequest {
		t.Fatalf("same-name review without selection status = %d: %s", withoutSelection.Code, withoutSelection.Body.String())
	}
	review := parentRequestAs(t, router, teacher, http.MethodPost, fmt.Sprintf("/api/v1/child-applications/%d/review", application.ID), fmt.Sprintf(`{"status":"approved","student_id":%d}`, first.ID))
	if review.Code != http.StatusOK {
		t.Fatalf("same-name review status = %d: %s", review.Code, review.Body.String())
	}
}

func TestParseClassTextNormalizesCommonParentInput(t *testing.T) {
	tests := []struct {
		input, grade, className string
	}{
		{input: "一年级2班", grade: "一年级", className: "2班"},
		{input: "1年级2班", grade: "一年级", className: "2班"},
		{input: "1年纪二班", grade: "一年级", className: "2班"},
		{input: "一（2）班", grade: "一年级", className: "2班"},
	}
	for _, test := range tests {
		grade, className := parseClassText(test.input)
		if grade != test.grade || className != test.className {
			t.Errorf("parseClassText(%q) = %q, %q; want %q, %q", test.input, grade, className, test.grade, test.className)
		}
	}
}

func parentRequest(t *testing.T, router http.Handler, method, path, body, openID string) *httptest.ResponseRecorder {
	return parentRequestWithHeaders(t, router, method, path, body, map[string]string{ParentOpenIDHeader: openID})
}

func parentRequestWithHeaders(t *testing.T, router http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		if value != "" {
			req.Header.Set(key, value)
		}
	}
	router.ServeHTTP(record, req)
	return record
}

func parentRequestAs(t *testing.T, router http.Handler, principal identity.Principal, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body)).WithContext(identity.WithPrincipal(context.Background(), principal))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(record, req)
	return record
}

func decodeParentData(t *testing.T, record *httptest.ResponseRecorder, target any) {
	t.Helper()
	if record.Code < 200 || record.Code >= 300 {
		t.Fatalf("unexpected status %d: %s", record.Code, record.Body.String())
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(record.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatalf("decode data: %v", err)
	}
}

func date(value string) time.Time {
	parsed, _ := time.ParseInLocation("2006-01-02", value, time.UTC)
	return parsed
}
