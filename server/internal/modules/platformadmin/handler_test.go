package platformadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
)

func TestHandlerRegistrationApprovalCreatesTenantAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := NewMemoryStore()
	users := identity.NewMemoryStore()
	handler := NewHandler(store, users)

	platformRouter := gin.New()
	platformRouter.Use(testPlatformPrincipal(identity.UserRolePlatformAdmin))
	handler.RegisterRoutes(platformRouter.Group("/api/v1"))

	publicRouter := gin.New()
	handler.RegisterPublicRoutes(publicRouter.Group("/api/v1"))

	inviteResponse := platformRequest(t, platformRouter, http.MethodPost, "/api/v1/platform/invites", `{"maxUses":1}`, http.StatusCreated)
	var inviteEnvelope struct {
		Data struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	decodePlatformEnvelope(t, inviteResponse, &inviteEnvelope)
	if inviteEnvelope.Data.Code == "" {
		t.Fatal("generated invite code is empty")
	}

	registrationBody := `{"inviteCode":"` + inviteEnvelope.Data.Code + `","organizationName":"阳光托管中心","contactName":"李老师","adminUsername":"sunny-admin","adminPassword":"strong-password"}`
	registrationResponse := platformRequest(t, publicRouter, http.MethodPost, "/api/v1/auth/organization-register", registrationBody, http.StatusCreated)
	var registrationEnvelope struct {
		Data struct {
			ID uint64 `json:"id"`
		} `json:"data"`
	}
	decodePlatformEnvelope(t, registrationResponse, &registrationEnvelope)
	if registrationEnvelope.Data.ID == 0 {
		t.Fatal("registration id is empty")
	}

	path := "/api/v1/platform/registrations/" + strconv.FormatUint(registrationEnvelope.Data.ID, 10) + "/review"
	approved := platformRequest(t, platformRouter, http.MethodPost, path, `{"status":"approved","reviewNote":"资料已核验"}`, http.StatusOK)
	var approvalEnvelope struct {
		Data struct {
			OrganizationID uint64 `json:"organizationId"`
			AdminUserID    uint64 `json:"adminUserId"`
		} `json:"data"`
	}
	decodePlatformEnvelope(t, approved, &approvalEnvelope)
	if approvalEnvelope.Data.OrganizationID == 0 || approvalEnvelope.Data.AdminUserID == 0 {
		t.Fatalf("approval result = %+v", approvalEnvelope.Data)
	}

	admin, err := users.FindUserByUsername(context.Background(), "sunny-admin")
	if err != nil {
		t.Fatal(err)
	}
	if admin.OrganizationID != approvalEnvelope.Data.OrganizationID || admin.Role != identity.UserRoleAdmin {
		t.Fatalf("created admin = %+v", admin)
	}
	organizations := platformRequest(t, platformRouter, http.MethodGet, "/api/v1/platform/organizations", "", http.StatusOK)
	var organizationsEnvelope struct {
		Data struct {
			Items []Organization `json:"items"`
		} `json:"data"`
	}
	decodePlatformEnvelope(t, organizations, &organizationsEnvelope)
	found := false
	for _, item := range organizationsEnvelope.Data.Items {
		if item.ID == approvalEnvelope.Data.OrganizationID && item.Status == OrganizationStatusActive {
			found = true
		}
	}
	if !found {
		t.Fatalf("approved organization not found: %+v", organizationsEnvelope.Data.Items)
	}
}

func TestHandlerRejectsOrganizationAdminFromPlatformRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(NewMemoryStore(), identity.NewMemoryStore())
	router := gin.New()
	router.Use(testPlatformPrincipal(identity.UserRoleAdmin))
	handler.RegisterRoutes(router.Group("/api/v1"))

	platformRequest(t, router, http.MethodGet, "/api/v1/platform/organizations", "", http.StatusForbidden)
}

func testPlatformPrincipal(role identity.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(identity.WithPrincipal(c.Request.Context(), identity.Principal{
			Kind:           identity.PrincipalKindUser,
			SubjectID:      1,
			OrganizationID: 1,
			Role:           role,
		}))
		c.Next()
	}
}

func platformRequest(t *testing.T, router http.Handler, method, path, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	record := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(record, req)
	if record.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d: %s", method, path, record.Code, wantStatus, record.Body.String())
	}
	return record
}

func decodePlatformEnvelope(t *testing.T, record *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(record.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}
