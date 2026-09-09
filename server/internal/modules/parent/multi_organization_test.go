package parent

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/platformadmin"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/wechat"
)

func TestParentCanJoinAndSwitchOrganizationsWithoutSharingChildren(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	organizations := platformadmin.NewMemoryStore()
	orgTwo, err := organizations.CreateOrganization(ctx, platformadmin.CreateOrganizationParams{Name: "第二托管班", Slug: "second", Status: platformadmin.OrganizationStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	parents := NewMemoryStore()
	tokens, err := identity.NewTokenManager("test-parent-multiorg-secret-012345678901234567890123", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(parents, master, pickup.NewMemoryStore(), tokens)
	handler.SetOrganizationStore(organizations)
	if handler.organizationInviteToken(orgTwo.ID) == "" {
		t.Fatal("organization invite token should not be empty")
	}
	router := gin.New()
	handler.RegisterAuthRoutes(router.Group("/api/v1"))
	handler.RegisterParentRoutes(router.Group("/api/v1"))

	first := parentRequestWithHeaders(t, router, http.MethodPost, "/api/v1/auth/parent/wechat", fmt.Sprintf(`{"openid":"same-openid","invite_token":"%s"}`, handler.organizationInviteToken(1)), nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first organization login status = %d: %s", first.Code, first.Body.String())
	}
	var firstToken parentTokenView
	decodeParentData(t, first, &firstToken)
	firstPrincipal, err := tokens.ParseAccess(firstToken.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if firstPrincipal.OrganizationID != 1 {
		t.Fatalf("first login organization = %d, want 1", firstPrincipal.OrganizationID)
	}

	second := parentRequestAs(t, router, firstPrincipal, http.MethodPost, "/api/v1/parent/organizations/switch", fmt.Sprintf(`{"invite_token":"%s"}`, handler.organizationInviteToken(orgTwo.ID)))
	if second.Code != http.StatusOK {
		t.Fatalf("second organization join status = %d: %s", second.Code, second.Body.String())
	}
	var secondToken parentTokenView
	decodeParentData(t, second, &secondToken)
	secondPrincipal, err := tokens.ParseAccess(secondToken.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if secondPrincipal.OrganizationID != orgTwo.ID {
		t.Fatalf("second login organization = %d, want %d", secondPrincipal.OrganizationID, orgTwo.ID)
	}

	organizationsResponse := parentRequestAs(t, router, secondPrincipal, http.MethodGet, "/api/v1/parent/organizations", "")
	if organizationsResponse.Code != http.StatusOK {
		t.Fatalf("organization list status = %d: %s", organizationsResponse.Code, organizationsResponse.Body.String())
	}
	var organizationPage listResponse[parentOrganizationView]
	decodeParentData(t, organizationsResponse, &organizationPage)
	if organizationPage.Total != 2 || len(organizationPage.Items) != 2 {
		t.Fatalf("organization list = %+v", organizationPage)
	}
	for _, item := range organizationPage.Items {
		if item.ID == secondPrincipal.OrganizationID && !item.Current {
			t.Fatalf("current organization was not marked current: %+v", item)
		}
		if item.ID == 1 && item.Current {
			t.Fatalf("old organization remained current: %+v", item)
		}
	}

	oldOrganizationSwitch := parentRequestAs(t, router, secondPrincipal, http.MethodPost, "/api/v1/parent/organizations/switch", `{"organization_id":1}`)
	if oldOrganizationSwitch.Code != http.StatusOK {
		t.Fatalf("switch to existing organization status = %d: %s", oldOrganizationSwitch.Code, oldOrganizationSwitch.Body.String())
	}
	var oldToken parentTokenView
	decodeParentData(t, oldOrganizationSwitch, &oldToken)
	oldPrincipal, err := tokens.ParseAccess(oldToken.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if oldPrincipal.OrganizationID != 1 {
		t.Fatalf("existing organization switch = %d, want 1", oldPrincipal.OrganizationID)
	}

	accounts, err := parents.ListAccountsByOpenID(ctx, "same-openid")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || accounts[0].OrganizationID == accounts[1].OrganizationID {
		t.Fatalf("accounts by openid = %+v", accounts)
	}
}

func TestParentCannotJoinOrganizationWithForgedInvite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	organizations := platformadmin.NewMemoryStore()
	orgTwo, err := organizations.CreateOrganization(ctx, platformadmin.CreateOrganizationParams{Name: "第二托管班", Slug: "second", Status: platformadmin.OrganizationStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	parents := NewMemoryStore()
	tokens, err := identity.NewTokenManager("test-parent-forged-invite-secret-012345678901234567", time.Hour, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(parents, masterdata.NewMemoryStore(), pickup.NewMemoryStore(), tokens)
	handler.SetOrganizationStore(organizations)
	router := gin.New()
	handler.RegisterAuthRoutes(router.Group("/api/v1"))

	forged := "o" + fmt.Sprintf("%x", orgTwo.ID) + ".forged"
	result := parentRequestWithHeaders(t, router, http.MethodPost, "/api/v1/auth/parent/wechat", fmt.Sprintf(`{"openid":"forged-openid","invite_token":"%s"}`, forged), nil)
	if result.Code == http.StatusOK {
		t.Fatal("forged organization invite was accepted")
	}
}

func TestClassInviteTokenCarriesOrganizationAndLegacyTokenStillWorks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	master := masterdata.NewMemoryStore()
	defaultSchool, err := master.CreateSchool(ctx, 1, masterdata.CreateSchoolParams{Name: "默认小学"})
	if err != nil {
		t.Fatal(err)
	}
	defaultTerm, err := master.CreateAcademicTerm(ctx, 1, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	defaultClass, err := master.CreateSchoolClass(ctx, 1, masterdata.CreateSchoolClassParams{SchoolID: defaultSchool.ID, TermID: defaultTerm.ID, Grade: "二年级", Name: "1班"})
	if err != nil {
		t.Fatal(err)
	}
	orgTwoSchool, err := master.CreateSchool(ctx, 2, masterdata.CreateSchoolParams{Name: "第二小学"})
	if err != nil {
		t.Fatal(err)
	}
	term, err := master.CreateAcademicTerm(ctx, 2, masterdata.CreateAcademicTermParams{Name: "2026 秋季", StartsOn: date("2026-09-01"), EndsOn: date("2027-01-31"), IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	secondClass, err := master.CreateSchoolClass(ctx, 2, masterdata.CreateSchoolClassParams{SchoolID: orgTwoSchool.ID, TermID: term.ID, Grade: "一年级", Name: "2班"})
	if err != nil {
		t.Fatal(err)
	}
	parents := NewMemoryStore()
	handler := NewHandler(parents, master, pickup.NewMemoryStore())
	handler.SetClassInviteSecret("class-invite-multiorg-test-secret")
	router := gin.New()
	handler.RegisterParentRoutes(router.Group("/api/v1"))

	newToken := handler.classInviteToken(2, secondClass.ID)
	newResult := parentRequestWithHeaders(t, router, http.MethodGet, "/api/v1/parent/class-invites/"+newToken, "", nil)
	if newResult.Code != http.StatusOK {
		t.Fatalf("new class invite status = %d: %s", newResult.Code, newResult.Body.String())
	}
	var newInvite classInviteView
	decodeParentData(t, newResult, &newInvite)
	if newInvite.OrganizationID != 2 || newInvite.SchoolClassID != secondClass.ID {
		t.Fatalf("new class invite = %+v", newInvite)
	}

	legacyEncoded := fmt.Sprintf("%x", defaultClass.ID)
	legacyToken := "c" + legacyEncoded + "." + handler.classInviteSignature(1, legacyEncoded)
	legacyResult := parentRequestWithHeaders(t, router, http.MethodGet, "/api/v1/parent/class-invites/"+legacyToken, "", nil)
	if legacyResult.Code != http.StatusOK {
		t.Fatalf("legacy class invite status = %d: %s", legacyResult.Code, legacyResult.Body.String())
	}
	var legacyInvite classInviteView
	decodeParentData(t, legacyResult, &legacyInvite)
	if legacyInvite.OrganizationID != 1 || legacyInvite.SchoolClassID != defaultClass.ID {
		t.Fatalf("legacy class invite = %+v", legacyInvite)
	}
}

func TestOrganizationInviteQRCodeUsesCurrentOrganization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	organizations := platformadmin.NewMemoryStore()
	orgTwo, err := organizations.CreateOrganization(ctx, platformadmin.CreateOrganizationParams{Name: "第二托管班", Slug: "second", Status: platformadmin.OrganizationStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	generator := &testMiniProgramCodeGeneratorMultiOrganization{}
	handler := NewHandler(NewMemoryStore(), masterdata.NewMemoryStore(), pickup.NewMemoryStore())
	handler.SetOrganizationStore(organizations)
	handler.SetMiniProgramCodeGenerator(generator)
	router := gin.New()
	handler.RegisterStaffRoutes(router.Group("/api/v1"))

	principal := identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: 8, OrganizationID: orgTwo.ID, Role: identity.UserRoleTeacher}
	result := parentRequestAs(t, router, principal, http.MethodGet, "/api/v1/organization-invites/qrcode", "")
	if result.Code != http.StatusOK {
		t.Fatalf("organization qr status = %d: %s", result.Code, result.Body.String())
	}
	if generator.params.Scene != handler.organizationInviteToken(orgTwo.ID) {
		t.Fatalf("organization qr scene = %q, want %q", generator.params.Scene, handler.organizationInviteToken(orgTwo.ID))
	}
}

type testMiniProgramCodeGeneratorMultiOrganization struct {
	params wechat.MiniProgramCodeParams
}

func (g *testMiniProgramCodeGeneratorMultiOrganization) GenerateMiniProgramCode(_ context.Context, params wechat.MiniProgramCodeParams) ([]byte, error) {
	g.params = params
	return []byte("test-png"), nil
}
