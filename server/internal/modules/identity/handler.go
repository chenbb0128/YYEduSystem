package identity

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/request"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

type Handler struct {
	users  UserStore
	tokens *TokenManager
}

func NewHandler(users UserStore, tokens *TokenManager) *Handler {
	return &Handler{users: users, tokens: tokens}
}

// RegisterRoutes is kept as a convenient all-in-one registration for focused
// handler tests. The application uses the split registrations below so that
// staff and parent endpoints are protected by different route groups.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	h.RegisterPublicRoutes(api)
	h.RegisterAuthenticatedRoutes(api)
	h.RegisterStaffRoutes(api)
}

func (h *Handler) RegisterPublicRoutes(api *gin.RouterGroup) {
	api.POST("/auth/login", h.login)
	api.POST("/auth/refresh", h.refresh)
}

func (h *Handler) RegisterAuthenticatedRoutes(api *gin.RouterGroup) {
	api.GET("/auth/me", h.me)
	api.GET("/auth/codes", h.codes)
	api.POST("/auth/logout", h.logout)
}

func (h *Handler) RegisterStaffRoutes(api *gin.RouterGroup) {
	api.GET("/system/users", h.listUsers)
	api.POST("/system/users", h.createUser)
	api.PUT("/system/users/:id", h.updateUser)
	api.DELETE("/system/users/:id", h.disableUser)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r loginRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 2)
	if strings.TrimSpace(r.Username) == "" {
		details = append(details, response.ValidationDetail{Field: "username", Reason: "required"})
	}
	if r.Password == "" {
		details = append(details, response.ValidationDetail{Field: "password", Reason: "required"})
	}
	return details
}

type tokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
	Principal    string `json:"principal"`
	Role         string `json:"role"`
}

type userView struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	RealName  string `json:"realName"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type authMeView struct {
	ID        uint64   `json:"id"`
	Username  string   `json:"username,omitempty"`
	RealName  string   `json:"realName"`
	Nickname  string   `json:"nickname,omitempty"`
	Avatar    string   `json:"avatar,omitempty"`
	Role      string   `json:"role"`
	Roles     []string `json:"roles"`
	Principal string   `json:"principal"`
	HomePath  string   `json:"homePath"`
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	RealName string `json:"realName"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

func (r createUserRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 4)
	if len(strings.TrimSpace(r.Username)) < 3 || len(strings.TrimSpace(r.Username)) > 64 {
		details = append(details, response.ValidationDetail{Field: "username", Reason: "length"})
	}
	if len(r.Password) < 6 {
		details = append(details, response.ValidationDetail{Field: "password", Reason: "min_length_6"})
	}
	if !validRole(UserRole(strings.TrimSpace(r.Role))) {
		details = append(details, response.ValidationDetail{Field: "role", Reason: "invalid_value"})
	}
	if r.Status != "" && r.Status != string(UserStatusActive) && r.Status != string(UserStatusDisabled) {
		details = append(details, response.ValidationDetail{Field: "status", Reason: "invalid_value"})
	}
	return details
}

type updateUserRequest struct {
	Password string `json:"password"`
	RealName string `json:"realName"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

func (r updateUserRequest) Validate() []response.ValidationDetail {
	details := make([]response.ValidationDetail, 0, 3)
	if r.Password != "" && len(r.Password) < 6 {
		details = append(details, response.ValidationDetail{Field: "password", Reason: "min_length_6"})
	}
	if !validRole(UserRole(strings.TrimSpace(r.Role))) {
		details = append(details, response.ValidationDetail{Field: "role", Reason: "invalid_value"})
	}
	if r.Status != string(UserStatusActive) && r.Status != string(UserStatusDisabled) {
		details = append(details, response.ValidationDetail{Field: "status", Reason: "invalid_value"})
	}
	return details
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	user, err := h.users.FindUserByUsername(c.Request.Context(), strings.TrimSpace(req.Username))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		response.Error(c, response.BadRequest("用户名或密码错误", ErrInvalidCredentials))
		return
	}
	if user.Status != UserStatusActive {
		response.Error(c, response.BadRequest("账号已停用", ErrUserDisabled))
		return
	}
	if !IsPlatformAdmin(user.Role) && user.OrganizationStatus != "" && user.OrganizationStatus != "active" {
		response.Error(c, response.BadRequest("所属机构暂不可用", ErrUserDisabled))
		return
	}
	pair, err := h.tokens.IssuePair(Principal{Kind: PrincipalKindUser, SubjectID: user.ID, OrganizationID: user.OrganizationID, Role: user.Role})
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	response.OK(c, tokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresIn: pair.ExpiresIn, Principal: string(PrincipalKindUser), Role: string(user.Role)})
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (r refreshRequest) Validate() []response.ValidationDetail {
	if strings.TrimSpace(r.RefreshToken) == "" {
		return []response.ValidationDetail{{Field: "refreshToken", Reason: "required"}}
	}
	return nil
}

func (h *Handler) refresh(c *gin.Context) {
	var req refreshRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	principal, err := h.tokens.ParseRefresh(strings.TrimSpace(req.RefreshToken))
	if err != nil {
		response.Error(c, response.Unauthorized())
		return
	}
	if principal.Kind == PrincipalKindUser {
		user, findErr := h.users.FindUserByID(c.Request.Context(), principal.SubjectID)
		if findErr != nil || user.Status != UserStatusActive || (!IsPlatformAdmin(user.Role) && user.OrganizationStatus != "" && user.OrganizationStatus != "active") {
			response.Error(c, response.Unauthorized())
			return
		}
		principal.Role = user.Role
		principal.OrganizationID = user.OrganizationID
	} else if principal.Kind != PrincipalKindParent {
		response.Error(c, response.Unauthorized())
		return
	}
	pair, err := h.tokens.IssuePair(principal)
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	response.OK(c, tokenResponse{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresIn: pair.ExpiresIn, Principal: string(principal.Kind), Role: string(principal.Role)})
}

func (h *Handler) me(c *gin.Context) {
	principal, ok := PrincipalFromContext(c.Request.Context())
	if !ok {
		response.Error(c, response.Unauthorized())
		return
	}
	if principal.Kind == PrincipalKindParent {
		response.OK(c, authMeView{ID: principal.SubjectID, Role: "parent", Principal: string(PrincipalKindParent), HomePath: "/pages/parent/index"})
		return
	}
	user, err := h.users.FindUserByID(c.Request.Context(), principal.SubjectID)
	if err != nil || user.Status != UserStatusActive {
		response.Error(c, response.Unauthorized())
		return
	}
	response.OK(c, authMeView{ID: user.ID, Username: user.Username, RealName: user.Nickname, Nickname: user.Nickname, Avatar: user.Avatar, Role: string(user.Role), Roles: []string{string(user.Role)}, Principal: string(PrincipalKindUser), HomePath: homePath(user.Role)})
}

func (h *Handler) codes(c *gin.Context) {
	principal, ok := PrincipalFromContext(c.Request.Context())
	if !ok {
		response.Error(c, response.Unauthorized())
		return
	}
	if principal.Kind != PrincipalKindUser {
		response.OK(c, []string{})
		return
	}
	switch principal.Role {
	case UserRolePlatformAdmin:
		response.OK(c, []string{"platform:dashboard", "platform:organizations:view", "platform:organizations:write", "platform:invites:view", "platform:invites:write", "platform:registrations:review"})
	case UserRoleAdmin:
		response.OK(c, []string{"dashboard:view", "master-data:view", "master-data:write", "pickup:view", "pickup:write", "homework:view", "homework:write", "meal:view", "meal:write", "summary:view", "summary:write", "leave:review", "child-applications:view", "assignment:view", "assignment:write", "notification:view", "notification:retry", "system:user:create", "system:user:update", "system:user:delete"})
	case UserRoleEditor:
		response.OK(c, []string{"dashboard:view", "master-data:view", "master-data:write", "pickup:view", "pickup:write", "homework:view", "homework:write", "meal:view", "meal:write", "summary:view", "summary:write", "leave:review", "child-applications:view", "assignment:view", "assignment:write", "notification:view", "notification:retry"})
	case UserRoleViewer:
		response.OK(c, []string{"dashboard:view", "master-data:view", "pickup:view", "homework:view", "meal:view", "summary:view", "assignment:view"})
	default:
		response.OK(c, []string{"dashboard:view", "master-data:view", "pickup:view", "homework:view", "homework:write", "meal:view", "meal:write", "summary:view", "summary:write", "leave:review", "child-applications:view", "assignment:view"})
	}
}

func (h *Handler) logout(c *gin.Context) { response.OK(c, true) }

func (h *Handler) listUsers(c *gin.Context) {
	if _, ok := h.staffPrincipal(c); !ok {
		return
	}
	users, err := h.users.ListUsers(c.Request.Context())
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	principal, _ := h.staffPrincipal(c)
	keyword := strings.ToLower(strings.TrimSpace(c.Query("keyword")))
	status := strings.TrimSpace(c.Query("status"))
	out := make([]userView, 0, len(users))
	for _, user := range users {
		// Platform administrators belong to the platform owner, not to an
		// institution's staff directory. Never expose or mutate them through
		// the organization user-management endpoints.
		if IsPlatformAdmin(user.Role) {
			continue
		}
		if user.OrganizationID != 0 && user.OrganizationID != principal.OrganizationID {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(user.Username), keyword) && !strings.Contains(strings.ToLower(user.Nickname), keyword) {
			continue
		}
		if status != "" && string(user.Status) != status {
			continue
		}
		out = append(out, toUserView(user))
	}
	response.OK(c, gin.H{"items": out, "total": len(out)})
}

func (h *Handler) createUser(c *gin.Context) {
	if _, ok := h.staffPrincipal(c); !ok {
		return
	}
	var req createUserRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	if IsPlatformAdmin(UserRole(strings.TrimSpace(req.Role))) {
		response.Error(c, response.Forbidden())
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Error(c, response.Internal(err))
		return
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = strings.TrimSpace(req.RealName)
	}
	status := UserStatus(req.Status)
	if status == "" {
		status = UserStatusActive
	}
	principal, _ := h.staffPrincipal(c)
	user, err := h.users.CreateUser(c.Request.Context(), CreateUserParams{OrganizationID: principal.OrganizationID, Username: strings.TrimSpace(req.Username), PasswordHash: string(passwordHash), Role: UserRole(strings.TrimSpace(req.Role)), Nickname: nickname, Status: status})
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.Created(c, "/api/v1/system/users/"+strconv.FormatUint(user.ID, 10), toUserView(user))
}

func (h *Handler) updateUser(c *gin.Context) {
	if _, ok := h.staffPrincipal(c); !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, response.BadRequest("用户 ID 不合法", err))
		return
	}
	var req updateUserRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, err)
		return
	}
	principal, _ := h.staffPrincipal(c)
	current, err := h.users.FindUserByID(c.Request.Context(), id)
	if err != nil {
		respondUserError(c, err)
		return
	}
	if current.OrganizationID != 0 && current.OrganizationID != principal.OrganizationID {
		response.Error(c, response.NotFound())
		return
	}
	if IsPlatformAdmin(current.Role) || IsPlatformAdmin(UserRole(strings.TrimSpace(req.Role))) {
		response.Error(c, response.Forbidden())
		return
	}
	passwordHash := current.PasswordHash
	if req.Password != "" {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			response.Error(c, response.Internal(hashErr))
			return
		}
		passwordHash = string(hash)
	}
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = strings.TrimSpace(req.RealName)
	}
	if nickname == "" {
		nickname = current.Nickname
	}
	if err := h.users.UpdateUser(c.Request.Context(), UpdateUserParams{ID: id, PasswordHash: passwordHash, Role: UserRole(strings.TrimSpace(req.Role)), Nickname: nickname, Avatar: current.Avatar, Status: UserStatus(req.Status)}); err != nil {
		respondUserError(c, err)
		return
	}
	updated, err := h.users.FindUserByID(c.Request.Context(), id)
	if err != nil {
		respondUserError(c, err)
		return
	}
	response.OK(c, toUserView(updated))
}

func (h *Handler) disableUser(c *gin.Context) {
	if _, ok := h.staffPrincipal(c); !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, response.BadRequest("用户 ID 不合法", err))
		return
	}
	principal, _ := h.staffPrincipal(c)
	current, findErr := h.users.FindUserByID(c.Request.Context(), id)
	if findErr != nil || (current.OrganizationID != 0 && current.OrganizationID != principal.OrganizationID) {
		response.Error(c, response.NotFound())
		return
	}
	if IsPlatformAdmin(current.Role) {
		response.Error(c, response.Forbidden())
		return
	}
	if current.Username == "admin" && principal.OrganizationID == 1 {
		response.Error(c, response.BadRequest("不能停用初始管理员", nil))
		return
	}
	if err := h.users.SetUserStatus(c.Request.Context(), SetUserStatusParams{ID: id, Status: UserStatusDisabled}); err != nil {
		respondUserError(c, err)
		return
	}
	response.OK(c, true)
}

func (h *Handler) staffPrincipal(c *gin.Context) (Principal, bool) {
	principal, ok := PrincipalFromContext(c.Request.Context())
	if !ok || principal.Kind != PrincipalKindUser {
		response.Error(c, response.Unauthorized())
		return Principal{}, false
	}
	if principal.Role != UserRoleAdmin && principal.Role != UserRoleEditor {
		response.Error(c, response.BadRequest("没有操作用户的权限", nil))
		return Principal{}, false
	}
	return principal, true
}

func toUserView(user User) userView {
	return userView{ID: user.ID, Username: user.Username, RealName: user.Nickname, Role: string(user.Role), Status: string(user.Status), CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00")}
}

func validRole(role UserRole) bool {
	return role == UserRoleAdmin || role == UserRoleTeacher || role == UserRoleEditor || role == UserRoleViewer
}

func homePath(role UserRole) string {
	if role == UserRolePlatformAdmin {
		return "/platform"
	}
	if role == UserRoleTeacher {
		return "/dashboard"
	}
	return "/dashboard"
}

func respondUserError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUsernameTaken):
		response.Error(c, response.BadRequest("用户名已存在", err))
	case errors.Is(err, ErrUserNotFound):
		response.Error(c, response.NotFound())
	case errors.Is(err, ErrInvalidRole):
		response.Error(c, response.ValidationFailed([]response.ValidationDetail{{Field: "role", Reason: "invalid_value"}}))
	default:
		response.Error(c, response.Internal(err))
	}
}
