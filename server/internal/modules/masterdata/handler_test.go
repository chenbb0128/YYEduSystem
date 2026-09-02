package masterdata

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandlerCreatesAndFiltersStudentArchive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewMemoryStore())
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	assertStatus(t, router, http.MethodPost, "/api/v1/schools", `{"name":"实验小学","address":"人民路 1 号"}`, http.StatusCreated)
	assertStatus(t, router, http.MethodPost, "/api/v1/academic-terms", `{"name":"2026-2027 学年第一学期","starts_on":"2026-09-01","ends_on":"2027-01-31","is_current":true}`, http.StatusCreated)
	assertStatus(t, router, http.MethodPost, "/api/v1/school-classes", `{"school_id":1,"term_id":2,"grade":"三年级","name":"1班"}`, http.StatusCreated)
	assertStatus(t, router, http.MethodPost, "/api/v1/care-classes", `{"name":"晚托一班","capacity":30}`, http.StatusCreated)

	assertStatus(t, router, http.MethodPost, "/api/v1/students", `{"school_id":1,"term_id":2,"school_class_id":3,"care_class_id":4,"name":"小明","gender":"male","guardian_phone":"13800000000"}`, http.StatusCreated)
	response := assertStatus(t, router, http.MethodGet, "/api/v1/students?school_class_id=3&keyword=小明", "")
	var page struct {
		Items []studentView `json:"items"`
		Total int           `json:"total"`
	}
	decodeData(t, response, &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Name != "小明" {
		t.Fatalf("filtered students = %+v, want one 小明", page)
	}

	assertStatus(t, router, http.MethodPut, "/api/v1/students/5", `{"school_id":1,"term_id":2,"school_class_id":3,"care_class_id":4,"name":"小明同学","gender":"male","status":"active"}`, http.StatusOK)
}

func TestHandlerCreatesStudentProfileAndAutoCategorizes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewMemoryStore())
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	response := assertStatus(t, router, http.MethodPost, "/api/v1/students/profile", `{"school_name":"实验小学","grade":"三年级","class_name":"1班","care_class_name":"晚托一班","name":"小红","guardian_phone":"13800000000"}`, http.StatusCreated)
	var student studentView
	decodeData(t, response, &student)
	if student.ID == 0 || student.SchoolID == 0 || student.TermID == 0 || student.SchoolClassID == 0 || student.CareClassID == nil {
		t.Fatalf("profile student = %+v, want auto-created relations", student)
	}
	assertStatus(t, router, http.MethodPut, "/api/v1/students/"+strconv.FormatUint(student.ID, 10)+"/profile", `{"school_name":"实验小学","grade":"三年级","class_name":"2班","name":"小红同学","status":"active"}`, http.StatusOK)

	assertStatus(t, router, http.MethodPost, "/api/v1/students/profile", `{"school_name":"实验小学","grade":"三年级","class_name":"1班","care_class_name":"晚托一班","name":"小刚"}`, http.StatusCreated)
	for path, want := range map[string]int{
		"/api/v1/schools":        1,
		"/api/v1/academic-terms": 1,
		"/api/v1/school-classes": 2,
		"/api/v1/care-classes":   1,
	} {
		var page struct {
			Total int `json:"total"`
		}
		decodeData(t, assertStatus(t, router, http.MethodGet, path, ""), &page)
		if page.Total != want {
			t.Fatalf("%s total = %d, want %d", path, page.Total, want)
		}
	}

	var page struct {
		Items []studentView `json:"items"`
		Total int           `json:"total"`
	}
	decodeData(t, assertStatus(t, router, http.MethodGet, "/api/v1/students?grade=%E4%B8%89%E5%B9%B4%E7%BA%A7&status=active", ""), &page)
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("profile filter result = %+v, want two students", page)
	}
}

func TestHandlerRejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewMemoryStore())
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"))

	record := assertStatus(t, router, http.MethodPost, "/api/v1/schools", `{"name":"实验小学","unexpected":true}`, http.StatusUnprocessableEntity)
	var envelope struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(record.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if envelope.Code != 10007 {
		t.Fatalf("error code = %d, want 10007", envelope.Code)
	}
}

func assertStatus(t *testing.T, router http.Handler, method, path, body string, wantStatus ...int) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	var requestBody *bytes.Reader
	if body == "" {
		requestBody = bytes.NewReader(nil)
	} else {
		requestBody = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, requestBody)
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
		t.Fatalf("decode response: %v", err)
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatalf("decode response data: %v", err)
	}
}
