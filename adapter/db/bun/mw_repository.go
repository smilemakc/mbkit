package bunrepo

import (
	"context"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/domain/filters"
	"github.com/uptrace/bun"
)

// PolicyScope applies a custom scope to List queries.
//
// Typical use cases:
//   - Multi-tenancy: restrict by tenant_id
//   - Access control: restrict by user visibility
//   - Soft delete: exclude rows with deleted_at IS NOT NULL
type PolicyScope[T any, ID pkg.IDLike] struct {
	// Apply decorates the SELECT query with additional conditions.
	Apply func(ctx context.Context, tx bun.IDB, q *bun.SelectQuery) *bun.SelectQuery
}

// Middleware wraps Repository and overrides only List,
// all other CRUD methods are delegated directly.
func (mw PolicyScope[T, ID]) Middleware(next Repository[T, ID]) Repository[T, ID] {
	return Func[T, ID]{
		SaveFunc:   next.Save,
		GetFunc:    next.Get,
		UpdateFunc: next.Update,
		DeleteFunc: next.Delete,
		ListFunc: func(ctx context.Context, tx bun.IDB, f filters.ListFilter) (filters.ListResponse[T], error) {
			if lf, ok := any(f).(interface {
				Apply(q *bun.SelectQuery) *bun.SelectQuery
			}); ok && mw.Apply != nil {
				f = filters.WrapWithCustomApply(f, func(q *bun.SelectQuery) *bun.SelectQuery {
					return mw.Apply(ctx, tx, lf.Apply(q))
				})
			}
			return next.List(ctx, tx, f)
		},
	}
}

func NewPolicyScope[T any, ID pkg.IDLike](
	apply func(
		ctx context.Context,
		tx bun.IDB,
		q *bun.SelectQuery,
	) *bun.SelectQuery,
) PolicyScope[T, ID] {
	return PolicyScope[T, ID]{Apply: apply}
}
