package auth

import (
	"context"

	pkg "github.com/smilemakc/mbkit"
	"github.com/uptrace/bun"
)

// PredicateAccessControl wraps plain functions into an AccessControl.
type PredicateAccessControl[ID pkg.IDLike] struct {
	canRead   func(ctx context.Context, tx bun.IDB, id ID) error
	canList   func(ctx context.Context, tx bun.IDB) error
	canCreate func(ctx context.Context, tx bun.IDB) error
	canUpdate func(ctx context.Context, tx bun.IDB, id ID) error
	canDelete func(ctx context.Context, tx bun.IDB, id ID) error
}

func (p *PredicateAccessControl[ID]) CanRead(ctx context.Context, tx bun.IDB, id ID) error {
	return p.canRead(ctx, tx, id)
}
func (p *PredicateAccessControl[ID]) CanList(ctx context.Context, tx bun.IDB) error {
	return p.canList(ctx, tx)
}
func (p *PredicateAccessControl[ID]) CanCreate(ctx context.Context, tx bun.IDB) error {
	return p.canCreate(ctx, tx)
}
func (p *PredicateAccessControl[ID]) CanUpdate(ctx context.Context, tx bun.IDB, id ID) error {
	return p.canUpdate(ctx, tx, id)
}
func (p *PredicateAccessControl[ID]) CanDelete(ctx context.Context, tx bun.IDB, id ID) error {
	return p.canDelete(ctx, tx, id)
}

// NewPredicateACL builds a new ACL from function handlers.
func NewPredicateACL[ID pkg.IDLike](
	canRead func(ctx context.Context, tx bun.IDB, id ID) error,
	canList func(ctx context.Context, tx bun.IDB) error,
	canCreate func(ctx context.Context, tx bun.IDB) error,
	canUpdate func(ctx context.Context, tx bun.IDB, id ID) error,
	canDelete func(ctx context.Context, tx bun.IDB, id ID) error,
) *PredicateAccessControl[ID] {
	return &PredicateAccessControl[ID]{canRead, canList, canCreate, canUpdate, canDelete}
}
