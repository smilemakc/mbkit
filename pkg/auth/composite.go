package auth

import (
	"context"
	"fmt"

	pkg "github.com/smilemakc/mbkit"
	"github.com/uptrace/bun"
)

// CompositeAccessControl combines multiple ACL strategies.
// Access is granted only if ALL strategies allow the action.
type CompositeAccessControl[ID pkg.IDLike] struct {
	strategies []AccessControl[ID]
}

// NewCompositeAccessControl creates a new composite ACL.
func NewCompositeAccessControl[ID pkg.IDLike](strategies ...AccessControl[ID]) *CompositeAccessControl[ID] {
	return &CompositeAccessControl[ID]{strategies: strategies}
}

func (c *CompositeAccessControl[ID]) runCheck(fn func(AccessControl[ID]) error) error {
	for _, s := range c.strategies {
		if err := fn(s); err != nil {
			// fail fast: first error stops everything
			return fmt.Errorf("permission denied: %w", err)
		}
	}
	return nil
}

func (c *CompositeAccessControl[ID]) CanRead(ctx context.Context, tx bun.IDB, id ID) error {
	return c.runCheck(func(s AccessControl[ID]) error { return s.CanRead(ctx, tx, id) })
}
func (c *CompositeAccessControl[ID]) CanList(ctx context.Context, tx bun.IDB) error {
	return c.runCheck(func(s AccessControl[ID]) error { return s.CanList(ctx, tx) })
}
func (c *CompositeAccessControl[ID]) CanCreate(ctx context.Context, tx bun.IDB) error {
	return c.runCheck(func(s AccessControl[ID]) error { return s.CanCreate(ctx, tx) })
}
func (c *CompositeAccessControl[ID]) CanUpdate(ctx context.Context, tx bun.IDB, id ID) error {
	return c.runCheck(func(s AccessControl[ID]) error { return s.CanUpdate(ctx, tx, id) })
}
func (c *CompositeAccessControl[ID]) CanDelete(ctx context.Context, tx bun.IDB, id ID) error {
	return c.runCheck(func(s AccessControl[ID]) error { return s.CanDelete(ctx, tx, id) })
}
