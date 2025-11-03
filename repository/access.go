package repository

import (
	"context"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/auth"
	"github.com/smilemakc/mbkit/filters"
	"github.com/uptrace/bun"
)

// AccessRepository wraps a Repository with authorization checks.
// Each method performs an access control check before delegating
// to the underlying Repository.
//
// This is useful for enforcing permissions at the data access layer,
// independent of higher-level business logic.
type AccessRepository[T any, ID pkg.IDLike] struct {
	repo       Repository[T, ID]
	authorizer auth.AccessControl[ID]
}

// NewAccessRepository creates a new Repository decorator with access control.
func NewAccessRepository[T any, ID pkg.IDLike](
	repo Repository[T, ID],
	authorizer auth.AccessControl[ID],
) *AccessRepository[T, ID] {
	return &AccessRepository[T, ID]{repo: repo, authorizer: authorizer}
}

// Save checks create permissions before saving the entity.
func (a *AccessRepository[T, ID]) Save(ctx context.Context, tx bun.IDB, item *T) error {
	if err := a.authorizer.CanCreate(ctx, tx); err != nil {
		return err
	}
	return a.repo.Save(ctx, tx, item)
}

// Get checks read permissions before fetching the entity by ID.
func (a *AccessRepository[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	if err := a.authorizer.CanRead(ctx, tx, id); err != nil {
		return nil, err
	}
	return a.repo.Get(ctx, tx, id, args)
}

// List checks list permissions before returning entities.
func (a *AccessRepository[T, ID]) List(ctx context.Context, tx bun.IDB, f filters.ListFilter) (filters.ListResponse[T], error) {
	if err := a.authorizer.CanList(ctx, tx); err != nil {
		return filters.ListResponse[T]{}, err
	}
	return a.repo.List(ctx, tx, f)
}

// Update checks update permissions before modifying the entity.
func (a *AccessRepository[T, ID]) Update(ctx context.Context, tx bun.IDB, id ID, item *T, columns ...string) error {
	if err := a.authorizer.CanUpdate(ctx, tx, id); err != nil {
		return err
	}
	return a.repo.Update(ctx, tx, id, item, columns...)
}

// Delete checks delete permissions before removing the entity.
func (a *AccessRepository[T, ID]) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	if err := a.authorizer.CanDelete(ctx, tx, id); err != nil {
		return err
	}
	return a.repo.Delete(ctx, tx, id)
}
