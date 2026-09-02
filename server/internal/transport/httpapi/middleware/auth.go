package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

func Authenticate(tokens *identity.TokenManager, userStores ...identity.UserStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tokens == nil {
			response.Error(c, response.Internal(identity.ErrInvalidToken))
			return
		}
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			response.Error(c, response.Unauthorized())
			return
		}
		principal, err := tokens.ParseAccess(strings.TrimSpace(parts[1]))
		if err != nil {
			response.Error(c, response.Unauthorized())
			return
		}
		if principal.Kind == identity.PrincipalKindUser && len(userStores) > 0 && userStores[0] != nil {
			user, findErr := userStores[0].FindUserByID(c.Request.Context(), principal.SubjectID)
			if findErr != nil || user.Status != identity.UserStatusActive {
				response.Error(c, response.Unauthorized())
				return
			}
			principal.Role = user.Role
		}
		ctx := identity.WithPrincipal(c.Request.Context(), principal)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func RequireStaff() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := identity.PrincipalFromContext(c.Request.Context())
		if !ok || principal.Kind != identity.PrincipalKindUser {
			response.Error(c, response.Unauthorized())
			return
		}
		c.Next()
	}
}

func RequireManager() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := identity.PrincipalFromContext(c.Request.Context())
		if !ok || principal.Kind != identity.PrincipalKindUser {
			response.Error(c, response.Unauthorized())
			return
		}
		if principal.Role != identity.UserRoleAdmin && principal.Role != identity.UserRoleEditor {
			response.Error(c, response.Forbidden())
			return
		}
		c.Next()
	}
}

func RequireParent() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := identity.PrincipalFromContext(c.Request.Context())
		if !ok || principal.Kind != identity.PrincipalKindParent {
			response.Error(c, response.Unauthorized())
			return
		}
		c.Next()
	}
}
