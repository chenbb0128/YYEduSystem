package parent

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/sms"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/verification"
)

func TestPhoneCodeEndpointsUseOneTimeRandomCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens, err := identity.NewTokenManager("01234567890123456789012345678901", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service, err := verification.NewService(verification.NewMemoryStore(), sms.LocalSender{}, config.SMSConfig{
		CodeSecret:        "handler-code-secret",
		CodeLength:        6,
		CodeTTL:           time.Minute,
		ResendInterval:    time.Second,
		MaxVerifyAttempts: 5,
	}, "fallback")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(NewMemoryStore(), masterdata.NewMemoryStore(), pickup.NewMemoryStore(), tokens)
	handler.SetPhoneCodeService(service)
	router := gin.New()
	handler.RegisterAuthRoutes(router.Group("/api/v1"))

	sent := parentRequest(t, router, http.MethodPost, "/api/v1/auth/phone-code", `{"phone":"13800000000"}`, "")
	if sent.Code != http.StatusOK {
		t.Fatalf("send status = %d: %s", sent.Code, sent.Body.String())
	}
	var sentData phoneCodeView
	decodeParentData(t, sent, &sentData)
	if sentData.DebugCode == "" {
		t.Fatalf("send data = %+v", sentData)
	}

	fixed := parentRequest(t, router, http.MethodPost, "/api/v1/auth/phone-login", `{"phone":"13800000000","code":"12345"}`, "")
	if fixed.Code != http.StatusBadRequest {
		t.Fatalf("fixed code status = %d: %s", fixed.Code, fixed.Body.String())
	}
	login := parentRequest(t, router, http.MethodPost, "/api/v1/auth/phone-login", `{"phone":"13800000000","code":"`+sentData.DebugCode+`"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", login.Code, login.Body.String())
	}
	var result struct {
		Roles []phoneLoginRoleView `json:"roles"`
	}
	decodeParentData(t, login, &result)
	if len(result.Roles) != 2 || !result.Roles[0].Available {
		t.Fatalf("login roles = %+v", result.Roles)
	}
}
