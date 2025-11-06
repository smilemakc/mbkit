package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/pkg/policy"
	"github.com/smilemakc/mbkit/pkg/roles"

	"github.com/uptrace/bun"
)

func UserIDFromContextExtractor(ctx context.Context) (uuid.UUID, error) {
	value := ctx.Value("user_id")
	if v, ok := value.(uuid.UUID); ok {
		return v, nil
	}
	if v, ok := value.(string); ok {
		return uuid.Parse(v)
	}
	return uuid.Nil, fmt.Errorf("user_id is not found in context")
}

func UserRolesFromContextExtractor(ctx context.Context) ([]roles.Role, error) {
	value := ctx.Value("user_roles")
	if v, ok := value.([]roles.Role); ok {
		return v, nil
	}
	return nil, fmt.Errorf("user_roles is not found in context")
}

func IsAdmin(ctx context.Context) (bool, error) {
	userRoles, err := UserRolesFromContextExtractor(ctx)
	if err != nil {
		return false, fmt.Errorf("can't extract user roles: %w", err)
	}
	roles.HasAny(userRoles, roles.AdminRole)
	return roles.HasAny(roles.AdminOrSuperuserRoles(), userRoles...), nil
}

// OwnerAccessControl implements owner-based access control for resources in a database table.
// It associates users to resources through a specific userIdField and allows access only for the owner.
// The extractor function retrieves the user ID from the context for validation.
type OwnerAccessControl[ID pkg.IDLike] struct {
	userIdField string
	table       string
	extractor   UserIDExtractor[ID]
}

func NewOwnerAccessControl[ID pkg.IDLike](userIdField, table string, extractor UserIDExtractor[ID]) *OwnerAccessControl[ID] {
	return &OwnerAccessControl[ID]{userIdField: userIdField, table: table, extractor: extractor}
}

func (o *OwnerAccessControl[ID]) checkOwner(ctx context.Context, tx bun.IDB, id ID) error {
	uid, err := o.extractor(ctx)
	if err != nil {
		return err
	}
	exists, err := tx.NewSelect().
		Table(o.table).
		Where("id = ?", id).
		Where(o.userIdField+" = ?", uid).
		Limit(1).
		Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return policy.ErrForbidden
	}
	return nil
}

func (o *OwnerAccessControl[ID]) CanRead(ctx context.Context, tx bun.IDB, id ID) error {
	return o.checkOwner(ctx, tx, id)
}
func (o *OwnerAccessControl[ID]) CanList(ctx context.Context, tx bun.IDB) error {
	_, err := o.extractor(ctx)
	return err
}
func (o *OwnerAccessControl[ID]) CanCreate(ctx context.Context, tx bun.IDB) error {
	_, err := o.extractor(ctx)
	return err
}
func (o *OwnerAccessControl[ID]) CanUpdate(ctx context.Context, tx bun.IDB, id ID) error {
	return o.checkOwner(ctx, tx, id)
}
func (o *OwnerAccessControl[ID]) CanDelete(ctx context.Context, tx bun.IDB, id ID) error {
	return o.checkOwner(ctx, tx, id)
}

// OwnerOrAdminAccessControl combines owner-based and admin-based access control mechanisms.
// It allows access if the user is the owner or has admin privileges.
type OwnerOrAdminAccessControl[ID pkg.IDLike] struct {
	owner   *OwnerAccessControl[ID]
	isAdmin func(ctx context.Context) (bool, error)
}

func NewOwnerOrAdminAccessControl[ID pkg.IDLike](
	owner *OwnerAccessControl[ID],
	isAdmin func(ctx context.Context) (bool, error),
) *OwnerOrAdminAccessControl[ID] {
	return &OwnerOrAdminAccessControl[ID]{owner: owner, isAdmin: isAdmin}
}

func (a *OwnerOrAdminAccessControl[ID]) CanRead(ctx context.Context, tx bun.IDB, id ID) error {
	if ok, err := a.isAdmin(ctx); err != nil {
		return err
	} else if ok {
		return nil
	}
	return a.owner.CanRead(ctx, tx, id)
}
func (a *OwnerOrAdminAccessControl[ID]) CanList(ctx context.Context, tx bun.IDB) error {
	if ok, err := a.isAdmin(ctx); err != nil {
		return err
	} else if ok {
		return nil
	}
	return a.owner.CanList(ctx, tx)
}
func (a *OwnerOrAdminAccessControl[ID]) CanCreate(ctx context.Context, tx bun.IDB) error {
	if ok, err := a.isAdmin(ctx); err != nil {
		return err
	} else if ok {
		return nil
	}
	return a.owner.CanCreate(ctx, tx)
}
func (a *OwnerOrAdminAccessControl[ID]) CanUpdate(ctx context.Context, tx bun.IDB, id ID) error {
	if ok, err := a.isAdmin(ctx); err != nil {
		return err
	} else if ok {
		return nil
	}
	return a.owner.CanUpdate(ctx, tx, id)
}
func (a *OwnerOrAdminAccessControl[ID]) CanDelete(ctx context.Context, tx bun.IDB, id ID) error {
	if ok, err := a.isAdmin(ctx); err != nil {
		return err
	} else if ok {
		return nil
	}
	return a.owner.CanDelete(ctx, tx, id)
}

var (
	_ AccessControl[uuid.UUID] = &OwnerAccessControl[uuid.UUID]{}
	_ AccessControl[uuid.UUID] = &OwnerOrAdminAccessControl[uuid.UUID]{}
)
