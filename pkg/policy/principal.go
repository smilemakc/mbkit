package policy

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/pkg/roles"
)

// Principal describes user identity and roles
type Principal struct {
	UserID   string         // UserID represents the unique identifier of a user within the Principal struct.
	Roles    []roles.Role   // Roles represent the roles of a user within the Principal struct.
	TenantID string         // TenantID represents the unique identifier of a tenant within the Principal struct.
	Extra    map[string]any // Extra represents additional metadata or attributes associated with the Principal.
}

// private key type to avoid collisions in context
type principalCtxKey struct{}

var principalKey = principalCtxKey{}

// WithPrincipal stores Principal in context
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

func WithGinPrincipal(c *gin.Context, p *Principal) *gin.Context {
	// gin.Context uses string keys; keep a namespaced string to avoid collisions
	const ginPrincipalKey = "mb.policy.principal"
	c.Set(ginPrincipalKey, p)
	return c
}

// GetPrincipal extracts Principal from context
func GetPrincipal(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey).(*Principal)
	return p, ok
}

// HasRole checks if Principal has a specific role
func HasRole(p *Principal, role roles.Role) bool {
	if p == nil {
		return false
	}
	if roles.HasAny(roles.AdminOrSuperuserRoles(), p.Roles...) {
		return true
	}
	return roles.HasAny([]roles.Role{role}, p.Roles...)
}

type FromPrincipalExtractor[T any] = func(p *Principal) (T, error)

func UserUUIDExtractor(p *Principal) (uuid.UUID, error) {
	return pkg.UUIDIdParser(p.UserID)
}

func IsAdminExtractor(p *Principal) bool {
	return roles.HasAny(p.Roles, roles.AdminOrSuperuserRoles()...)
}
