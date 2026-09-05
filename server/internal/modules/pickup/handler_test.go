package pickup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/storage"
)

func TestHandlerPickupLifecycleRequiresPhotoAtSchoolGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	photoStore := newTestPhotoStore(t)
	handler := NewHandler(NewMemoryStore(), master, photoStore)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	operation := requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":3,"teacher_name":"王老师"}`, http.StatusCreated)
	var created operationView
	decodeData(t, operation, &created)
	if created.Status != OperationStatusDraft {
		t.Fatalf("created status = %q, want draft", created.Status)
	}

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/start", "", http.StatusOK)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/status", `{"status":"picked_up"}`, http.StatusUnprocessableEntity)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/status", `{"status":"picked_up","photo_url":"/uploads/pickup/gate.jpg","operator_name":"王老师"}`, http.StatusOK)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/status", `{"status":"picked_up","photo_url":"/uploads/pickup/gate-2.jpg","operator_name":"王老师"}`, http.StatusOK)

	events := requestJSON(t, router, http.MethodGet, "/api/v1/pickup-operations/1/events", "")
	var eventPage struct {
		Items []eventView `json:"items"`
		Total int         `json:"total"`
	}
	decodeData(t, events, &eventPage)
	if eventPage.Total != 2 || eventPage.Items[0].EventType != MemberStatusPickedUp {
		t.Fatalf("events = %+v, want original event plus photo supplement", eventPage)
	}
	if eventPage.Items[0].PhotoURL != "/uploads/pickup/gate-2.jpg" {
		t.Fatalf("event photo = %q, want replacement photo", eventPage.Items[0].PhotoURL)
	}
	students := requestJSON(t, router, http.MethodGet, "/api/v1/pickup-operations/1/students", "", http.StatusOK)
	var studentPage struct {
		Items []operationStudentView `json:"items"`
	}
	decodeData(t, students, &studentPage)
	if len(studentPage.Items) != 1 || studentPage.Items[0].PhotoURL != "/uploads/pickup/gate-2.jpg" {
		t.Fatalf("student photo = %+v, want replacement photo", studentPage.Items)
	}

	notifications := requestJSON(t, router, http.MethodGet, "/api/v1/notifications", "")
	var notificationPage struct {
		Total int `json:"total"`
	}
	decodeData(t, notifications, &notificationPage)
	if notificationPage.Total != 1 {
		t.Fatalf("notification total = %d, want 1", notificationPage.Total)
	}

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/status", `{"status":"arrived"}`, http.StatusOK)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/status", `{"status":"left","note":"家长按时接走"}`, http.StatusOK)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/finish", "", http.StatusOK)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/status", `{"status":"leave"}`, http.StatusBadRequest)
}

func TestHandlerCloseCheckSeparatesBlockersAndWarnings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	store := NewMemoryStore()
	handler := NewHandler(store, master, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":3}`, http.StatusCreated)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/start", "", http.StatusOK)

	check := requestJSON(t, router, http.MethodGet, "/api/v1/pickup-operations/1/close-check", "", http.StatusOK)
	var first struct {
		CanFinish bool             `json:"can_finish"`
		Blockers  []workbenchAlert `json:"blockers"`
		Warnings  []workbenchAlert `json:"warnings"`
	}
	decodeData(t, check, &first)
	if first.CanFinish || len(first.Blockers) != 1 || first.Blockers[0].Kind != "student_pending" || len(first.Warnings) != 0 {
		t.Fatalf("initial close check = %+v", first)
	}

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/status", `{"status":"picked_up","photo_url":"/uploads/pickup/4.jpg"}`, http.StatusOK)
	check = requestJSON(t, router, http.MethodGet, "/api/v1/pickup-operations/1/close-check", "", http.StatusOK)
	var afterPickup struct {
		CanFinish bool             `json:"can_finish"`
		Blockers  []workbenchAlert `json:"blockers"`
	}
	decodeData(t, check, &afterPickup)
	if afterPickup.CanFinish || len(afterPickup.Blockers) != 1 || afterPickup.Blockers[0].Kind != "student_pending" {
		t.Fatalf("picked-up close check = %+v", afterPickup)
	}

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/status", `{"status":"arrived"}`, http.StatusOK)
	check = requestJSON(t, router, http.MethodGet, "/api/v1/pickup-operations/1/close-check", "", http.StatusOK)
	var ready struct {
		CanFinish bool             `json:"can_finish"`
		Blockers  []workbenchAlert `json:"blockers"`
		Warnings  []workbenchAlert `json:"warnings"`
	}
	decodeData(t, check, &ready)
	if !ready.CanFinish || len(ready.Blockers) != 0 || len(ready.Warnings) != 0 {
		t.Fatalf("ready close check = %+v", ready)
	}

	createdSecond := requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-02","school_class_id":3}`, http.StatusCreated)
	var secondOperation operationView
	decodeData(t, createdSecond, &secondOperation)
	secondOperationPath := fmt.Sprintf("/api/v1/pickup-operations/%d", secondOperation.ID)
	requestJSON(t, router, http.MethodPost, secondOperationPath+"/start", "", http.StatusOK)
	requestJSON(t, router, http.MethodPost, secondOperationPath+"/students/4/status", `{"status":"absent","note":"已联系家长确认"}`, http.StatusOK)
	check = requestJSON(t, router, http.MethodGet, secondOperationPath+"/close-check", "", http.StatusOK)
	var warning struct {
		CanFinish bool             `json:"can_finish"`
		Blockers  []workbenchAlert `json:"blockers"`
		Warnings  []workbenchAlert `json:"warnings"`
	}
	decodeData(t, check, &warning)
	if !warning.CanFinish || len(warning.Blockers) != 0 || len(warning.Warnings) != 1 || warning.Warnings[0].Kind != "exception" {
		t.Fatalf("warning close check = %+v", warning)
	}
}

func TestHandlerBulkArriveIsAtomicAndIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	store := NewMemoryStore()
	handler := NewHandler(store, master, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-04","school_class_id":3}`, http.StatusCreated)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/start", "", http.StatusOK)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/bulk-arrive", `{"student_ids":[4]}`, http.StatusBadRequest)

	students := requestJSON(t, router, http.MethodGet, "/api/v1/pickup-operations/1/students", "", http.StatusOK)
	var before struct {
		Items []operationStudentView `json:"items"`
	}
	decodeData(t, students, &before)
	if len(before.Items) != 1 || before.Items[0].Status != MemberStatusPlanned {
		t.Fatalf("failed bulk request changed member = %+v", before.Items)
	}

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/status", `{"status":"picked_up","photo_url":"/uploads/pickup/4.jpg"}`, http.StatusOK)
	bulk := requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/bulk-arrive", `{"student_ids":[4,4],"note":"全员到班核对"}`, http.StatusOK)
	var page struct {
		Items []operationStudentView `json:"items"`
	}
	decodeData(t, bulk, &page)
	if len(page.Items) != 1 || page.Items[0].Status != MemberStatusArrived {
		t.Fatalf("bulk result = %+v", page.Items)
	}

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/bulk-arrive", `{"student_ids":[4]}`, http.StatusOK)
	notifications := requestJSON(t, router, http.MethodGet, "/api/v1/notifications", "", http.StatusOK)
	var notificationPage struct {
		Items []notificationView `json:"items"`
	}
	decodeData(t, notifications, &notificationPage)
	if len(notificationPage.Items) != 2 {
		t.Fatalf("idempotent bulk notification count = %d, want 2", len(notificationPage.Items))
	}
	if notificationPage.Items[0].Title != "孩子已确认到班" {
		t.Fatalf("bulk notification title = %q, want arrival title", notificationPage.Items[0].Title)
	}
}

func TestHandlerBatchNotificationsRespectStudentScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	store := NewMemoryStore()
	handler := NewHandler(store, master, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	created := requestJSON(t, router, http.MethodPost, "/api/v1/notifications/batch", `{"student_ids":[4],"title":"明日提醒","content":"请带好水杯"}`, http.StatusOK)
	var page struct {
		Items []batchNotificationView `json:"items"`
		Total int                     `json:"total"`
	}
	decodeData(t, created, &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].StudentID != 4 {
		t.Fatalf("batch notification result = %+v", page)
	}
	notifications := requestJSON(t, router, http.MethodGet, "/api/v1/notifications", "", http.StatusOK)
	var notificationPage struct {
		Items []notificationView `json:"items"`
	}
	decodeData(t, notifications, &notificationPage)
	if len(notificationPage.Items) != 1 || notificationPage.Items[0].Title != "明日提醒" {
		t.Fatalf("notifications = %+v", notificationPage.Items)
	}

	assignments := assignment.NewMemoryStore()
	users := identity.NewMemoryStore()
	teacher, err := users.CreateUser(context.Background(), identity.CreateUserParams{Username: "teacher-batch", Nickname: "王老师", PasswordHash: "test-hash", Role: identity.UserRoleTeacher, Status: identity.UserStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assignments.Create(context.Background(), masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.ID, SchoolClassID: 999}); err != nil {
		t.Fatal(err)
	}
	scopedHandler := NewHandler(NewMemoryStore(), master, nil)
	scopedHandler.SetStaffScope(assignments, users)
	scopedRouter := gin.New()
	scopedHandler.RegisterRoutes(scopedRouter.Group("/api/v1"))
	requestJSONAs(t, scopedRouter, identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: teacher.ID, Role: identity.UserRoleTeacher}, http.MethodPost, "/api/v1/notifications/batch", `{"student_ids":[4],"title":"不应发送","content":"无权限"}`, http.StatusBadRequest)
}

func TestHandlerPhotoUploadReturnsImageFormatMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	handler := NewHandler(NewMemoryStore(), master, newTestPhotoStore(t))
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("not an image")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	record := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/pickup", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(record, req)
	if record.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("upload status = %d, want %d: %s", record.Code, http.StatusUnsupportedMediaType, record.Body.String())
	}
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(record.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 10002 || response.Message != "仅支持 JPEG、PNG 或 WebP 图片" {
		t.Fatalf("upload error = %+v, want code 10002 and image format message", response)
	}
}

func TestHandlerSupportsAuditedPickupHandover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	handler := NewHandler(NewMemoryStore(), master, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-02","school_class_id":3,"teacher_name":"王老师"}`, http.StatusCreated)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/confirm", `{}`, http.StatusOK)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/start", "", http.StatusOK)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/handover", `{"to_teacher_user_id":9,"to_teacher_name":"李老师","teacher_role":"collaborator","note":"校门口交接"}`, http.StatusOK)
	handoffs := requestJSON(t, router, http.MethodGet, "/api/v1/pickup-operations/1/handoffs", "", http.StatusOK)
	var page struct {
		Items []handoffView `json:"items"`
	}
	decodeData(t, handoffs, &page)
	if len(page.Items) != 1 || page.Items[0].ToTeacherName != "李老师" || page.Items[0].Note != "校门口交接" {
		t.Fatalf("handoffs = %+v", page.Items)
	}
}

func TestHandoffTeacherListIsScopedToAssignedClass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	users := identity.NewMemoryStore()
	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	first, err := users.CreateUser(ctx, identity.CreateUserParams{Username: "teacher-one", PasswordHash: string(hash), Role: identity.UserRoleTeacher, Nickname: "王老师", Status: identity.UserStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	second, err := users.CreateUser(ctx, identity.CreateUserParams{Username: "teacher-two", PasswordHash: string(hash), Role: identity.UserRoleTeacher, Nickname: "李老师", Status: identity.UserStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	for _, teacherID := range []uint64{first.ID, second.ID} {
		if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacherID, SchoolClassID: 3}); err != nil {
			t.Fatal(err)
		}
	}
	store := NewMemoryStore()
	handler := NewHandler(store, master, nil)
	handler.SetStaffScope(assignments, users)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))
	principal := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: first.ID, Role: identity.UserRoleTeacher}
	requestJSONAs(t, router, principal, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-02","school_class_id":3}`, http.StatusCreated)
	requestJSONAs(t, router, principal, http.MethodPost, "/api/v1/pickup-operations/1/confirm", `{}`, http.StatusOK)
	requestJSONAs(t, router, principal, http.MethodPost, "/api/v1/pickup-operations/1/start", "", http.StatusOK)
	teachers := requestJSONAs(t, router, principal, http.MethodGet, "/api/v1/pickup-operations/1/handoff-teachers", "", http.StatusOK)
	var page struct {
		Items []handoffTeacherView `json:"items"`
	}
	decodeData(t, teachers, &page)
	if len(page.Items) != 2 || page.Items[1].TeacherName == "" {
		t.Fatalf("handoff teachers = %+v, want both assigned teachers", page.Items)
	}
	requestJSONAs(t, router, principal, http.MethodPost, "/api/v1/pickup-operations/1/handover", fmt.Sprintf(`{"to_teacher_user_id":%d}`, second.ID), http.StatusOK)
}

func TestHandlerRejectsDuplicateOperationForSameDayAndClass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	handler := NewHandler(NewMemoryStore(), master, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":3}`, http.StatusCreated)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":3}`, http.StatusBadRequest)
}

func TestHandlerSecondVersionConfirmationWorkbenchAndTemporaryStudent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	store := NewMemoryStore()
	handler := NewHandler(store, master, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	created := requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":3,"teacher_name":"王老师"}`, http.StatusCreated)
	var operation operationView
	decodeData(t, created, &operation)
	confirmed := requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/confirm", `{"teacher_role":"substitute","expected_pickup_time":"16:20"}`, http.StatusOK)
	decodeData(t, confirmed, &operation)
	if operation.Status != OperationStatusConfirmed || operation.TeacherName != "王老师" || operation.ExpectedPickupTime != "16:20" || operation.TeacherRole != "substitute" {
		t.Fatalf("confirmed operation = %+v", operation)
	}

	notifications := requestJSON(t, router, http.MethodGet, "/api/v1/notifications", "", http.StatusOK)
	var notificationPage struct {
		Items []notificationView `json:"items"`
	}
	decodeData(t, notifications, &notificationPage)
	if len(notificationPage.Items) != 1 || notificationPage.Items[0].Kind != "pickup_plan_confirmed" {
		t.Fatalf("confirmation notifications = %+v", notificationPage.Items)
	}
	if _, err := store.CreateNotificationDeliveryLog(context.Background(), masterdata.DefaultOrganizationID, CreateDeliveryLogParams{NotificationID: notificationPage.Items[0].ID, ParentAccountID: 22, MessageKind: "pickup", TemplateID: "pickup-v1"}); err != nil {
		t.Fatal(err)
	}
	deliveryLogs := requestJSON(t, router, http.MethodGet, "/api/v1/notifications/delivery-logs?message_kind=pickup", "", http.StatusOK)
	var deliveryPage struct {
		Items []notificationDeliveryLogView `json:"items"`
		Total int                           `json:"total"`
	}
	decodeData(t, deliveryLogs, &deliveryPage)
	if deliveryPage.Total != 1 || deliveryPage.Items[0].StudentID != 4 || deliveryPage.Items[0].StudentName != "小明" || deliveryPage.Items[0].NotificationStatus != "pending" {
		t.Fatalf("delivery log details = %+v", deliveryPage.Items)
	}
	requestJSON(t, router, http.MethodGet, "/api/v1/notifications/delivery-logs?message_kind=unknown", "", http.StatusUnprocessableEntity)

	workbench := requestJSON(t, router, http.MethodGet, "/api/v1/pickup-workbench?date=2026-09-01", "", http.StatusOK)
	var workbenchData struct {
		Operations []workbenchOperationView `json:"operations"`
		Alerts     []workbenchAlert         `json:"alerts"`
	}
	decodeData(t, workbench, &workbenchData)
	if len(workbenchData.Operations) != 1 || len(workbenchData.Alerts) != 0 {
		t.Fatalf("workbench = %+v", workbenchData)
	}

	temporary := requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students", `{"name":"临时小雨","pickup_mode":"parent_picked_up","note":"家长临时联系"}`, http.StatusCreated)
	var temporaryStudent operationStudentView
	decodeData(t, temporary, &temporaryStudent)
	if !temporaryStudent.IsTemporary || !temporaryStudent.ProfilePending || temporaryStudent.StudentName != "临时小雨" {
		t.Fatalf("temporary student = %+v", temporaryStudent)
	}
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/4/profile", `{"guardian_phone":"13800000000"}`, http.StatusBadRequest)
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/5/profile", `{"notes":"仅补备注"}`, http.StatusUnprocessableEntity)
	profile := requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/5/profile", `{"guardian_phone":"13811112222","gender":"female","student_no":"TMP-05","emergency_contact":"李老师","emergency_phone":"13911112222","notes":"已与家长核实"}`, http.StatusOK)
	var profileData struct {
		Student          operationStudentProfileView `json:"student"`
		OperationStudent operationStudentView        `json:"operation_student"`
	}
	decodeData(t, profile, &profileData)
	if profileData.Student.GuardianPhone != "13811112222" || profileData.Student.Gender != "female" || profileData.OperationStudent.ProfilePending {
		t.Fatalf("completed temporary profile = %+v", profileData)
	}
	updatedStudent, err := master.FindStudent(context.Background(), masterdata.DefaultOrganizationID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if updatedStudent.EmergencyContact != "李老师" || updatedStudent.EmergencyPhone != "13911112222" || updatedStudent.Notes != "已与家长核实" {
		t.Fatalf("updated student profile = %+v", updatedStudent)
	}
	operationID := operation.ID
	change, err := store.CreatePickupChangeRequest(context.Background(), masterdata.DefaultOrganizationID, CreatePickupChangeRequestParams{StudentID: 4, OperationID: &operationID, ChangeDate: date("2026-09-01"), RequestedStatus: MemberStatusParentPickedUp, Note: "家长提前提交临时接走", SubmittedBy: "parent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReviewPickupChangeRequest(context.Background(), masterdata.DefaultOrganizationID, ReviewPickupChangeRequestParams{ID: change.ID, Status: ChangeRequestStatusApproved, ReviewNote: "已确认"}); err != nil {
		t.Fatal(err)
	}
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/start", "", http.StatusOK)
	members, err := store.ListOperationStudents(context.Background(), masterdata.DefaultOrganizationID, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range members {
		if member.StudentID == 4 && member.Status != MemberStatusParentPickedUp {
			t.Fatalf("approved pickup change status = %q, want %q", member.Status, MemberStatusParentPickedUp)
		}
	}
	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations/1/students/5/status", `{"status":"parent_picked_up","note":"家长现场接走"}`, http.StatusOK)
}

func TestViewerCannotWritePickupOperation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	handler := NewHandler(NewMemoryStore(), master, nil)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	viewer := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 99, Role: identity.UserRoleViewer}
	requestJSONAs(t, router, viewer, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":3}`, http.StatusForbidden)
}

func TestHandlerExcludesApprovedLeaveFromRoster(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	approvedLeave := approvedLeaveReader{studentIDs: map[uint64]struct{}{4: {}}}
	handler := NewHandler(NewMemoryStore(), master, nil, approvedLeave)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	requestJSON(t, router, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":3}`, http.StatusBadRequest)
}

func TestTeacherCanOnlyOperateAssignedClass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	if _, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: 1, TermID: 2, Grade: "三年级", Name: "2班"}); err != nil {
		t.Fatal(err)
	}
	users := identity.NewMemoryStore()
	teacherHash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	teacher, err := users.CreateUser(ctx, identity.CreateUserParams{Username: "teacher", PasswordHash: string(teacherHash), Role: identity.UserRoleTeacher, Nickname: "王老师", Status: identity.UserStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.ID, SchoolClassID: 3}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewMemoryStore(), master, nil)
	handler.SetStaffScope(assignments, users)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	teacherPrincipal := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: teacher.ID, Role: identity.UserRoleTeacher}
	created := requestJSONAs(t, router, teacherPrincipal, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":3}`, http.StatusCreated)
	var operation operationView
	decodeData(t, created, &operation)
	if operation.TeacherUserID == nil || *operation.TeacherUserID != teacher.ID || operation.TeacherName != "王老师" {
		t.Fatalf("operation teacher = %+v", operation)
	}

	requestJSONAs(t, router, teacherPrincipal, http.MethodGet, "/api/v1/pickup-operations/1/students", "", http.StatusOK)
	requestJSONAs(t, router, teacherPrincipal, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":5}`, http.StatusBadRequest)

	otherPrincipal := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 99, Role: identity.UserRoleTeacher}
	otherList := requestJSONAs(t, router, otherPrincipal, http.MethodGet, "/api/v1/pickup-operations?date=2026-09-01", "", http.StatusOK)
	var page struct {
		Total int `json:"total"`
	}
	decodeData(t, otherList, &page)
	if page.Total != 0 {
		t.Fatalf("unassigned teacher sees %d operations, want 0", page.Total)
	}
	requestJSONAs(t, router, otherPrincipal, http.MethodGet, "/api/v1/pickup-operations/1/students", "", http.StatusBadRequest)
}

func TestTeacherCanCreateTasksForMultipleAssignedClasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	seedMasterData(t, master)
	secondClass, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: 1, TermID: 2, Grade: "三年级", Name: "2班"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: 1, TermID: 2, SchoolClassID: secondClass.ID, Name: "小红", Gender: "female"}); err != nil {
		t.Fatal(err)
	}

	users := identity.NewMemoryStore()
	teacherHash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	teacher, err := users.CreateUser(ctx, identity.CreateUserParams{Username: "teacher", PasswordHash: string(teacherHash), Role: identity.UserRoleTeacher, Nickname: "王老师", Status: identity.UserStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	for _, classID := range []uint64{3, secondClass.ID} {
		if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.ID, SchoolClassID: classID}); err != nil {
			t.Fatal(err)
		}
	}

	handler := NewHandler(NewMemoryStore(), master, nil)
	handler.SetStaffScope(assignments, users)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))
	principal := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: teacher.ID, Role: identity.UserRoleTeacher}
	for _, classID := range []uint64{3, secondClass.ID} {
		created := requestJSONAs(t, router, principal, http.MethodPost, "/api/v1/pickup-operations", `{"operation_date":"2026-09-01","school_class_id":`+fmt.Sprint(classID)+`}`, http.StatusCreated)
		var operation operationView
		decodeData(t, created, &operation)
		if operation.SchoolClassID != classID || operation.TeacherUserID == nil || *operation.TeacherUserID != teacher.ID {
			t.Fatalf("operation = %+v, want class %d and teacher %d", operation, classID, teacher.ID)
		}
	}
}

type approvedLeaveReader struct{ studentIDs map[uint64]struct{} }

func (r approvedLeaveReader) ListApprovedLeaveStudentIDs(context.Context, uint64, time.Time) (map[uint64]struct{}, error) {
	return r.studentIDs, nil
}

func seedMasterData(t *testing.T, store *masterdata.MemoryStore) {
	t.Helper()
	ctx := t.Context()
	if _, err := store.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026-2027学年第一学期", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: 1, TermID: 2, Grade: "三年级", Name: "1班"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: 1, TermID: 2, SchoolClassID: 3, Name: "小明", Gender: "male"}); err != nil {
		t.Fatal(err)
	}
}

func newTestPhotoStore(t *testing.T) *storage.LocalFileStore {
	t.Helper()
	directory, err := os.MkdirTemp("", "tuoguan-pickup-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	store, err := storage.NewLocalFileStore(filepath.Join(directory, "uploads"), "/uploads", storage.DefaultMaxPhotoBytes)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func requestJSON(t *testing.T, router http.Handler, method, path, body string, wantStatus ...int) *httptest.ResponseRecorder {
	return requestJSONAs(t, router, identity.Principal{}, method, path, body, wantStatus...)
}

func requestJSONAs(t *testing.T, router http.Handler, principal identity.Principal, method, path, body string, wantStatus ...int) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if principal.Kind != "" {
		req = req.WithContext(identity.WithPrincipal(req.Context(), principal))
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(record, req)
	if len(wantStatus) > 0 && record.Code != wantStatus[0] {
		t.Fatalf("%s %s status = %d, want %d: %s", method, path, record.Code, wantStatus[0], record.Body.String())
	}
	return record
}

func decodeData(t *testing.T, record *httptest.ResponseRecorder, target any) {
	t.Helper()
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
