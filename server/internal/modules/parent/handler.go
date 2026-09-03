package parent

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	auditmodule "github.com/chenbb0128/tuoguan-system-server/internal/modules/audit"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/pickup"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/storage"
	"github.com/chenbb0128/tuoguan-system-server/internal/platform/verification"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

const (
	ParentOpenIDHeader = "X-Parent-OpenID"
	ParentIDHeader     = "X-Parent-ID"
)

type Handler struct {
	store               Store
	masterData          masterdata.Store
	pickup              pickup.Store
	users               identity.UserStore
	tokens              *identity.TokenManager
	exchanger           CodeExchanger
	allowLocalCode      bool
	allowLocalPhoneCode bool
	phoneCodeService    *verification.Service
	assignments         assignment.Store
	audit               auditmodule.Writer
	photoSigner         *storage.URLSigner
	photoURLTTL         time.Duration
	orgID               uint64
}

type CodeExchanger interface {
	ExchangeCode(context.Context, string) (string, error)
}

func NewHandler(store Store, masterData masterdata.Store, pickupStore pickup.Store, tokens ...*identity.TokenManager) *Handler {
	var tokenManager *identity.TokenManager
	if len(tokens) > 0 {
		tokenManager = tokens[0]
	}
	return &Handler{store: store, masterData: masterData, pickup: pickupStore, tokens: tokenManager, allowLocalCode: true, allowLocalPhoneCode: true, photoURLTTL: 15 * time.Minute, orgID: masterdata.DefaultOrganizationID}
}

func (h *Handler) SetCodeExchanger(exchanger CodeExchanger) { h.exchanger = exchanger }

func (h *Handler) SetAllowLocalCode(allow bool) { h.allowLocalCode = allow }

// SetAllowLocalPhoneCode controls the development-only demo SMS flow. A
// production deployment must use WeChat login or a real SMS provider; the
// hard-coded local verification code is never allowed there.
func (h *Handler) SetAllowLocalPhoneCode(allow bool) { h.allowLocalPhoneCode = allow }

func (h *Handler) SetPhoneCodeService(service *verification.Service) { h.phoneCodeService = service }

func (h *Handler) SetStaffScope(assignments assignment.Store) { h.assignments = assignments }

func (h *Handler) SetUserStore(users identity.UserStore) { h.users = users }

func (h *Handler) SetPhotoSigner(signer *storage.URLSigner) { h.photoSigner = signer }

func (h *Handler) SetPhotoURLTTL(ttl time.Duration) {
	if ttl > 0 {
		h.photoURLTTL = ttl
	}
}

func (h *Handler) SetAuditWriter(writer auditmodule.Writer) { h.audit = writer }

func (h *Handler) recordAudit(c *gin.Context, action, resourceType string, resourceID uint64) {
	auditmodule.RecordForContext(c.Request.Context(), h.audit, identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), action, resourceType, &resourceID, "{}", c.GetHeader("X-Request-ID"))
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	h.RegisterAuthRoutes(api)
	h.RegisterParentRoutes(api)
	h.RegisterStaffRoutes(api)
}

func (h *Handler) RegisterAuthRoutes(api *gin.RouterGroup) {
	api.POST("/auth/parent/wechat", h.loginWithWeChat)
	api.POST("/auth/phone-code", h.sendPhoneCode)
	api.POST("/auth/phone-login", h.loginWithPhone)
}

func (h *Handler) RegisterParentRoutes(api *gin.RouterGroup) {
	api.POST("/parent/bindings", h.bindStudent)
	api.POST("/parent/child-applications", h.createChildApplication)
	api.GET("/parent/child-applications", h.listParentChildApplications)
	api.PUT("/parent/child-applications/:id", h.updateChildApplication)
	api.GET("/parent/me", h.getMe)
	api.GET("/parent/students", h.listStudents)
	api.GET("/parent/students/:student_id/pickup-events", h.listStudentPickupEvents)
	api.GET("/parent/students/:student_id/pickup-today", h.getStudentPickupToday)
	api.GET("/parent/notifications", h.listNotifications)
	api.POST("/parent/notifications/:id/read", h.markNotificationRead)
	api.GET("/parent/subscriptions", h.listSubscriptions)
	api.POST("/parent/subscriptions", h.updateSubscriptions)
	api.GET("/parent/privacy-consent", h.getPrivacyConsent)
	api.POST("/parent/privacy-consent", h.recordPrivacyConsent)
	api.GET("/parent/leave-requests", h.listParentLeaveRequests)
	api.POST("/parent/students/:student_id/leave-requests", h.createLeaveRequest)
	api.PUT("/parent/leave-requests/:id", h.updateParentLeaveRequest)
	api.POST("/parent/leave-requests/:id/cancel", h.cancelParentLeaveRequest)
	api.POST("/parent/students/:student_id/pickup-changes", h.createPickupChangeRequest)
}

func (h *Handler) RegisterStaffRoutes(api *gin.RouterGroup) {
	api.GET("/child-applications", h.listStaffChildApplications)
	api.POST("/child-applications/:id/review", h.reviewChildApplication)
	api.GET("/leave-requests", h.listAllLeaveRequests)
	api.POST("/leave-requests/teacher", h.createTeacherLeaveRequest)
	api.POST("/leave-requests/:id/review", h.reviewLeaveRequest)
}

type accountView struct {
	ID       uint64 `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Status   string `json:"status"`
}

type bindingView struct {
	ID            uint64  `json:"id"`
	StudentID     uint64  `json:"student_id"`
	StudentName   string  `json:"student_name"`
	SchoolClassID uint64  `json:"school_class_id"`
	CareClassID   *uint64 `json:"care_class_id,omitempty"`
	SchoolName    string  `json:"school_name,omitempty"`
	Grade         string  `json:"grade,omitempty"`
	ClassName     string  `json:"class_name,omitempty"`
	CareClassName string  `json:"care_class_name,omitempty"`
	Relationship  string  `json:"relationship"`
	IsPrimary     bool    `json:"is_primary"`
}

type childApplicationView struct {
	StudentMatches  []studentMatchView `json:"student_matches,omitempty"`
	ID              uint64             `json:"id"`
	ParentAccountID uint64             `json:"parent_account_id,omitempty"`
	StudentID       *uint64            `json:"student_id,omitempty"`
	StudentName     string             `json:"student_name"`
	SchoolNameInput string             `json:"school_name_input"`
	GradeInput      string             `json:"grade_input"`
	ClassNameInput  string             `json:"class_name_input"`
	SchoolID        *uint64            `json:"school_id,omitempty"`
	SchoolClassID   *uint64            `json:"school_class_id,omitempty"`
	Grade           string             `json:"grade"`
	ClassName       string             `json:"class_name"`
	GuardianName    string             `json:"guardian_name"`
	GuardianPhone   string             `json:"guardian_phone"`
	Relationship    string             `json:"relationship"`
	Notes           string             `json:"notes"`
	Status          string             `json:"status"`
	ReviewNote      string             `json:"review_note"`
	ReviewedAt      *string            `json:"reviewed_at,omitempty"`
	CreatedAt       string             `json:"created_at"`
}

type studentMatchView struct {
	ID            uint64 `json:"id"`
	Name          string `json:"name"`
	GuardianPhone string `json:"guardian_phone,omitempty"`
}

type leaveRequestView struct {
	ID              uint64  `json:"id"`
	StudentID       uint64  `json:"student_id"`
	ParentAccountID *uint64 `json:"parent_account_id,omitempty"`
	SubmittedByType string  `json:"submitted_by_type"`
	LeaveDate       string  `json:"leave_date"`
	Reason          string  `json:"reason"`
	Status          string  `json:"status"`
	TeacherNote     string  `json:"teacher_note"`
	ReviewedAt      *string `json:"reviewed_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

type parentMeView struct {
	Account  accountView   `json:"account"`
	Children []bindingView `json:"children"`
}

type subscriptionView struct {
	Kind            string  `json:"kind"`
	Status          string  `json:"status"`
	TemplateVersion string  `json:"template_version,omitempty"`
	AuthorizedAt    *string `json:"authorized_at,omitempty"`
	UpdatedAt       string  `json:"updated_at"`
}

type privacyConsentView struct {
	Accepted             bool    `json:"accepted"`
	PolicyVersion        string  `json:"policy_version"`
	CurrentPolicyVersion string  `json:"current_policy_version"`
	ConsentedAt          *string `json:"consented_at,omitempty"`
}

type subscriptionRequestItem struct {
	Kind            string `json:"kind"`
	Status          string `json:"status"`
	TemplateVersion string `json:"template_version"`
}

type subscriptionRequest struct {
	Subscriptions []subscriptionRequestItem `json:"subscriptions"`
}

type privacyConsentRequest struct {
	PolicyVersion string `json:"policy_version"`
}

func (r privacyConsentRequest) Validate() []response.ValidationDetail {
	if strings.TrimSpace(r.PolicyVersion) == "" {
		return []response.ValidationDetail{{Field: "policy_version", Reason: "required"}}
	}
	if len([]rune(strings.TrimSpace(r.PolicyVersion))) > 128 {
		return []response.ValidationDetail{{Field: "policy_version", Reason: "too_long"}}
	}
	return nil
}

func (r subscriptionRequest) Validate() []response.ValidationDetail {
	if len(r.Subscriptions) == 0 {
		return []response.ValidationDetail{{Field: "subscriptions", Reason: "required"}}
	}
	details := make([]response.ValidationDetail, 0)
	seenKinds := make(map[string]struct{}, len(r.Subscriptions))
	for index, item := range r.Subscriptions {
		kind := strings.TrimSpace(item.Kind)
		if !isMessageKind(kind) {
			details = append(details, response.ValidationDetail{Field: fmt.Sprintf("subscriptions[%d].kind", index), Reason: "invalid_value"})
		} else if _, exists := seenKinds[kind]; exists {
			details = append(details, response.ValidationDetail{Field: fmt.Sprintf("subscriptions[%d].kind", index), Reason: "duplicate_value"})
		} else {
			seenKinds[kind] = struct{}{}
		}
		switch strings.TrimSpace(item.Status) {
		case "accept", "reject", "ban", "filter", "unknown":
		default:
			details = append(details, response.ValidationDetail{Field: fmt.Sprintf("subscriptions[%d].status", index), Reason: "invalid_value"})
		}
		if len([]rune(strings.TrimSpace(item.TemplateVersion))) > 128 {
			details = append(details, response.ValidationDetail{Field: fmt.Sprintf("subscriptions[%d].template_version", index), Reason: "too_long"})
		}
	}
	return details
}

type parentPickupTodayView struct {
	OperationID        uint64 `json:"operation_id"`
	OperationDate      string `json:"operation_date"`
	SchoolClassID      uint64 `json:"school_class_id"`
	SchoolName         string `json:"school_name,omitempty"`
	Grade              string `json:"grade,omitempty"`
	ClassName          string `json:"class_name,omitempty"`
	PickupMode         string `json:"pickup_mode"`
	Status             string `json:"status"`
	TeacherName        string `json:"teacher_name"`
	TeacherRole        string `json:"teacher_role"`
	ExpectedPickupTime string `json:"expected_pickup_time,omitempty"`
	StudentStatus      string `json:"student_status"`
	PhotoURL           string `json:"photo_url,omitempty"`
	ProfilePending     bool   `json:"profile_pending"`
}

type parentPickupChangeRequest struct {
	ChangeDate      string `json:"change_date"`
	RequestedStatus string `json:"requested_status"`
	Note            string `json:"note"`
}

func (r parentPickupChangeRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 3)
	if _, err := parseDate(r.ChangeDate); err != nil {
		details = append(details, response.ValidationDetail{Field: "change_date", Reason: "date_format"})
	}
	switch r.RequestedStatus {
	case pickup.MemberStatusParentPickedUp, pickup.MemberStatusSelfArrived, pickup.MemberStatusLeave, pickup.MemberStatusAbsent:
	default:
		details = append(details, response.ValidationDetail{Field: "requested_status", Reason: "invalid_value"})
	}
	if strings.TrimSpace(r.Note) == "" {
		details = append(details, response.ValidationDetail{Field: "note", Reason: "required"})
	}
	return details
}

type bindStudentRequest struct {
	OpenID        string `json:"openid"`
	Nickname      string `json:"nickname"`
	Avatar        string `json:"avatar"`
	StudentID     uint64 `json:"student_id"`
	GuardianPhone string `json:"guardian_phone"`
	Relationship  string `json:"relationship"`
	IsPrimary     bool   `json:"is_primary"`
}

type childApplicationRequest struct {
	StudentName   string `json:"student_name"`
	SchoolName    string `json:"school_name"`
	Grade         string `json:"grade"`
	ClassName     string `json:"class_name"`
	ClassText     string `json:"class_text"`
	SchoolClassID uint64 `json:"school_class_id"`
	GuardianName  string `json:"guardian_name"`
	GuardianPhone string `json:"guardian_phone"`
	Relationship  string `json:"relationship"`
	Notes         string `json:"notes"`
}

func (r childApplicationRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 4)
	if strings.TrimSpace(r.StudentName) == "" {
		details = append(details, response.ValidationDetail{Field: "student_name", Reason: "required"})
	}
	if strings.TrimSpace(r.GuardianPhone) == "" {
		details = append(details, response.ValidationDetail{Field: "guardian_phone", Reason: "required"})
	}
	if r.SchoolClassID == 0 && strings.TrimSpace(r.Grade) == "" && strings.TrimSpace(r.ClassText) == "" {
		details = append(details, response.ValidationDetail{Field: "grade", Reason: "required"})
	}
	return details
}

type reviewChildApplicationRequest struct {
	Status            string `json:"status"`
	SchoolClassID     uint64 `json:"school_class_id"`
	StudentID         uint64 `json:"student_id"`
	CreateSchoolClass bool   `json:"create_school_class"`
	ReviewNote        string `json:"review_note"`
}

func (r reviewChildApplicationRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	switch r.Status {
	case ChildApplicationStatusApproved, ChildApplicationStatusRejected, ChildApplicationStatusNeedsInfo:
	default:
		details = append(details, response.ValidationDetail{Field: "status", Reason: "invalid_value"})
	}
	if r.Status == ChildApplicationStatusApproved && r.SchoolClassID == 0 && r.StudentID == 0 {
		// An existing class can be carried by the application; the handler checks
		// that after loading it. This keeps the request compact for invite links.
	}
	return details
}

type parentLoginRequest struct {
	Code     string `json:"code"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	OpenID   string `json:"openid"`
}

type phoneLoginRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

type phoneCodeRequest struct {
	Phone string `json:"phone"`
}

func (r phoneCodeRequest) Validate() []response.ValidationDetail {
	if normalizeLoginPhone(r.Phone) == "" {
		return []response.ValidationDetail{{Field: "phone", Reason: "invalid_value"}}
	}
	return nil
}

func (r phoneLoginRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if normalizeLoginPhone(r.Phone) == "" {
		details = append(details, response.ValidationDetail{Field: "phone", Reason: "required"})
	}
	if strings.TrimSpace(r.Code) == "" {
		details = append(details, response.ValidationDetail{Field: "code", Reason: "required"})
	}
	return details
}

func (r parentLoginRequest) Validate() []response.ValidationDetail {
	if strings.TrimSpace(r.Code) == "" && strings.TrimSpace(r.OpenID) == "" {
		return []response.ValidationDetail{{Field: "code", Reason: "required"}}
	}
	return nil
}

type parentTokenView struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	Principal    string `json:"principal"`
	Role         string `json:"role"`
}

type phoneLoginRoleView struct {
	Key          string `json:"key"`
	Principal    string `json:"principal"`
	Role         string `json:"role"`
	Label        string `json:"label"`
	Available    bool   `json:"available"`
	Message      string `json:"message,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int64  `json:"expiresIn,omitempty"`
}

type phoneLoginView struct {
	Phone       string               `json:"phone"`
	MaskedPhone string               `json:"masked_phone"`
	Roles       []phoneLoginRoleView `json:"roles"`
}

func (r bindStudentRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if r.StudentID == 0 {
		details = append(details, response.ValidationDetail{Field: "student_id", Reason: "required"})
	}
	if strings.TrimSpace(r.GuardianPhone) == "" {
		details = append(details, response.ValidationDetail{Field: "guardian_phone", Reason: "required"})
	}
	return details
}

func (h *Handler) loginWithWeChat(c *gin.Context) {
	if h.tokens == nil {
		response.Error(c, response.Internal(errors.New("家长登录认证未配置")))
		return
	}
	var req parentLoginRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	var err error
	// An OpenID supplied by the client is only a local-development escape hatch.
	// In production the identity must always come from code2Session; otherwise a
	// caller could impersonate another parent by posting an arbitrary OpenID.
	openID := ""
	if h.allowLocalCode {
		openID = strings.TrimSpace(req.OpenID)
	}
	if openID == "" {
		if h.exchanger != nil {
			openID, err = h.exchanger.ExchangeCode(c.Request.Context(), req.Code)
		} else if h.allowLocalCode {
			// Local development uses a stable code supplied by the mini-program
			// adapter. Production requires the WeChat code2Session client above.
			openID = "wechat:" + strings.TrimSpace(req.Code)
		} else {
			err = errors.New("微信登录未配置 code2Session")
		}
		if err != nil {
			response.Error(c, response.BadRequest("微信登录凭证无效", err))
			return
		}
	}
	account, err := h.store.FindAccountByOpenID(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), openID)
	if errors.Is(err, ErrNotFound) {
		account, err = h.store.CreateAccount(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CreateAccountParams{OpenID: openID, Nickname: strings.TrimSpace(req.Nickname), Avatar: strings.TrimSpace(req.Avatar)})
		if errors.Is(err, ErrConflict) {
			account, err = h.store.FindAccountByOpenID(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), openID)
		}
	}
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	if account.Status != AccountStatusActive {
		response.Error(c, response.BadRequest("家长账号已停用", nil))
		return
	}
	pair, err := h.tokens.IssuePair(identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: account.ID, OrganizationID: account.OrganizationID, Role: identity.UserRole("parent")})
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	response.OK(c, parentTokenView{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresIn: pair.ExpiresIn, Principal: string(identity.PrincipalKindParent), Role: "parent"})
}

func (h *Handler) loginWithPhone(c *gin.Context) {
	if h.tokens == nil {
		response.Error(c, response.Internal(errors.New("手机号登录认证未配置")))
		return
	}
	if h.phoneCodeService == nil {
		response.Error(c, response.DependencyUnavailable(errors.New("phone verification service is not configured")))
		return
	}
	if h.phoneCodeService.Local() && !h.allowLocalPhoneCode {
		response.Error(c, response.BadRequest("正式环境未配置短信登录，请使用微信登录或教师工作账号", nil))
		return
	}
	var req phoneLoginRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	phone := normalizeLoginPhone(req.Phone)
	if err := h.phoneCodeService.Verify(c.Request.Context(), phone, req.Code); err != nil {
		h.respondPhoneCodeError(c, err)
		return
	}

	parentAccount, err := h.accountForOpenID(c, "phone:"+phone, "家长"+maskPhoneSuffix(phone), "")
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	if parentAccount.Status != AccountStatusActive {
		response.Error(c, response.BadRequest("家长账号已停用", nil))
		return
	}
	parentPair, err := h.tokens.IssuePair(identity.Principal{Kind: identity.PrincipalKindParent, SubjectID: parentAccount.ID, OrganizationID: parentAccount.OrganizationID, Role: identity.UserRole("parent")})
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}

	roles := []phoneLoginRoleView{{
		Key:          "parent",
		Principal:    string(identity.PrincipalKindParent),
		Role:         "parent",
		Label:        "家长入口",
		Available:    true,
		Message:      "可提交孩子入班申请，审核通过后查看孩子动态",
		AccessToken:  parentPair.AccessToken,
		RefreshToken: parentPair.RefreshToken,
		ExpiresIn:    parentPair.ExpiresIn,
	}}

	staffRole := phoneLoginRoleView{
		Key:       "staff",
		Principal: string(identity.PrincipalKindUser),
		Role:      "teacher",
		Label:     "老师 / 校长入口",
		Available: false,
		Message:   "该手机号未登记为教职工，请联系管理员开通",
	}
	if h.users != nil {
		user, findErr := h.users.FindUserByUsername(c.Request.Context(), phone)
		switch {
		case findErr == nil && user.Status == identity.UserStatusActive:
			userPair, issueErr := h.tokens.IssuePair(identity.Principal{Kind: identity.PrincipalKindUser, SubjectID: user.ID, OrganizationID: user.OrganizationID, Role: user.Role})
			if issueErr != nil {
				response.Error(c, response.Internal(issueErr))
				return
			}
			staffRole.Role = string(user.Role)
			staffRole.Label = staffLoginLabel(user.Role)
			staffRole.Available = true
			staffRole.Message = "手机号已登记，可进入工作台"
			staffRole.AccessToken = userPair.AccessToken
			staffRole.RefreshToken = userPair.RefreshToken
			staffRole.ExpiresIn = userPair.ExpiresIn
		case findErr == nil:
			staffRole.Message = "该教职工账号已停用，请联系管理员"
		case errors.Is(findErr, identity.ErrUserNotFound):
			// Keep the unavailable staff role in the response so the mini-app can
			// show a clear explanation after the user chooses the staff entry.
		default:
			response.Error(c, response.Internal(findErr))
			return
		}
	}
	roles = append(roles, staffRole)

	response.OK(c, phoneLoginView{Phone: phone, MaskedPhone: maskPhone(phone), Roles: roles})
}

type phoneCodeView struct {
	Phone       string `json:"phone"`
	MaskedPhone string `json:"masked_phone"`
	ExpiresIn   int    `json:"expires_in"`
	RetryAfter  int    `json:"retry_after"`
	DebugCode   string `json:"debug_code,omitempty"`
}

func (h *Handler) sendPhoneCode(c *gin.Context) {
	if h.phoneCodeService == nil {
		response.Error(c, response.DependencyUnavailable(errors.New("phone verification service is not configured")))
		return
	}
	if h.phoneCodeService.Local() && !h.allowLocalPhoneCode {
		response.Error(c, response.BadRequest("正式环境未配置短信服务，请联系管理员", nil))
		return
	}
	var req phoneCodeRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	phone := normalizeLoginPhone(req.Phone)
	result, err := h.phoneCodeService.Issue(c.Request.Context(), phone)
	if err != nil {
		h.respondPhoneCodeError(c, err)
		return
	}
	response.OK(c, phoneCodeView{
		Phone:       result.Phone,
		MaskedPhone: maskPhone(result.Phone),
		ExpiresIn:   result.ExpiresIn,
		RetryAfter:  result.RetryAfter,
		DebugCode:   result.DebugCode,
	})
}

func (h *Handler) respondPhoneCodeError(c *gin.Context, err error) {
	var rateLimit *verification.RateLimitError
	switch {
	case errors.As(err, &rateLimit):
		seconds := int(rateLimit.RetryAfter / time.Second)
		if rateLimit.RetryAfter%time.Second != 0 {
			seconds++
		}
		if seconds < 1 {
			seconds = 1
		}
		response.Error(c, response.RateLimited(fmt.Sprintf("请%d秒后再获取验证码", seconds), err))
	case errors.Is(err, verification.ErrInvalidPhone):
		response.Error(c, response.BadRequest("手机号格式不正确", err))
	case errors.Is(err, verification.ErrCodeExpired):
		response.Error(c, response.BadRequest("验证码已过期，请重新获取", err))
	case errors.Is(err, verification.ErrTooManyAttempts):
		response.Error(c, response.BadRequest("验证码错误次数过多，请重新获取", err))
	case errors.Is(err, verification.ErrInvalidCode):
		response.Error(c, response.BadRequest("验证码错误或已过期", err))
	default:
		response.Error(c, response.DependencyUnavailable(err))
	}
}

type createLeaveRequest struct {
	LeaveDate string `json:"leave_date"`
	Reason    string `json:"reason"`
}

type updateLeaveRequest struct {
	LeaveDate string `json:"leave_date"`
	Reason    string `json:"reason"`
}

func (r updateLeaveRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if _, err := parseDate(r.LeaveDate); err != nil {
		details = append(details, response.ValidationDetail{Field: "leave_date", Reason: "date_format"})
	}
	if strings.TrimSpace(r.Reason) == "" {
		details = append(details, response.ValidationDetail{Field: "reason", Reason: "required"})
	}
	return details
}

func (r createLeaveRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if _, err := parseDate(r.LeaveDate); err != nil {
		details = append(details, response.ValidationDetail{Field: "leave_date", Reason: "date_format"})
	}
	if strings.TrimSpace(r.Reason) == "" {
		details = append(details, response.ValidationDetail{Field: "reason", Reason: "required"})
	}
	return details
}

type reviewLeaveRequest struct {
	Status      string `json:"status"`
	TeacherNote string `json:"teacher_note"`
}

func (r reviewLeaveRequest) Validate() []response.ValidationDetail {
	if r.Status != LeaveStatusApproved && r.Status != LeaveStatusRejected {
		return []response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}
	}
	return nil
}

func (h *Handler) bindStudent(c *gin.Context) {
	var req bindStudentRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	var err error
	var account Account
	var ok bool
	_, hasPrincipal := identity.PrincipalFromContext(c.Request.Context())
	if hasPrincipal || strings.TrimSpace(c.GetHeader(ParentOpenIDHeader)) != "" || strings.TrimSpace(c.GetHeader(ParentIDHeader)) != "" {
		account, ok = h.currentAccount(c)
	} else if strings.TrimSpace(req.OpenID) != "" {
		// Compatibility for old local test clients. Production requests use
		// the parent access token and never trust an OpenID from the body.
		account, err = h.accountForOpenID(c, req.OpenID, req.Nickname, req.Avatar)
		if err != nil {
			h.respondStoreError(c, err)
			return
		}
		ok = true
	}
	if !ok {
		return
	}
	student, err := h.masterData.FindStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), req.StudentID)
	if err != nil {
		h.respondMasterError(c, err)
		return
	}
	if strings.TrimSpace(student.GuardianPhone) == "" || strings.TrimSpace(student.GuardianPhone) != strings.TrimSpace(req.GuardianPhone) {
		response.Error(c, response.BadRequest("监护人手机号与学生档案不一致", nil))
		return
	}
	if account.Status != AccountStatusActive {
		response.Error(c, response.BadRequest("家长账号已停用", nil))
		return
	}
	item, err := h.store.CreateBinding(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), BindStudentParams{ParentAccountID: account.ID, StudentID: student.ID, Relationship: defaultString(req.Relationship, "guardian"), IsPrimary: req.IsPrimary})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	item.StudentName, item.SchoolClassID, item.CareClassID = student.Name, student.SchoolClassID, student.CareClassID
	response.Created(c, "/api/v1/parent/students", toBindingView(item))
}

func (h *Handler) createChildApplication(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	var req childApplicationRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	schoolID, schoolClassID, grade, className, err := h.resolveChildApplicationClass(c, req)
	if err != nil {
		h.respondMasterError(c, err)
		return
	}
	existing, err := h.store.ListChildApplications(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), &account.ID)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	for _, item := range existing {
		if item.Status != ChildApplicationStatusPending && item.Status != ChildApplicationStatusNeedsInfo && item.Status != ChildApplicationStatusApproved {
			continue
		}
		if normalizeName(item.StudentName) == normalizeName(req.StudentName) && sameOptionalID(item.SchoolClassID, schoolClassID) {
			response.Error(c, response.BadRequest("已存在相同孩子的入班申请或绑定记录", nil))
			return
		}
	}
	guardianName := strings.TrimSpace(req.GuardianName)
	if guardianName == "" {
		guardianName = strings.TrimSpace(account.Nickname)
	}
	item, err := h.store.CreateChildApplication(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CreateChildApplicationParams{
		ParentAccountID: account.ID,
		StudentName:     strings.TrimSpace(req.StudentName),
		SchoolNameInput: strings.TrimSpace(req.SchoolName),
		GradeInput:      strings.TrimSpace(gradeInput(req, grade)),
		ClassNameInput:  strings.TrimSpace(classInput(req, className)),
		SchoolID:        schoolID,
		SchoolClassID:   schoolClassID,
		Grade:           grade,
		ClassName:       className,
		GuardianName:    guardianName,
		GuardianPhone:   strings.TrimSpace(req.GuardianPhone),
		Relationship:    defaultString(req.Relationship, "家长"),
		Notes:           strings.TrimSpace(req.Notes),
	})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "parent.child_application.create", "child_application", item.ID)
	response.Created(c, "/api/v1/parent/child-applications/"+strconv.FormatUint(item.ID, 10), toChildApplicationView(item, false))
}

func (h *Handler) updateChildApplication(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	id, ok := parsePathValue(c, "id")
	if !ok {
		return
	}
	application, err := h.store.GetChildApplication(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	if application.ParentAccountID != account.ID {
		response.Error(c, response.NotFound())
		return
	}
	var req childApplicationRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if req.SchoolClassID == 0 && application.SchoolClassID != nil && strings.TrimSpace(req.SchoolName) == "" && strings.TrimSpace(req.ClassText) == "" {
		req.SchoolClassID = *application.SchoolClassID
	}
	schoolID, schoolClassID, grade, className, err := h.resolveChildApplicationClass(c, req)
	if err != nil {
		h.respondMasterError(c, err)
		return
	}
	existing, err := h.store.ListChildApplications(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), &account.ID)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	for _, item := range existing {
		if item.ID == id || (item.Status != ChildApplicationStatusPending && item.Status != ChildApplicationStatusNeedsInfo && item.Status != ChildApplicationStatusApproved) {
			continue
		}
		if normalizeName(item.StudentName) == normalizeName(req.StudentName) && sameOptionalID(item.SchoolClassID, schoolClassID) {
			response.Error(c, response.BadRequest("已存在相同孩子的入班申请或绑定记录", nil))
			return
		}
	}
	guardianName := strings.TrimSpace(req.GuardianName)
	if guardianName == "" {
		guardianName = strings.TrimSpace(account.Nickname)
	}
	item, err := h.store.UpdateChildApplication(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), UpdateChildApplicationParams{ID: id, CreateChildApplicationParams: CreateChildApplicationParams{
		ParentAccountID: account.ID,
		StudentName:     strings.TrimSpace(req.StudentName),
		SchoolNameInput: strings.TrimSpace(req.SchoolName),
		GradeInput:      strings.TrimSpace(gradeInput(req, grade)),
		ClassNameInput:  strings.TrimSpace(classInput(req, className)),
		SchoolID:        schoolID,
		SchoolClassID:   schoolClassID,
		Grade:           grade,
		ClassName:       className,
		GuardianName:    guardianName,
		GuardianPhone:   strings.TrimSpace(req.GuardianPhone),
		Relationship:    defaultString(req.Relationship, "家长"),
		Notes:           strings.TrimSpace(req.Notes),
	}})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "parent.child_application.update", "child_application", item.ID)
	response.OK(c, toChildApplicationView(item, false))
}

func (h *Handler) listParentChildApplications(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	items, err := h.store.ListChildApplications(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), &account.ID)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	out := make([]childApplicationView, 0, len(items))
	for _, item := range items {
		out = append(out, toChildApplicationView(item, false))
	}
	response.OK(c, listResponse[childApplicationView]{Items: out, Total: len(out)})
}

func (h *Handler) listStaffChildApplications(c *gin.Context) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser {
		response.Error(c, response.Unauthorized())
		return
	}
	items, err := h.store.ListChildApplications(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), nil)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	if principal.Role == identity.UserRoleTeacher {
		filtered := make([]ChildApplication, 0, len(items))
		for _, item := range items {
			if item.SchoolClassID != nil && h.teacherHasClassAccess(c, principal, *item.SchoolClassID) {
				filtered = append(filtered, item)
				continue
			}
			// An unresolved application may enter a teacher's intake queue only
			// when its school belongs to one of the teacher's assigned classes.
			// Applications without a recognized school stay with an admin so a
			// teacher cannot claim work outside their responsibility boundary.
			if item.SchoolClassID == nil &&
				item.SchoolID != nil &&
				(item.Status == ChildApplicationStatusPending || item.Status == ChildApplicationStatusNeedsInfo) &&
				h.teacherHasSchoolAccess(c, principal, *item.SchoolID) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	out := make([]childApplicationView, 0, len(items))
	for _, item := range items {
		view := toChildApplicationView(item, true)
		h.enrichChildApplicationView(c, item, &view)
		out = append(out, view)
	}
	response.OK(c, listResponse[childApplicationView]{Items: out, Total: len(out)})
}

func (h *Handler) reviewChildApplication(c *gin.Context) {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser {
		response.Error(c, response.Unauthorized())
		return
	}
	id, ok := parsePathValue(c, "id")
	if !ok {
		return
	}
	application, err := h.store.GetChildApplication(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	var req reviewChildApplicationRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if principal.Role == identity.UserRoleTeacher {
		if application.SchoolClassID != nil && !h.teacherHasClassAccess(c, principal, *application.SchoolClassID) {
			response.Error(c, response.Forbidden())
			return
		}
		if req.Status != ChildApplicationStatusApproved &&
			(application.SchoolID == nil || !h.teacherHasSchoolAccess(c, principal, *application.SchoolID)) {
			response.Error(c, response.Forbidden())
			return
		}
	}
	if req.Status != ChildApplicationStatusApproved {
		item, reviewErr := h.store.ReviewChildApplication(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), ReviewChildApplicationParams{ID: id, Status: req.Status, StudentID: application.StudentID, SchoolID: application.SchoolID, SchoolClassID: application.SchoolClassID, ReviewNote: strings.TrimSpace(req.ReviewNote), ReviewedByUserID: principal.SubjectID})
		if reviewErr != nil {
			h.respondStoreError(c, reviewErr)
			return
		}
		h.recordAudit(c, "parent.child_application.review", "child_application", id)
		response.OK(c, toChildApplicationView(item, true))
		return
	}

	schoolClassID := req.SchoolClassID
	if schoolClassID == 0 && application.SchoolClassID != nil {
		schoolClassID = *application.SchoolClassID
	}
	if schoolClassID == 0 && req.CreateSchoolClass {
		var ensureErr error
		schoolClassID, ensureErr = h.ensureApplicationClass(c, principal, application)
		if ensureErr != nil {
			if errors.Is(ensureErr, pickup.ErrUnauthorizedOperation) {
				response.Error(c, response.Forbidden())
				return
			}
			h.respondMasterError(c, ensureErr)
			return
		}
	}
	if schoolClassID == 0 {
		response.Error(c, response.BadRequest("请先选择孩子所在的学校班级", nil))
		return
	}
	classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		h.respondMasterError(c, err)
		return
	}
	var schoolClass masterdata.SchoolClass
	for _, item := range classes {
		if item.ID == schoolClassID && item.Status == "active" {
			schoolClass = item
			break
		}
	}
	if schoolClass.ID == 0 {
		response.Error(c, response.BadRequest("学校班级不存在或已停用", nil))
		return
	}
	if principal.Role == identity.UserRoleTeacher {
		if !h.teacherHasClassAccess(c, principal, schoolClassID) {
			response.Error(c, response.Forbidden())
			return
		}
	}

	students, err := h.masterData.ListStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		h.respondMasterError(c, err)
		return
	}
	var student masterdata.Student
	if req.StudentID != 0 {
		for _, item := range students {
			if item.ID == req.StudentID {
				student = item
				break
			}
		}
		if student.ID == 0 || student.Status != "active" || student.SchoolClassID != schoolClassID || normalizeName(student.Name) != normalizeName(application.StudentName) {
			response.Error(c, response.BadRequest("选择的学生与申请信息不匹配", nil))
			return
		}
	} else {
		matches := make([]masterdata.Student, 0, 2)
		for _, item := range students {
			if item.Status == "active" && item.SchoolClassID == schoolClassID && normalizeName(item.Name) == normalizeName(application.StudentName) {
				matches = append(matches, item)
			}
		}
		switch len(matches) {
		case 0:
			student, err = h.masterData.CreateStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), masterdata.CreateStudentParams{SchoolID: schoolClass.SchoolID, TermID: schoolClass.TermID, SchoolClassID: schoolClass.ID, Name: application.StudentName, Gender: "unknown", GuardianPhone: application.GuardianPhone, Notes: "家长提交入班申请后审核建立"})
			if err != nil {
				h.respondMasterError(c, err)
				return
			}
		case 1:
			student = matches[0]
		default:
			response.Error(c, response.BadRequest("该班存在同名学生，请选择要匹配的学生档案", nil))
			return
		}
	}
	if err := h.createApplicationBinding(c, application, student.ID); err != nil {
		return
	}
	studentID := student.ID
	schoolID := schoolClass.SchoolID
	updated, err := h.store.ReviewChildApplication(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), ReviewChildApplicationParams{ID: id, Status: ChildApplicationStatusApproved, StudentID: &studentID, SchoolID: &schoolID, SchoolClassID: &schoolClassID, ReviewNote: strings.TrimSpace(req.ReviewNote), ReviewedByUserID: principal.SubjectID})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	if h.pickup != nil {
		_, _ = h.pickup.CreateNotification(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), pickup.CreateNotificationParams{StudentID: student.ID, Kind: "child_application_approved", Title: "孩子入班申请已通过", Content: application.StudentName + "已通过审核，现在可以接收接送和作业通知。"})
	}
	h.recordAudit(c, "parent.child_application.approve", "child_application", id)
	response.OK(c, toChildApplicationView(updated, true))
}

func (h *Handler) teacherHasClassAccess(c *gin.Context, principal identity.Principal, schoolClassID uint64) bool {
	if principal.Role != identity.UserRoleTeacher {
		return true
	}
	if h.assignments == nil {
		return false
	}
	item, err := h.assignments.FindByPair(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), principal.SubjectID, schoolClassID)
	return err == nil && item.Status == assignment.AssignmentStatusActive
}

func (h *Handler) teacherHasSchoolAccess(c *gin.Context, principal identity.Principal, schoolID uint64) bool {
	if principal.Role != identity.UserRoleTeacher || h.assignments == nil {
		return false
	}
	classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return false
	}
	assigned, err := h.assignments.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), principal.SubjectID, 0)
	if err != nil {
		return false
	}
	for _, assignmentItem := range assigned {
		if assignmentItem.Status != assignment.AssignmentStatusActive {
			continue
		}
		for _, classItem := range classes {
			if classItem.ID == assignmentItem.SchoolClassID && classItem.Status == "active" && classItem.SchoolID == schoolID {
				return true
			}
		}
	}
	return false
}

func (h *Handler) ensureApplicationClass(c *gin.Context, principal identity.Principal, application ChildApplication) (uint64, error) {
	schoolID := valueOrZero(application.SchoolID)
	schoolName := strings.TrimSpace(application.SchoolNameInput)
	usingFallbackSchool := false
	if schoolName == "" {
		schoolName = "待确认学校"
		usingFallbackSchool = true
	}
	if schoolID == 0 {
		schools, err := h.masterData.ListSchools(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		if err != nil {
			return 0, err
		}
		for _, school := range schools {
			if school.Status == "active" && normalizeSchool(school.Name) == normalizeSchool(schoolName) {
				schoolID = school.ID
				break
			}
		}
		if schoolID == 0 {
			if principal.Role == identity.UserRoleTeacher {
				return 0, pickup.ErrUnauthorizedOperation
			}
			created, createErr := h.masterData.CreateSchool(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), masterdata.CreateSchoolParams{Name: schoolName})
			if createErr != nil && !errors.Is(createErr, masterdata.ErrConflict) {
				return 0, createErr
			}
			if createErr == nil {
				schoolID = created.ID
			} else {
				for _, school := range schools {
					if school.Status == "active" && normalizeSchool(school.Name) == normalizeSchool(schoolName) {
						schoolID = school.ID
						break
					}
				}
			}
		}
	}
	if schoolID == 0 {
		return 0, masterdata.ErrNotFound
	}
	if principal.Role == identity.UserRoleTeacher && (usingFallbackSchool || !h.teacherHasSchoolAccess(c, principal, schoolID)) {
		return 0, pickup.ErrUnauthorizedOperation
	}

	terms, err := h.masterData.ListAcademicTerms(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return 0, err
	}
	var termID uint64
	for _, term := range terms {
		if term.Status == "active" && term.IsCurrent {
			termID = term.ID
			break
		}
	}
	if termID == 0 {
		now := time.Now().UTC()
		termName := fmt.Sprintf("%d-%d学年", now.Year(), now.Year()+1)
		created, createErr := h.masterData.CreateAcademicTerm(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), masterdata.CreateAcademicTermParams{Name: termName, StartsOn: time.Date(now.Year(), 9, 1, 0, 0, 0, 0, time.UTC), EndsOn: time.Date(now.Year()+1, 8, 31, 0, 0, 0, 0, time.UTC), IsCurrent: true})
		if createErr != nil && !errors.Is(createErr, masterdata.ErrConflict) {
			return 0, createErr
		}
		if createErr == nil {
			termID = created.ID
		} else {
			for _, term := range terms {
				if term.Status == "active" && strings.EqualFold(term.Name, termName) {
					termID = term.ID
					break
				}
			}
		}
	}
	if termID == 0 {
		return 0, masterdata.ErrNotFound
	}

	grade := strings.TrimSpace(application.Grade)
	className := strings.TrimSpace(application.ClassName)
	if grade == "" || className == "" {
		parsedGrade, parsedClass := parseClassText(application.ClassNameInput)
		if grade == "" {
			grade = parsedGrade
		}
		if className == "" {
			className = parsedClass
		}
	}
	// A parent may provide a valid but non-standard class description that
	// cannot be split into grade and class by the normalizer. Preserve that
	// text as the class name and keep the application approvable; staff can
	// refine the master data later without blocking enrollment.
	if grade == "" {
		grade = "未分年级"
	}
	if className == "" {
		className = defaultString(application.ClassNameInput, "待确认班级")
	}
	classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return 0, err
	}
	for _, classItem := range classes {
		if classItem.Status == "active" && classItem.SchoolID == schoolID && classItem.TermID == termID && normalizeGrade(classItem.Grade) == normalizeGrade(grade) && normalizeClassName(classItem.Name) == normalizeClassName(className) {
			if principal.Role == identity.UserRoleTeacher && h.assignments != nil {
				_, _ = h.assignments.Create(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), assignment.CreateParams{TeacherUserID: principal.SubjectID, SchoolClassID: classItem.ID})
			}
			return classItem.ID, nil
		}
	}
	created, err := h.masterData.CreateSchoolClass(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), masterdata.CreateSchoolClassParams{SchoolID: schoolID, TermID: termID, Grade: grade, Name: className})
	if errors.Is(err, masterdata.ErrConflict) {
		classes, listErr := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		if listErr != nil {
			return 0, listErr
		}
		for _, classItem := range classes {
			if classItem.Status == "active" && classItem.SchoolID == schoolID && classItem.TermID == termID && normalizeGrade(classItem.Grade) == normalizeGrade(grade) && normalizeClassName(classItem.Name) == normalizeClassName(className) {
				created = classItem
				err = nil
				break
			}
		}
	}
	if err != nil {
		return 0, err
	}
	if principal.Role == identity.UserRoleTeacher && h.assignments != nil {
		if _, assignmentErr := h.assignments.Create(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), assignment.CreateParams{TeacherUserID: principal.SubjectID, SchoolClassID: created.ID}); assignmentErr != nil && !errors.Is(assignmentErr, assignment.ErrConflict) {
			return 0, assignmentErr
		}
	}
	return created.ID, nil
}

func valueOrZero(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}

func (h *Handler) createApplicationBinding(c *gin.Context, application ChildApplication, studentID uint64) error {
	_, err := h.store.CreateBinding(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), BindStudentParams{ParentAccountID: application.ParentAccountID, StudentID: studentID, Relationship: defaultString(application.Relationship, "家长"), IsPrimary: true})
	if err != nil && !errors.Is(err, ErrConflict) {
		h.respondStoreError(c, err)
		return err
	}
	return nil
}

func (h *Handler) resolveChildApplicationClass(c *gin.Context, req childApplicationRequest) (*uint64, *uint64, string, string, error) {
	classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return nil, nil, "", "", err
	}
	if req.SchoolClassID != 0 {
		for _, item := range classes {
			if item.ID == req.SchoolClassID && item.Status == "active" {
				schoolID := item.SchoolID
				classID := item.ID
				return &schoolID, &classID, item.Grade, item.Name, nil
			}
		}
		return nil, nil, "", "", masterdata.ErrNotFound
	}
	grade, className := normalizeGrade(req.Grade), normalizeClassName(req.ClassName)
	if grade == "" || className == "" {
		parsedGrade, parsedClass := parseClassText(req.ClassText)
		if grade == "" {
			grade = parsedGrade
		}
		if className == "" {
			className = parsedClass
		}
	}
	schools, err := h.masterData.ListSchools(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return nil, nil, grade, className, err
	}
	var schoolID *uint64
	for _, item := range schools {
		if item.Status == "active" && normalizeSchool(item.Name) == normalizeSchool(req.SchoolName) {
			value := item.ID
			schoolID = &value
			break
		}
	}
	if schoolID == nil {
		return nil, nil, grade, className, nil
	}
	matches := make([]masterdata.SchoolClass, 0, 2)
	for _, item := range classes {
		if item.Status != "active" || item.SchoolID != *schoolID || normalizeClassName(item.Name) != className {
			continue
		}
		if grade != "" && normalizeGrade(item.Grade) != grade {
			continue
		}
		matches = append(matches, item)
	}
	if len(matches) == 1 {
		classID := matches[0].ID
		return schoolID, &classID, matches[0].Grade, matches[0].Name, nil
	}
	return schoolID, nil, grade, className, nil
}

// enrichChildApplicationView adds only the candidate student records needed
// to resolve same-name students during staff review. It is intentionally
// called only for staff responses; parent responses never expose peers.
func (h *Handler) enrichChildApplicationView(c *gin.Context, item ChildApplication, view *childApplicationView) {
	if view == nil || item.StudentID != nil || item.SchoolClassID == nil || h.masterData == nil {
		return
	}
	students, err := h.masterData.ListStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return
	}
	for _, student := range students {
		if student.Status != "active" ||
			student.SchoolClassID != *item.SchoolClassID ||
			normalizeName(student.Name) != normalizeName(item.StudentName) {
			continue
		}
		view.StudentMatches = append(view.StudentMatches, studentMatchView{
			ID: student.ID, Name: student.Name, GuardianPhone: student.GuardianPhone,
		})
	}
}

func toChildApplicationView(item ChildApplication, includeParentID bool) childApplicationView {
	view := childApplicationView{ID: item.ID, StudentID: item.StudentID, StudentName: item.StudentName, SchoolNameInput: item.SchoolNameInput, GradeInput: item.GradeInput, ClassNameInput: item.ClassNameInput, SchoolID: item.SchoolID, SchoolClassID: item.SchoolClassID, Grade: item.Grade, ClassName: item.ClassName, GuardianName: item.GuardianName, GuardianPhone: item.GuardianPhone, Relationship: item.Relationship, Notes: item.Notes, Status: item.Status, ReviewNote: item.ReviewNote, CreatedAt: item.CreatedAt.Format(time.RFC3339)}
	if includeParentID {
		view.ParentAccountID = item.ParentAccountID
	}
	if item.ReviewedAt != nil {
		value := item.ReviewedAt.Format(time.RFC3339)
		view.ReviewedAt = &value
	}
	return view
}

func (h *Handler) getMe(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	bindings, err := h.store.ListBindings(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), account.ID)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.enrichBindings(c, bindings)
	out := make([]bindingView, 0, len(bindings))
	for _, item := range bindings {
		out = append(out, toBindingView(item))
	}
	response.OK(c, parentMeView{Account: accountView{ID: account.ID, Nickname: account.Nickname, Avatar: account.Avatar, Status: account.Status}, Children: out})
}

func (h *Handler) listStudents(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	bindings, err := h.store.ListBindings(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), account.ID)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.enrichBindings(c, bindings)
	out := make([]bindingView, 0, len(bindings))
	for _, item := range bindings {
		out = append(out, toBindingView(item))
	}
	response.OK(c, listResponse[bindingView]{Items: out, Total: len(out)})
}

func (h *Handler) listStudentPickupEvents(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	studentID, ok := parsePathValue(c, "student_id")
	if !ok {
		return
	}
	if !h.isBound(c, account.ID, studentID) {
		return
	}
	// Store adapters use operation-scoped event queries; collect the student's events
	// from today's and historical operations without exposing other students.
	operations, err := h.pickup.ListOperations(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		h.respondPickupError(c, err)
		return
	}
	type eventView struct {
		ID                 uint64 `json:"id"`
		OperationID        uint64 `json:"operation_id"`
		OperationStudentID uint64 `json:"operation_student_id"`
		StudentID          uint64 `json:"student_id"`
		EventType          string `json:"event_type"`
		EventAt            string `json:"event_at"`
		OperatorName       string `json:"operator_name"`
		PhotoURL           string `json:"photo_url,omitempty"`
		Note               string `json:"note"`
	}
	out := make([]eventView, 0)
	dateFilter := strings.TrimSpace(c.Query("date"))
	var filteredDate time.Time
	if dateFilter != "" {
		var parseErr error
		filteredDate, parseErr = parseDate(dateFilter)
		if parseErr != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
			return
		}
	}
	for _, operation := range operations {
		if dateFilter != "" {
			if !sameDay(operation.OperationDate, filteredDate) {
				continue
			}
		}
		events, eventErr := h.pickup.ListEvents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), operation.ID)
		if eventErr != nil {
			h.respondPickupError(c, eventErr)
			return
		}
		for _, item := range events {
			if item.StudentID != studentID {
				continue
			}
			out = append(out, eventView{ID: item.ID, OperationID: item.OperationID, OperationStudentID: item.OperationStudentID, StudentID: item.StudentID, EventType: item.EventType, EventAt: item.EventAt.Format(time.RFC3339), OperatorName: item.OperatorName, PhotoURL: h.signedPhotoURL(item.PhotoURL), Note: item.Note})
		}
	}
	response.OK(c, listResponse[eventView]{Items: out, Total: len(out)})
}

func (h *Handler) getStudentPickupToday(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	studentID, ok := parsePathValue(c, "student_id")
	if !ok || !h.isBound(c, account.ID, studentID) {
		return
	}
	dateValue := strings.TrimSpace(c.Query("date"))
	if dateValue == "" {
		dateValue = time.Now().UTC().Format("2006-01-02")
	}
	date, err := parseDate(dateValue)
	if err != nil {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "date", Reason: "date_format"}}))
		return
	}
	operations, err := h.pickup.ListOperations(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		h.respondPickupError(c, err)
		return
	}
	for _, operation := range operations {
		if !sameDay(operation.OperationDate, date) {
			continue
		}
		members, listErr := h.pickup.ListOperationStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), operation.ID)
		if listErr != nil {
			h.respondPickupError(c, listErr)
			return
		}
		for _, member := range members {
			if member.StudentID != studentID {
				continue
			}
			teacherName := operation.ExecutingTeacherName
			if strings.TrimSpace(teacherName) == "" {
				teacherName = operation.TeacherName
			}
			classDisplay := h.schoolClassDisplay(c, operation.SchoolClassID)
			response.OK(c, parentPickupTodayView{OperationID: operation.ID, OperationDate: operation.OperationDate.Format("2006-01-02"), SchoolClassID: operation.SchoolClassID, SchoolName: classDisplay.schoolName, Grade: classDisplay.grade, ClassName: classDisplay.className, PickupMode: member.PickupMode, Status: operation.Status, TeacherName: teacherName, TeacherRole: operation.TeacherRole, ExpectedPickupTime: operation.ExpectedPickupTime, StudentStatus: member.Status, PhotoURL: h.signedPhotoURL(member.PhotoURL), ProfilePending: member.ProfilePending})
			return
		}
	}
	response.OK(c, nil)
}

func (h *Handler) signedPhotoURL(value string) string {
	if h.photoSigner == nil || strings.TrimSpace(value) == "" {
		return value
	}
	return h.photoSigner.Sign(value, h.photoURLTTL)
}

func (h *Handler) listNotifications(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	bindings, err := h.store.ListBindings(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), account.ID)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	allowed := make(map[uint64]struct{}, len(bindings))
	for _, item := range bindings {
		allowed[item.StudentID] = struct{}{}
	}
	items, err := h.pickup.ListNotifications(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		h.respondPickupError(c, err)
		return
	}
	type notificationView struct {
		ID          uint64  `json:"id"`
		StudentID   uint64  `json:"student_id"`
		OperationID *uint64 `json:"operation_id,omitempty"`
		EventID     *uint64 `json:"event_id,omitempty"`
		Kind        string  `json:"kind"`
		Title       string  `json:"title"`
		Content     string  `json:"content"`
		Status      string  `json:"status"`
		ReadAt      *string `json:"read_at,omitempty"`
		CreatedAt   string  `json:"created_at"`
	}
	kind := strings.TrimSpace(c.Query("kind"))
	if kind != "" && !isMessageKind(kind) {
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "kind", Reason: "invalid_value"}}))
		return
	}
	limit := 50
	if value := strings.TrimSpace(c.Query("limit")); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil || parsed <= 0 || parsed > 100 {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "limit", Reason: "invalid_value"}}))
			return
		}
		limit = parsed
	}
	var cursor uint64
	if value := strings.TrimSpace(c.Query("cursor")); value != "" {
		parsed, parseErr := strconv.ParseUint(value, 10, 64)
		if parseErr != nil {
			response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "cursor", Reason: "invalid_value"}}))
			return
		}
		cursor = parsed
	}
	unread := 0
	filtered := make([]pickup.Notification, 0, len(items))
	for _, item := range items {
		if _, exists := allowed[item.StudentID]; !exists {
			continue
		}
		if kind != "" && item.Kind != kind {
			continue
		}
		if item.ReadAt == nil {
			unread++
		}
		if cursor != 0 && item.ID >= cursor {
			continue
		}
		filtered = append(filtered, item)
	}
	out := make([]notificationView, 0, minInt(limit, len(filtered)))
	for _, item := range filtered {
		if len(out) >= limit {
			break
		}
		out = append(out, notificationView{ID: item.ID, StudentID: item.StudentID, OperationID: item.OperationID, EventID: item.EventID, Kind: item.Kind, Title: item.Title, Content: item.Content, Status: item.Status, ReadAt: formatTime(item.ReadAt), CreatedAt: item.CreatedAt.Format(time.RFC3339)})
	}
	nextCursor := uint64(0)
	if len(out) == limit {
		nextCursor = out[len(out)-1].ID
	}
	response.OK(c, gin.H{"items": out, "total": len(filtered), "unread": unread, "next_cursor": nextCursor})
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func (h *Handler) listSubscriptions(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	items, err := h.store.ListMessageSubscriptions(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), account.ID)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	out := make([]subscriptionView, 0, len(items))
	for _, item := range items {
		out = append(out, subscriptionView{Kind: item.Kind, Status: item.Status, TemplateVersion: item.TemplateVersion, AuthorizedAt: formatTime(item.AuthorizedAt), UpdatedAt: item.UpdatedAt.Format(time.RFC3339)})
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func (h *Handler) updateSubscriptions(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	var req subscriptionRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	params := make([]UpdateMessageSubscriptionParams, 0, len(req.Subscriptions))
	for _, item := range req.Subscriptions {
		params = append(params, UpdateMessageSubscriptionParams{Kind: strings.TrimSpace(item.Kind), Status: strings.TrimSpace(item.Status), TemplateVersion: strings.TrimSpace(item.TemplateVersion)})
	}
	if err := h.store.UpdateMessageSubscriptions(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), account.ID, params); err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.listSubscriptions(c)
}

func (h *Handler) getPrivacyConsent(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	item, err := h.store.GetLatestPrivacyConsent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), account.ID)
	if errors.Is(err, ErrNotFound) {
		response.OK(c, privacyConsentView{Accepted: false, CurrentPolicyVersion: PrivacyPolicyCurrentVersion, PolicyVersion: PrivacyPolicyCurrentVersion})
		return
	}
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	consentedAt := item.ConsentedAt.Format(time.RFC3339)
	response.OK(c, privacyConsentView{Accepted: item.PolicyVersion == PrivacyPolicyCurrentVersion, PolicyVersion: item.PolicyVersion, CurrentPolicyVersion: PrivacyPolicyCurrentVersion, ConsentedAt: &consentedAt})
}

func (h *Handler) recordPrivacyConsent(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	var req privacyConsentRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if strings.TrimSpace(req.PolicyVersion) != PrivacyPolicyCurrentVersion {
		response.Error(c, response.BadRequest("隐私协议版本已更新，请刷新后重新确认", nil))
		return
	}
	item, err := h.store.RecordPrivacyConsent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), account.ID, RecordPrivacyConsentParams{PolicyVersion: req.PolicyVersion})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	consentedAt := item.ConsentedAt.Format(time.RFC3339)
	response.OK(c, privacyConsentView{Accepted: true, PolicyVersion: item.PolicyVersion, CurrentPolicyVersion: PrivacyPolicyCurrentVersion, ConsentedAt: &consentedAt})
}

func isMessageKind(value string) bool {
	switch strings.TrimSpace(value) {
	case MessageKindPickup, MessageKindMeal, MessageKindHomework, MessageKindLeave, MessageKindSummary:
		return true
	default:
		return false
	}
}

func (h *Handler) markNotificationRead(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	id, ok := parsePathValue(c, "id")
	if !ok {
		return
	}
	bindings, err := h.store.ListBindings(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), account.ID)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	allowed := make(map[uint64]struct{}, len(bindings))
	for _, binding := range bindings {
		allowed[binding.StudentID] = struct{}{}
	}
	notifications, err := h.pickup.ListNotifications(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		h.respondPickupError(c, err)
		return
	}
	found := false
	for _, notification := range notifications {
		if notification.ID == id {
			_, found = allowed[notification.StudentID]
			break
		}
	}
	if !found {
		response.Error(c, response.NotFound())
		return
	}
	if err := h.pickup.MarkNotificationRead(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id); err != nil {
		h.respondPickupError(c, err)
		return
	}
	response.OK(c, gin.H{"id": id, "read": true})
}

func (h *Handler) listParentLeaveRequests(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	items, err := h.store.ListLeaveRequests(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), &account.ID)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.writeLeaveRequests(c, items)
}

func (h *Handler) createLeaveRequest(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	studentID, ok := parsePathValue(c, "student_id")
	if !ok {
		return
	}
	if !h.isBound(c, account.ID, studentID) {
		return
	}
	var req createLeaveRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	leaveDate, _ := parseDate(req.LeaveDate)
	item, err := h.store.CreateLeaveRequest(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CreateLeaveRequestParams{StudentID: studentID, ParentAccountID: account.ID, LeaveDate: leaveDate, Reason: strings.TrimSpace(req.Reason)})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "parent.leave_request.create", "leave_request", item.ID)
	response.Created(c, "/api/v1/parent/leave-requests", toLeaveRequestView(item))
}

func (h *Handler) updateParentLeaveRequest(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	id, ok := parsePathValue(c, "id")
	if !ok {
		return
	}
	var req updateLeaveRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	leaveDate, _ := parseDate(req.LeaveDate)
	item, err := h.store.UpdateLeaveRequest(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), UpdateLeaveRequestParams{ID: id, ParentAccountID: account.ID, LeaveDate: leaveDate, Reason: strings.TrimSpace(req.Reason)})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "parent.leave_request.update", "leave_request", item.ID)
	response.OK(c, toLeaveRequestView(item))
}

func (h *Handler) cancelParentLeaveRequest(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	id, ok := parsePathValue(c, "id")
	if !ok {
		return
	}
	item, err := h.store.CancelLeaveRequest(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CancelLeaveRequestParams{ID: id, ParentAccountID: account.ID})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.recordAudit(c, "parent.leave_request.cancel", "leave_request", item.ID)
	response.OK(c, toLeaveRequestView(item))
}

func (h *Handler) createPickupChangeRequest(c *gin.Context) {
	account, ok := h.currentAccount(c)
	if !ok {
		return
	}
	studentID, ok := parsePathValue(c, "student_id")
	if !ok || !h.isBound(c, account.ID, studentID) {
		return
	}
	var req parentPickupChangeRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	changeDate, _ := parseDate(req.ChangeDate)
	var operationID *uint64
	operations, err := h.pickup.ListOperations(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		h.respondPickupError(c, err)
		return
	}
	for _, operation := range operations {
		if !sameDay(operation.OperationDate, changeDate) {
			continue
		}
		members, membersErr := h.pickup.ListOperationStudents(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), operation.ID)
		if membersErr != nil {
			h.respondPickupError(c, membersErr)
			return
		}
		for _, member := range members {
			if member.StudentID == studentID {
				id := operation.ID
				operationID = &id
				break
			}
		}
		if operationID != nil {
			break
		}
	}
	item, err := h.pickup.CreatePickupChangeRequest(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), pickup.CreatePickupChangeRequestParams{StudentID: studentID, OperationID: operationID, ChangeDate: changeDate, RequestedStatus: req.RequestedStatus, Note: strings.TrimSpace(req.Note), SubmittedBy: "parent"})
	if err != nil {
		h.respondPickupError(c, err)
		return
	}
	response.Created(c, "/api/v1/parent/students/"+strconv.FormatUint(studentID, 10)+"/pickup-changes", gin.H{"id": item.ID, "status": item.Status, "change_date": item.ChangeDate.Format("2006-01-02"), "requested_status": item.RequestedStatus, "note": item.Note})
}

type createTeacherLeaveRequest struct {
	StudentID uint64 `json:"student_id"`
	LeaveDate string `json:"leave_date"`
	Reason    string `json:"reason"`
}

func (r createTeacherLeaveRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 3)
	if r.StudentID == 0 {
		details = append(details, response.ValidationDetail{Field: "student_id", Reason: "required"})
	}
	if _, err := parseDate(r.LeaveDate); err != nil {
		details = append(details, response.ValidationDetail{Field: "leave_date", Reason: "date_format"})
	}
	if strings.TrimSpace(r.Reason) == "" {
		details = append(details, response.ValidationDetail{Field: "reason", Reason: "required"})
	}
	return details
}

func (h *Handler) createTeacherLeaveRequest(c *gin.Context) {
	if !canWriteLeave(c) {
		return
	}
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != identity.PrincipalKindUser {
		response.Error(c, response.Unauthorized())
		return
	}
	var req createTeacherLeaveRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	student, err := h.masterData.FindStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), req.StudentID)
	if err != nil {
		h.respondMasterError(c, err)
		return
	}
	if student.Status != "active" {
		response.Error(c, response.NotFound())
		return
	}
	if principal.Role == identity.UserRoleTeacher {
		if h.assignments == nil {
			response.Error(c, response.Internal(errors.New("教师班级权限未配置")))
			return
		}
		assigned, listErr := h.assignments.List(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), principal.SubjectID, student.SchoolClassID)
		if listErr != nil {
			response.Error(c, response.Internal(listErr))
			return
		}
		allowed := false
		for _, item := range assigned {
			if item.Status == assignment.AssignmentStatusActive {
				allowed = true
				break
			}
		}
		if !allowed {
			response.Error(c, response.BadRequest("当前教师没有负责该学生所在班级", nil))
			return
		}
	}
	writer, ok := h.store.(TeacherLeaveStore)
	if !ok {
		response.Error(c, response.Internal(errors.New("老师代记请假服务未配置")))
		return
	}
	leaveDate, _ := parseDate(req.LeaveDate)
	item, err := writer.CreateTeacherLeaveRequest(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CreateTeacherLeaveRequestParams{StudentID: req.StudentID, SubmittedByUserID: principal.SubjectID, LeaveDate: leaveDate, Reason: strings.TrimSpace(req.Reason)})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	_ = h.notifyLeaveStatus(c, item)
	response.Created(c, "/api/v1/leave-requests/teacher", toLeaveRequestView(item))
}

func (h *Handler) listAllLeaveRequests(c *gin.Context) {
	items, err := h.store.ListLeaveRequests(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), nil)
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	h.writeLeaveRequests(c, items)
}

func (h *Handler) reviewLeaveRequest(c *gin.Context) {
	if !canWriteLeave(c) {
		return
	}
	id, ok := parsePathValue(c, "id")
	if !ok {
		return
	}
	var req reviewLeaveRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.store.ReviewLeaveRequest(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), ReviewLeaveRequestParams{ID: id, Status: req.Status, TeacherNote: strings.TrimSpace(req.TeacherNote), ReviewedByUserID: parseTeacherID(c)})
	if err != nil {
		h.respondStoreError(c, err)
		return
	}
	_ = h.notifyLeaveStatus(c, item)
	h.recordAudit(c, "leave_request.review", "leave_request", item.ID)
	response.OK(c, toLeaveRequestView(item))
}

func (h *Handler) accountForOpenID(c *gin.Context, openID, nickname, avatar string) (Account, error) {
	account, err := h.store.FindAccountByOpenID(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), strings.TrimSpace(openID))
	if err == nil {
		return account, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Account{}, err
	}
	account, err = h.store.CreateAccount(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), CreateAccountParams{OpenID: openID, Nickname: nickname, Avatar: avatar})
	if errors.Is(err, ErrConflict) {
		return h.store.FindAccountByOpenID(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), strings.TrimSpace(openID))
	}
	return account, err
}

func (h *Handler) currentAccount(c *gin.Context) (Account, bool) {
	if principal, ok := identity.PrincipalFromContext(c.Request.Context()); ok {
		if principal.Kind != identity.PrincipalKindParent {
			response.Error(c, response.Unauthorized())
			return Account{}, false
		}
		account, err := h.store.FindAccountByID(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), principal.SubjectID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				response.Error(c, response.Unauthorized())
				return Account{}, false
			}
			h.respondStoreError(c, err)
			return Account{}, false
		}
		if account.Status != AccountStatusActive {
			response.Error(c, response.BadRequest("家长账号已停用", nil))
			return Account{}, false
		}
		return account, true
	}
	openID := strings.TrimSpace(c.GetHeader(ParentOpenIDHeader))
	if openID != "" {
		account, err := h.store.FindAccountByOpenID(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), openID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				response.Error(c, response.Unauthorized())
				return Account{}, false
			}
			h.respondStoreError(c, err)
			return Account{}, false
		}
		if account.Status != AccountStatusActive {
			response.Error(c, response.BadRequest("家长账号已停用", nil))
			return Account{}, false
		}
		return account, true
	}
	if rawID := strings.TrimSpace(c.GetHeader(ParentIDHeader)); rawID != "" {
		id, err := strconv.ParseUint(rawID, 10, 64)
		if err != nil || id == 0 {
			response.Error(c, response.BadRequest("家长身份不合法", err))
			return Account{}, false
		}
		account, err := h.store.FindAccountByID(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				response.Error(c, response.Unauthorized())
				return Account{}, false
			}
			h.respondStoreError(c, err)
			return Account{}, false
		}
		if account.Status != AccountStatusActive {
			response.Error(c, response.BadRequest("家长账号已停用", nil))
			return Account{}, false
		}
		return account, true
	}
	response.Error(c, response.BadRequest("缺少家长身份，请提供 X-Parent-OpenID", nil))
	return Account{}, false
}

func (h *Handler) isBound(c *gin.Context, parentID, studentID uint64) bool {
	items, err := h.store.ListBindings(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), parentID)
	if err != nil {
		h.respondStoreError(c, err)
		return false
	}
	for _, item := range items {
		if item.StudentID == studentID {
			return true
		}
	}
	response.Error(c, response.NotFound())
	return false
}

func (h *Handler) enrichBindings(c *gin.Context, items []Binding) {
	schools, _ := h.masterData.ListSchools(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	schoolNames := make(map[uint64]string, len(schools))
	for _, school := range schools {
		schoolNames[school.ID] = school.Name
	}
	classes, _ := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	classByID := make(map[uint64]masterdata.SchoolClass, len(classes))
	for _, schoolClass := range classes {
		classByID[schoolClass.ID] = schoolClass
	}
	careClasses, _ := h.masterData.ListCareClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	careClassNames := make(map[uint64]string, len(careClasses))
	for _, careClass := range careClasses {
		careClassNames[careClass.ID] = careClass.Name
	}
	for index := range items {
		student, err := h.masterData.FindStudent(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), items[index].StudentID)
		if err != nil {
			continue
		}
		items[index].StudentName = student.Name
		items[index].SchoolClassID = student.SchoolClassID
		items[index].CareClassID = student.CareClassID
		if schoolClass, exists := classByID[student.SchoolClassID]; exists {
			items[index].SchoolName = schoolNames[schoolClass.SchoolID]
			items[index].Grade = schoolClass.Grade
			items[index].ClassName = schoolClass.Name
		}
		if student.CareClassID != nil {
			items[index].CareClassName = careClassNames[*student.CareClassID]
		}
	}
}

type schoolClassDisplay struct {
	schoolName string
	grade      string
	className  string
}

func (h *Handler) schoolClassDisplay(c *gin.Context, schoolClassID uint64) schoolClassDisplay {
	classes, err := h.masterData.ListSchoolClasses(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
	if err != nil {
		return schoolClassDisplay{}
	}
	for _, schoolClass := range classes {
		if schoolClass.ID != schoolClassID {
			continue
		}
		schools, schoolErr := h.masterData.ListSchools(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID))
		if schoolErr != nil {
			return schoolClassDisplay{grade: schoolClass.Grade, className: schoolClass.Name}
		}
		for _, school := range schools {
			if school.ID == schoolClass.SchoolID {
				return schoolClassDisplay{schoolName: school.Name, grade: schoolClass.Grade, className: schoolClass.Name}
			}
		}
		return schoolClassDisplay{grade: schoolClass.Grade, className: schoolClass.Name}
	}
	return schoolClassDisplay{}
}

func (h *Handler) writeLeaveRequests(c *gin.Context, items []LeaveRequest) {
	out := make([]leaveRequestView, 0, len(items))
	for _, item := range items {
		out = append(out, toLeaveRequestView(item))
	}
	response.OK(c, listResponse[leaveRequestView]{Items: out, Total: len(out)})
}

func toBindingView(item Binding) bindingView {
	return bindingView{ID: item.ID, StudentID: item.StudentID, StudentName: item.StudentName, SchoolClassID: item.SchoolClassID, CareClassID: item.CareClassID, SchoolName: item.SchoolName, Grade: item.Grade, ClassName: item.ClassName, CareClassName: item.CareClassName, Relationship: item.Relationship, IsPrimary: item.IsPrimary}
}
func toLeaveRequestView(item LeaveRequest) leaveRequestView {
	return leaveRequestView{ID: item.ID, StudentID: item.StudentID, ParentAccountID: item.ParentAccountID, SubmittedByType: item.SubmittedByType, LeaveDate: item.LeaveDate.Format("2006-01-02"), Reason: item.Reason, Status: item.Status, TeacherNote: item.TeacherNote, ReviewedAt: formatTime(item.ReviewedAt), CreatedAt: item.CreatedAt.Format(time.RFC3339)}
}
func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}
func parseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.UTC)
}
func sameDay(left, right time.Time) bool {
	left, right = left.UTC(), right.UTC()
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}
func parsePathValue(c *gin.Context, key string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || value == 0 {
		response.Error(c, response.BadRequest(fmt.Sprintf("%s 不合法", key), err))
		return 0, false
	}
	return value, true
}

func gradeInput(req childApplicationRequest, normalized string) string {
	if strings.TrimSpace(req.Grade) != "" {
		return strings.TrimSpace(req.Grade)
	}
	if strings.TrimSpace(req.ClassText) != "" && strings.TrimSpace(req.ClassName) == "" {
		parsed, _ := parseClassText(req.ClassText)
		if parsed != "" {
			return parsed
		}
	}
	return normalized
}

func classInput(req childApplicationRequest, normalized string) string {
	if strings.TrimSpace(req.ClassName) != "" {
		return strings.TrimSpace(req.ClassName)
	}
	if strings.TrimSpace(req.ClassText) != "" {
		_, parsed := parseClassText(req.ClassText)
		if parsed != "" {
			return parsed
		}
		return strings.TrimSpace(req.ClassText)
	}
	return normalized
}

func normalizeName(value string) string {
	return strings.NewReplacer(" ", "", "　", "", "·", "", ".", "").Replace(strings.TrimSpace(value))
}

func normalizeSchool(value string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", "　", "", "·", "", ".", "", "-", "", "—", "").Replace(strings.TrimSpace(value)))
}

func normalizeGrade(value string) string {
	value = strings.NewReplacer(" ", "", "　", "", "（", "(", "）", ")", "年纪", "年级").Replace(strings.TrimSpace(value))
	for number, name := range map[int]string{1: "一", 2: "二", 3: "三", 4: "四", 5: "五", 6: "六"} {
		if strings.Contains(value, name+"年级") || strings.Contains(value, fmt.Sprintf("%d年级", number)) || strings.Contains(value, fmt.Sprintf("%d年纪", number)) || strings.HasPrefix(value, name+"(") || strings.HasPrefix(value, fmt.Sprintf("%d(", number)) {
			return name + "年级"
		}
	}
	return ""
}

var classNumberPattern = regexp.MustCompile(`([0-9]{1,2})\)?班`)
var chineseClassNumberPattern = regexp.MustCompile(`([一二三四五六七八九十]{1,3})\)?班`)

func normalizeClassName(value string) string {
	value = strings.NewReplacer(" ", "", "　", "", "（", "(", "）", ")").Replace(strings.TrimSpace(value))
	if match := classNumberPattern.FindStringSubmatch(value); len(match) == 2 {
		return match[1] + "班"
	}
	if match := chineseClassNumberPattern.FindStringSubmatch(value); len(match) == 2 {
		if number, ok := chineseNumber(match[1]); ok {
			return strconv.Itoa(number) + "班"
		}
	}
	return ""
}

func chineseNumber(value string) (int, bool) {
	if len([]rune(value)) == 1 {
		for number, name := range map[int]string{1: "一", 2: "二", 3: "三", 4: "四", 5: "五", 6: "六", 7: "七", 8: "八", 9: "九"} {
			if value == name {
				return number, true
			}
		}
		if value == "十" {
			return 10, true
		}
		return 0, false
	}
	if value == "十一" {
		return 11, true
	}
	if value == "十二" {
		return 12, true
	}
	if value == "十三" {
		return 13, true
	}
	if value == "十四" {
		return 14, true
	}
	if value == "十五" {
		return 15, true
	}
	return 0, false
}

func parseClassText(value string) (string, string) {
	return normalizeGrade(value), normalizeClassName(value)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func normalizeLoginPhone(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, char := range value {
		if char >= '0' && char <= '9' {
			builder.WriteRune(char)
		}
	}
	normalized := builder.String()
	if len(normalized) < 7 || len(normalized) > 32 {
		return ""
	}
	return normalized
}

func maskPhone(value string) string {
	phone := normalizeLoginPhone(value)
	if len(phone) <= 4 {
		return phone
	}
	prefixLength := 3
	if len(phone) < 7 {
		prefixLength = 1
	}
	return phone[:prefixLength] + "****" + phone[len(phone)-4:]
}

func maskPhoneSuffix(value string) string {
	phone := normalizeLoginPhone(value)
	if len(phone) <= 4 {
		return phone
	}
	return phone[len(phone)-4:]
}

func staffLoginLabel(role identity.UserRole) string {
	switch role {
	case identity.UserRoleAdmin:
		return "管理员入口"
	case identity.UserRoleEditor:
		return "校长 / 管理入口"
	case identity.UserRoleViewer:
		return "查看账号入口"
	default:
		return "老师 / 校长入口"
	}
}

func parseTeacherID(c *gin.Context) uint64 {
	if principal, ok := identity.PrincipalFromContext(c.Request.Context()); ok && principal.Kind == identity.PrincipalKindUser {
		return principal.SubjectID
	}
	value, _ := strconv.ParseUint(strings.TrimSpace(c.GetHeader("X-Teacher-User-ID")), 10, 64)
	return value
}

type listResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

func (h *Handler) respondMasterError(c *gin.Context, err error) {
	if errors.Is(err, masterdata.ErrNotFound) {
		response.Error(c, response.NotFound())
		return
	}
	response.Error(c, response.Internal(err))
}
func (h *Handler) respondStoreError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrConflict):
		response.Error(c, response.BadRequest("资源已存在", err))
	case errors.Is(err, ErrInvalidState):
		response.Error(c, response.BadRequest("当前请假状态不允许此操作", err))
	case errors.Is(err, ErrInvalidStatus):
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "status", Reason: "invalid_value"}}))
	case errors.Is(err, ErrNotFound):
		response.Error(c, response.NotFound())
	default:
		response.Error(c, response.Internal(err))
	}
}
func (h *Handler) respondPickupError(c *gin.Context, err error) {
	if errors.Is(err, pickup.ErrNotFound) {
		response.Error(c, response.NotFound())
		return
	}
	response.Error(c, response.Internal(err))
}

func canWriteLeave(c *gin.Context) bool {
	principal, ok := identity.PrincipalFromContext(c.Request.Context())
	if !ok {
		return true
	}
	if principal.Kind != identity.PrincipalKindUser || principal.Role == identity.UserRoleViewer {
		response.Error(c, response.Forbidden())
		return false
	}
	return true
}

func (h *Handler) notifyLeaveStatus(c *gin.Context, item LeaveRequest) error {
	if h.pickup == nil {
		return nil
	}
	title := "请假申请已更新"
	statusText := "已更新"
	if item.Status == LeaveStatusApproved {
		title = "请假已同意"
		statusText = "已同意"
	} else if item.Status == LeaveStatusRejected {
		title = "请假未同意"
		statusText = "未同意"
	}
	content := item.LeaveDate.Format("2006-01-02") + "：请假申请" + statusText
	if strings.TrimSpace(item.TeacherNote) != "" {
		content += "；老师备注：" + strings.TrimSpace(item.TeacherNote)
	}
	_, err := h.pickup.CreateNotification(c.Request.Context(), identity.OrganizationIDFromContext(c.Request.Context(), h.orgID), pickup.CreateNotificationParams{StudentID: item.StudentID, Kind: "leave_review", Title: title, Content: content})
	return err
}
