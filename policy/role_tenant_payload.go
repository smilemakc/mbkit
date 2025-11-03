package policy

import (
	"context"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/internal/l"
	"github.com/smilemakc/mbkit/roles"
	"github.com/uptrace/bun"
)

// RolePolicy defines a policy for associating roles with specific actions or scopes in a system.
type RolePolicy[R any, ID pkg.IDLike] struct{ Has roles.Role }

func (r RolePolicy[R, ID]) Check(ctx context.Context, _ bun.IDB, _ Action, _ ObjID[ID]) error {
	p, ok := GetPrincipal(ctx)
	if !ok || p == nil {
		return ErrUnauthorized
	}
	if HasRole(p, r.Has) {
		return nil
	}
	return ErrForbidden
}

func (r RolePolicy[R, ID]) Scope(ctx context.Context, _ bun.IDB, q *bun.SelectQuery) *bun.SelectQuery {
	return q
}
func (r RolePolicy[R, ID]) Validate(context.Context, bun.IDB, Action, any) error { return nil }

func NewRolePolicy[R any, ID pkg.IDLike](has roles.Role) RolePolicy[R, ID] {
	return RolePolicy[R, ID]{Has: has}
}

// TenantPolicy — LIST queries by tenant_id in multi-tenant environments.
type TenantPolicy[R any, ID pkg.IDLike] struct{ Column string }

func (t TenantPolicy[R, ID]) Check(context.Context, bun.IDB, Action, ObjID[ID]) error { return nil }
func (t TenantPolicy[R, ID]) Validate(context.Context, bun.IDB, Action, any) error    { return nil }
func (t TenantPolicy[R, ID]) Scope(ctx context.Context, _ bun.IDB, q *bun.SelectQuery) *bun.SelectQuery {
	p, ok := GetPrincipal(ctx)
	if !ok || p == nil || p.TenantID == "" {
		return q.Where("1=0")
	}
	return q.Where(t.Column+" = ?", p.TenantID)
}

func NewTenantPolicy[R any, ID pkg.IDLike](column string) TenantPolicy[R, ID] {
	return TenantPolicy[R, ID]{Column: column}
}

// FieldOwnershipRule — Validates payload ownership, ensuring referenced resources (e.g., FileID) belong to the user.
type FieldOwnershipRule[Owner pkg.IDLike] struct {
	Table       string
	OwnerExpr   string
	ParseUserID pkg.IDParser[Owner]
	ExtractID   func(ctx context.Context, payload any) (any, bool)
}

type PayloadOwnershipPolicy[R any, ID pkg.IDLike, Owner pkg.IDLike] struct {
	Rules []FieldOwnershipRule[Owner]
}

func (p PayloadOwnershipPolicy[R, ID, Owner]) Check(context.Context, bun.IDB, Action, ObjID[ID]) error {
	return nil
}
func (p PayloadOwnershipPolicy[R, ID, Owner]) Scope(
	ctx context.Context,
	tx bun.IDB,
	q *bun.SelectQuery,
) *bun.SelectQuery {
	return q
}
func (p PayloadOwnershipPolicy[R, ID, Owner]) Validate(ctx context.Context, tx bun.IDB, _ Action, payload any) error {
	sub, ok := GetPrincipal(ctx)
	if !ok || sub == nil {
		return ErrUnauthorized
	}
	for _, r := range p.Rules {
		refID, exists := r.ExtractID(ctx, payload)
		if !exists {
			continue
		}
		owner, err := r.ParseUserID(sub.UserID)
		if err != nil {
			return err
		}
		exists, err = tx.NewSelect().
			TableExpr(r.Table+" AS f").
			Where("f.id = ?", refID).
			Where(r.OwnerExpr+" = ?", owner).
			Limit(1).
			Exists(ctx)
		if err != nil {
			return err
		}
		if !exists {
			l.Log.Debug().Str("table", r.Table).Interface("ref", refID).Msg("owner not found in table")
			return ErrNotFound
		}
	}
	return nil
}
