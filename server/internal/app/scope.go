package app

import (
	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/modules/assignment"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/identity"
	"github.com/chenbb0128/tuoguan-system-server/internal/modules/masterdata"
	"github.com/chenbb0128/tuoguan-system-server/internal/transport/httpapi/response"
)

func teacherMasterDataScope(assignments assignment.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := identity.PrincipalFromContext(c.Request.Context())
		if !ok || principal.Kind != identity.PrincipalKindUser || principal.Role != identity.UserRoleTeacher {
			c.Next()
			return
		}
		if assignments == nil {
			c.Request = c.Request.WithContext(masterdata.WithSchoolClassScope(c.Request.Context(), nil))
			c.Next()
			return
		}
		items, err := assignments.List(c.Request.Context(), principal.OrganizationID, principal.SubjectID, 0)
		if err != nil {
			response.Error(c, response.Internal(err))
			return
		}
		classIDs := make([]uint64, 0, len(items))
		for _, item := range items {
			if item.Status == assignment.AssignmentStatusActive {
				classIDs = append(classIDs, item.SchoolClassID)
			}
		}
		c.Request = c.Request.WithContext(masterdata.WithSchoolClassScope(c.Request.Context(), classIDs))
		c.Next()
	}
}
