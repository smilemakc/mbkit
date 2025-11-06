package endpoint

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/domain/authz"
	"github.com/smilemakc/mbkit/domain/policy"
)

// PrincipalExtractor returns a UserIDExtractor that fetches a user ID of type ID from the context using the specified extractor.
func PrincipalExtractor[ID pkg.IDLike](extractor policy.FromPrincipalExtractor[ID]) authz.UserIDExtractor[ID] {
	return func(ctx context.Context) (ID, error) {
		p, ok := policy.GetPrincipal(ctx)
		if !ok || p == nil {
			return *new(ID), policy.ErrPermission
		}
		return extractor(p)
	}
}

// ContextValueExtractor returns a UserIDExtractor that fetches a user ID of type ID from the context using the specified key.
// Returns an error if the key is not found or if the value type does not match the expected ID type.
func ContextValueExtractor[ID pkg.IDLike](key any) authz.UserIDExtractor[ID] {
	return func(ctx context.Context) (ID, error) {
		val := ctx.Value(key)
		if val == nil {
			return *new(ID), policy.ErrPermission
		}
		uid, ok := val.(ID)
		if !ok {
			return *new(ID), fmt.Errorf("unexpected user ID type: %T", val)
		}
		return uid, nil
	}
}

// PrincipalOwnerOrAdminAccessControl creates access control allowing only the owner or admin to access specific resources.
// It uses the provided table name and owner field to determine ownership along with admin role-based permissions.
func PrincipalOwnerOrAdminAccessControl(table string, ownerField string) *authz.OwnerOrAdminAccessControl[uuid.UUID] {
	owner := authz.NewOwnerAccessControl(ownerField, table, PrincipalExtractor(policy.UserUUIDExtractor))

	return authz.NewOwnerOrAdminAccessControl(owner, func(ctx context.Context) (bool, error) {
		p, ok := policy.GetPrincipal(ctx)
		if !ok || p == nil {
			return false, nil
		}
		return policy.IsAdminExtractor(p), nil
	})
}
