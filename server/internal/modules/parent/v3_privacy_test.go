package parent

import (
	"net/http"
	"testing"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/gin-gonic/gin"
)

func TestParentPrivacyConsentIsVersionedAndIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	master := masterdata.NewMemoryStore()
	parents := NewMemoryStore()
	if _, err := parents.CreateAccount(t.Context(), masterdata.DefaultOrganizationID, CreateAccountParams{OpenID: "privacy-parent"}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(parents, master, pickup.NewMemoryStore())
	router := gin.New()
	handler.RegisterParentRoutes(router.Group("/api/v1"))

	initial := parentRequest(t, router, http.MethodGet, "/api/v1/parent/privacy-consent", "", "privacy-parent")
	var initialView privacyConsentView
	decodeParentData(t, initial, &initialView)
	if initialView.Accepted || initialView.CurrentPolicyVersion != PrivacyPolicyCurrentVersion {
		t.Fatalf("initial privacy consent = %+v", initialView)
	}

	wrong := parentRequest(t, router, http.MethodPost, "/api/v1/parent/privacy-consent", `{"policy_version":"old"}`, "privacy-parent")
	if wrong.Code != http.StatusBadRequest {
		t.Fatalf("wrong policy version status = %d: %s", wrong.Code, wrong.Body.String())
	}

	accepted := parentRequest(t, router, http.MethodPost, "/api/v1/parent/privacy-consent", `{"policy_version":"`+PrivacyPolicyCurrentVersion+`"}`, "privacy-parent")
	var acceptedView privacyConsentView
	decodeParentData(t, accepted, &acceptedView)
	if !acceptedView.Accepted || acceptedView.ConsentedAt == nil {
		t.Fatalf("accepted privacy consent = %+v", acceptedView)
	}

	repeated := parentRequest(t, router, http.MethodPost, "/api/v1/parent/privacy-consent", `{"policy_version":"`+PrivacyPolicyCurrentVersion+`"}`, "privacy-parent")
	var repeatedView privacyConsentView
	decodeParentData(t, repeated, &repeatedView)
	if !repeatedView.Accepted || repeatedView.ConsentedAt == nil || *repeatedView.ConsentedAt != *acceptedView.ConsentedAt {
		t.Fatalf("repeated privacy consent = %+v", repeatedView)
	}
}
