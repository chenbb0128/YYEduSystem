package meal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/parent"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/gin-gonic/gin"
)

func TestParentDietNoteChangeRequiresStaffApproval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	school, err := master.CreateSchool(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolParams{Name: "实验小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, masterdata.DefaultOrganizationID, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: masterDate("2026-09-01"), EndsOn: masterDate("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	schoolClass, err := master.CreateSchoolClass(ctx, masterdata.DefaultOrganizationID, masterdata.CreateSchoolClassParams{SchoolID: school.ID, TermID: term.ID, Grade: "一年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	student, err := master.CreateStudent(ctx, masterdata.DefaultOrganizationID, masterdata.CreateStudentParams{SchoolID: school.ID, TermID: term.ID, SchoolClassID: schoolClass.ID, Name: "小明", GuardianPhone: "13800000000"})
	if err != nil {
		t.Fatal(err)
	}
	parents := parent.NewMemoryStore()
	account, err := parents.CreateAccount(ctx, masterdata.DefaultOrganizationID, parent.CreateAccountParams{OpenID: "diet-parent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parents.CreateBinding(ctx, masterdata.DefaultOrganizationID, parent.BindStudentParams{ParentAccountID: account.ID, StudentID: student.ID, Relationship: "妈妈"}); err != nil {
		t.Fatal(err)
	}
	store := NewMemoryStore()
	if _, err := store.UpsertDietNote(ctx, masterdata.DefaultOrganizationID, UpsertDietNoteParams{StudentID: student.ID, Note: "忌牛奶", UpdatedByName: "老师"}); err != nil {
		t.Fatal(err)
	}
	assignments := assignment.NewMemoryStore()
	teacher := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 7, Role: identity.UserRoleTeacher}
	if _, err := assignments.Create(ctx, masterdata.DefaultOrganizationID, assignment.CreateParams{TeacherUserID: teacher.SubjectID, SchoolClassID: schoolClass.ID}); err != nil {
		t.Fatal(err)
	}
	notifications := pickup.NewMemoryStore()
	handler := NewHandler(store, master)
	handler.SetParentStore(parents)
	handler.SetStaffScope(assignments)
	handler.SetNotificationWriter(notifications)
	router := gin.New()
	parentsAPI := router.Group("/api/v1")
	handler.RegisterParentRoutes(parentsAPI)
	staffAPI := router.Group("/api/v1")
	handler.RegisterStaffRoutes(staffAPI)

	created := mealParentRequest(t, router, http.MethodPost, fmt.Sprintf("/api/v1/parent/students/%d/diet-note-requests", student.ID), `{"note":"花生过敏"}`, identity.WithPrincipal(ctx, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: account.ID}))
	if created.Code != http.StatusCreated {
		t.Fatalf("create request status = %d: %s", created.Code, created.Body.String())
	}
	var requestView dietNoteChangeRequestView
	decodeMealData(t, created, &requestView)
	if requestView.Status != DietNoteChangeStatusPending || requestView.CurrentNote != "忌牛奶" {
		t.Fatalf("created request = %+v", requestView)
	}

	notes, err := store.ListDietNotes(ctx, masterdata.DefaultOrganizationID, &student.ID)
	if err != nil || len(notes) != 1 || notes[0].Note != "忌牛奶" {
		t.Fatalf("official note changed before review = %+v, err=%v", notes, err)
	}
	duplicate := mealParentRequest(t, router, http.MethodPost, fmt.Sprintf("/api/v1/parent/students/%d/diet-note-requests", student.ID), `{"note":"仍然过敏"}`, identity.WithPrincipal(ctx, identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: account.ID}))
	if duplicate.Code != http.StatusBadRequest {
		t.Fatalf("duplicate request status = %d: %s", duplicate.Code, duplicate.Body.String())
	}

	staffList := mealParentRequest(t, router, http.MethodGet, "/api/v1/diet-note-change-requests?status=pending", "", identity.WithPrincipal(ctx, teacher))
	var list struct {
		Items []dietNoteChangeRequestView `json:"items"`
	}
	decodeMealData(t, staffList, &list)
	if len(list.Items) != 1 || list.Items[0].ID != requestView.ID {
		t.Fatalf("staff requests = %+v", list)
	}
	review := mealParentRequest(t, router, http.MethodPost, fmt.Sprintf("/api/v1/diet-note-change-requests/%d/review", requestView.ID), `{"status":"approved","review_note":"已核对家长说明"}`, identity.WithPrincipal(ctx, teacher))
	if review.Code != http.StatusOK {
		t.Fatalf("review status = %d: %s", review.Code, review.Body.String())
	}
	notes, err = store.ListDietNotes(ctx, masterdata.DefaultOrganizationID, &student.ID)
	if err != nil || len(notes) != 1 || notes[0].Note != "花生过敏" {
		t.Fatalf("official note after review = %+v, err=%v", notes, err)
	}
	items, err := notifications.ListNotifications(ctx, masterdata.DefaultOrganizationID)
	if err != nil || len(items) != 1 || items[0].Kind != "meal_diet_note_review" {
		t.Fatalf("review notification = %+v, err=%v", items, err)
	}
}

func mealParentRequest(t *testing.T, router http.Handler, method, path, body string, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body)).WithContext(ctx)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(record, req)
	return record
}

func decodeMealData(t *testing.T, record *httptest.ResponseRecorder, target any) {
	t.Helper()
	if record.Code < 200 || record.Code >= 300 {
		t.Fatalf("unexpected status %d: %s", record.Code, record.Body.String())
	}
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

func masterDate(value string) (result time.Time) {
	result, _ = time.ParseInLocation("2006-01-02", value, time.UTC)
	return
}
