package homework

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/storage"
)

func TestHomeworkSupportsClassBatchReviewAndParentView(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	if _, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"}); err != nil {
		t.Fatal(err)
	}
	if _, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", IsCurrent: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: 1, TermID: 2, Grade: "三年级", Name: "1班"}); err != nil {
		t.Fatal(err)
	}
	if _, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: 1, TermID: 2, SchoolClassID: 3, Name: "小明"}); err != nil {
		t.Fatal(err)
	}
	if _, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: 1, TermID: 2, SchoolClassID: 3, Name: "小红"}); err != nil {
		t.Fatal(err)
	}

	users := identity.NewMemoryStore()
	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	teacher, err := users.CreateUser(ctx, identity.CreateUserParams{Username: "teacher", PasswordHash: string(hash), Role: identity.UserRoleTeacher, Nickname: "王老师", Status: identity.UserStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.ID, SchoolClassID: 3}); err != nil {
		t.Fatal(err)
	}
	parents := parent.NewMemoryStore()
	notifications := pickup.NewMemoryStore()
	account, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, parent.CreateAccountParams{OpenID: "wechat:parent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parents.CreateBinding(ctx, masterdata.DefaultOrganizationID, parent.BindStudentParams{ParentAccountID: account.ID, StudentID: 4, Relationship: "家长", IsPrimary: true}); err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(NewMemoryStore(), master)
	handler.SetStaffScope(assignments, users)
	handler.SetParentStore(parents)
	handler.SetNotificationWriter(notifications)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))
	teacherPrincipal := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: teacher.ID, Role: identity.UserRoleTeacher}
	created := requestHomeworkAs(t, router, teacherPrincipal, http.MethodPost, "/api/v1/homework-tasks", `{"homework_date":"2026-09-01","school_class_id":3,"subject":"语文","content":"完成第一课生字","attachment_urls":["/uploads/homework/test.jpg"]}`, http.StatusCreated)
	var task taskView
	decodeHomework(t, created, &task)
	if task.CreatorName != "王老师" || task.Subject != "语文" {
		t.Fatalf("task = %+v", task)
	}
	if len(task.AttachmentURLs) != 1 || task.AttachmentURLs[0] != "/uploads/homework/test.jpg" {
		t.Fatalf("task attachments = %+v", task.AttachmentURLs)
	}
	createdNotifications, err := notifications.ListNotifications(ctx, masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(createdNotifications) != 2 || createdNotifications[0].Kind != "homework_published" {
		t.Fatalf("homework notifications = %+v", createdNotifications)
	}

	students := requestHomeworkAs(t, router, teacherPrincipal, http.MethodGet, "/api/v1/homework-tasks/1/students", "", http.StatusOK)
	var page struct {
		Items []taskStudentView `json:"items"`
		Total int               `json:"total"`
	}
	decodeHomework(t, students, &page)
	if page.Total != 2 {
		t.Fatalf("batch roster total = %d, want 2", page.Total)
	}
	batchReview := requestHomeworkAs(t, router, teacherPrincipal, http.MethodPost, "/api/v1/homework-tasks/1/students/bulk-review", `{"items":[{"student_id":4,"status":"completed","correction_note":"书写认真"},{"student_id":5,"status":"incomplete","correction_note":"需要订正"}]}`, http.StatusOK)
	var batchPage struct {
		Items []taskStudentView `json:"items"`
		Total int               `json:"total"`
	}
	decodeHomework(t, batchReview, &batchPage)
	if batchPage.Total != 2 || batchPage.Items[0].Status != StudentStatusCompleted || batchPage.Items[1].Status != StudentStatusIncomplete {
		t.Fatalf("batch review = %+v", batchPage)
	}
	createdNotifications, err = notifications.ListNotifications(ctx, masterdata.DefaultOrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if len(createdNotifications) != 4 || createdNotifications[0].Kind != "homework_review" {
		t.Fatalf("review notifications = %+v", createdNotifications)
	}

	parentHomework := requestHomeworkAs(t, router, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: account.ID}, http.MethodGet, "/api/v1/parent/students/4/homework?date=2026-09-01", "", http.StatusOK)
	var parentPage struct {
		Items []parentHomeworkView `json:"items"`
		Total int                  `json:"total"`
	}
	decodeHomework(t, parentHomework, &parentPage)
	if parentPage.Total != 1 || parentPage.Items[0].Status != StudentStatusCompleted || parentPage.Items[0].CorrectionNote != "书写认真" {
		t.Fatalf("parent homework = %+v", parentPage)
	}
	if len(parentPage.Items[0].AttachmentURLs) != 1 {
		t.Fatalf("parent attachments = %+v", parentPage.Items[0].AttachmentURLs)
	}

	secondTask := requestHomeworkAs(t, router, teacherPrincipal, http.MethodPost, "/api/v1/homework-tasks", `{"homework_date":"2026-09-01","school_class_id":3,"content":"单独订正","student_ids":[4]}`, http.StatusCreated)
	var individualTask taskView
	decodeHomework(t, secondTask, &individualTask)
	individual := requestHomeworkAs(t, router, teacherPrincipal, http.MethodGet, "/api/v1/homework-tasks/"+strconv.FormatUint(individualTask.ID, 10)+"/students", "", http.StatusOK)
	decodeHomework(t, individual, &page)
	if page.Total != 1 || page.Items[0].StudentID != 4 {
		t.Fatalf("individual roster = %+v", page)
	}
}

func TestHomeworkPhotoUploadUsesHomeworkStorageCategory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	photoStore, err := storage.NewLocalFileStore(filepath.Join(t.TempDir(), "uploads"), "/uploads", storage.DefaultMaxPhotoBytes)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewMemoryStore(), master)
	handler.SetPhotoStore(photoStore)
	router := gin.New()
	handler.RegisterStaffRoutes(router.Group("/api/v1"))

	var body bytes.Buffer
	multipartWriter := multipart.NewWriter(&body)
	part, err := multipartWriter.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="homework.jpg"`},
		"Content-Type":        {"image/jpeg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01}); err != nil {
		t.Fatal(err)
	}
	if err := multipartWriter.Close(); err != nil {
		t.Fatal(err)
	}
	record := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/homework", &body)
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	router.ServeHTTP(record, req)
	if record.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d: %s", record.Code, http.StatusCreated, record.Body.String())
	}
	var envelope struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(record.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.URL != "/uploads/homework/"+filepath.Base(envelope.Data.URL) {
		t.Fatalf("upload url = %q, want homework category", envelope.Data.URL)
	}
}

func requestHomeworkAs(t *testing.T, router http.Handler, principal identity.Principal, method, path, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req = req.WithContext(identity.WithPrincipal(req.Context(), principal))
	router.ServeHTTP(record, req)
	if record.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d: %s", method, path, record.Code, wantStatus, record.Body.String())
	}
	return record
}

func decodeHomework(t *testing.T, record *httptest.ResponseRecorder, target any) {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(record.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatal(err)
	}
}
