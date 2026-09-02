package assignment

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
)

func TestHandlerCreatesListsAndDisablesTeacherAssignment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	users := identity.NewMemoryStore()
	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	teacher, err := users.CreateUser(ctx, identity.CreateUserParams{Username: "teacher", PasswordHash: string(hash), Role: identity.UserRoleTeacher, Nickname: "王老师", Status: identity.UserStatusActive})
	if err != nil {
		t.Fatal(err)
	}
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
	handler := NewHandler(NewMemoryStore(), users, master)
	router := gin.New()
	router.Use(withPrincipal(identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 1, Role: identity.UserRoleAdmin}))
	handler.RegisterRoutes(router.Group("/api/v1"))

	created := requestAssignment(t, router, http.MethodPost, "/api/v1/teacher-assignments", `{"teacher_user_id":2,"school_class_id":3}`, http.StatusCreated)
	var item assignmentView
	decodeAssignment(t, created, &item)
	if item.TeacherName != "王老师" || item.ClassName != "1班" || item.Status != AssignmentStatusActive {
		t.Fatalf("assignment = %+v", item)
	}

	list := requestAssignment(t, router, http.MethodGet, "/api/v1/teacher-assignments", "", http.StatusOK)
	var page struct {
		Items []assignmentView `json:"items"`
		Total int              `json:"total"`
	}
	decodeAssignment(t, list, &page)
	if page.Total != 1 {
		t.Fatalf("assignment total = %d, want 1", page.Total)
	}

	updated := requestAssignment(t, router, http.MethodPut, "/api/v1/teacher-assignments/1", `{"status":"disabled"}`, http.StatusOK)
	decodeAssignment(t, updated, &item)
	if item.Status != AssignmentStatusDisabled {
		t.Fatalf("updated status = %q, want disabled", item.Status)
	}

	reactivated := requestAssignment(t, router, http.MethodPost, "/api/v1/teacher-assignments", `{"teacher_user_id":2,"school_class_id":3}`, http.StatusCreated)
	decodeAssignment(t, reactivated, &item)
	if item.ID != 1 || item.Status != AssignmentStatusActive {
		t.Fatalf("reactivated assignment = %+v", item)
	}
	_ = teacher
}

func withPrincipal(principal identity.Principal) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(identity.WithPrincipal(c.Request.Context(), principal))
		c.Next()
	}
}

func requestAssignment(t *testing.T, router http.Handler, method, path, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(record, req)
	if record.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d: %s", method, path, record.Code, wantStatus, record.Body.String())
	}
	return record
}

func decodeAssignment(t *testing.T, record *httptest.ResponseRecorder, target any) {
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
