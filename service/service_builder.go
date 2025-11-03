package service

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/factory"
	"github.com/smilemakc/mbkit/filters"
	"github.com/smilemakc/mbkit/repository"
	"github.com/uptrace/bun"
)

// Middleware defines a decorator for Service.
// NOTE: middlewares are applied in the order they were added.
type Middleware[
	T any,
	CreateParams any,
	UpdateParams any,
	ID pkg.IDLike,
] func(next Service[T, CreateParams, UpdateParams, ID]) Service[T, CreateParams, UpdateParams, ID]

// HookFunc Hook for create
type HookFunc[T any] func(ctx context.Context, tx bun.IDB, obj *T) error

// DeleteHookFunc defines a function type that performs operations during deletion using a context and an ID value.
type DeleteHookFunc[ID pkg.IDLike] func(ctx context.Context, tx bun.IDB, id ID) error

// UpdateHookFunc Hook for update (object plus list of columns).
// Best-effort to allow mutation of the column set before the writing.
type UpdateHookFunc[T any] func(ctx context.Context, tx bun.IDB, obj *T, cols []string) ([]string, error)

// AfterUpdateHookFunc is executed after repo.Update; it MUST NOT attempt to change persisted columns.
type AfterUpdateHookFunc[T any] func(ctx context.Context, tx bun.IDB, obj *T, cols []string) error

// Builder assembles a Repository, a Factory, and a Service instance with middleware and hooks.
type Builder[T any, CreateParams any, UpdateParams any, ID pkg.IDLike] struct {
	repo       repository.Repository[T, ID]
	factory    factory.Factory[T, CreateParams, UpdateParams]
	repoMWs    []repository.Middleware[T, ID]
	factoryMWs []factory.Middleware[T, CreateParams, UpdateParams]
	serviceMWs []Middleware[T, CreateParams, UpdateParams, ID]

	beforeCreate []HookFunc[T]
	afterCreate  []HookFunc[T]

	beforeUpdate []UpdateHookFunc[T]
	afterUpdate  []AfterUpdateHookFunc[T]

	beforeDelete []DeleteHookFunc[ID]
	afterDelete  []DeleteHookFunc[ID]
}

// NewServiceBuilder initializes and returns a new Builder instance.
func NewServiceBuilder[T any, CreateParams any, UpdateParams any, ID pkg.IDLike]() *Builder[T, CreateParams, UpdateParams, ID] {
	return &Builder[T, CreateParams, UpdateParams, ID]{}
}

// WithRepository sets the Repository implementation.
func (b *Builder[T, CreateParams, UpdateParams, ID]) WithRepository(
	repo repository.Repository[T, ID],
) *Builder[T, CreateParams, UpdateParams, ID] {
	b.repo = repo
	return b
}

// WithFactory sets the Factory implementation.
func (b *Builder[T, CreateParams, UpdateParams, ID]) WithFactory(
	f factory.Factory[T, CreateParams, UpdateParams],
) *Builder[T, CreateParams, UpdateParams, ID] {
	b.factory = f
	return b
}

// WithRepoMiddleware adds a Repository middleware to the chain (applied in order).
func (b *Builder[T, CreateParams, UpdateParams, ID]) WithRepoMiddleware(
	mw repository.Middleware[T, ID],
) *Builder[T, CreateParams, UpdateParams, ID] {
	b.repoMWs = append(b.repoMWs, mw)
	return b
}

// WithFactoryMiddleware adds a Factory middleware to the chain (applied in order).
func (b *Builder[T, CreateParams, UpdateParams, ID]) WithFactoryMiddleware(
	mw factory.Middleware[T, CreateParams, UpdateParams],
) *Builder[T, CreateParams, UpdateParams, ID] {
	b.factoryMWs = append(b.factoryMWs, mw)
	return b
}

// WithServiceMiddleware adds a Service middleware to the chain (applied in order).
func (b *Builder[T, CreateParams, UpdateParams, ID]) WithServiceMiddleware(
	mw Middleware[T, CreateParams, UpdateParams, ID],
) *Builder[T, CreateParams, UpdateParams, ID] {
	b.serviceMWs = append(b.serviceMWs, mw)
	return b
}

// Hook registration
func (b *Builder[T, CreateParams, UpdateParams, ID]) BeforeCreate(h HookFunc[T]) *Builder[T, CreateParams, UpdateParams, ID] {
	b.beforeCreate = append(b.beforeCreate, h)
	return b
}
func (b *Builder[T, CreateParams, UpdateParams, ID]) AfterCreate(h HookFunc[T]) *Builder[T, CreateParams, UpdateParams, ID] {
	b.afterCreate = append(b.afterCreate, h)
	return b
}
func (b *Builder[T, CreateParams, UpdateParams, ID]) BeforeUpdate(h UpdateHookFunc[T]) *Builder[T, CreateParams, UpdateParams, ID] {
	b.beforeUpdate = append(b.beforeUpdate, h)
	return b
}
func (b *Builder[T, CreateParams, UpdateParams, ID]) AfterUpdate(h AfterUpdateHookFunc[T]) *Builder[T, CreateParams, UpdateParams, ID] {
	b.afterUpdate = append(b.afterUpdate, h)
	return b
}
func (b *Builder[T, CreateParams, UpdateParams, ID]) BeforeDelete(h DeleteHookFunc[ID]) *Builder[T, CreateParams, UpdateParams, ID] {
	b.beforeDelete = append(b.beforeDelete, h)
	return b
}
func (b *Builder[T, CreateParams, UpdateParams, ID]) AfterDelete(h DeleteHookFunc[ID]) *Builder[T, CreateParams, UpdateParams, ID] {
	b.afterDelete = append(b.afterDelete, h)
	return b
}

// Build composes Repository, Factory, and Service with all middlewares and hooks.
func (b *Builder[T, CreateParams, UpdateParams, ID]) Build() (
	Service[T, CreateParams, UpdateParams, ID], error,
) {
	if b.repo == nil {
		return nil, errors.New("build service: repository is required")
	}
	if b.factory == nil {
		return nil, errors.New("build service: factory is required")
	}

	// Apply repository middlewares (execute in the order they were added)
	// Wrap in reverse so the first added is the outermost and runs first.
	repo := b.repo
	for i := len(b.repoMWs) - 1; i >= 0; i-- {
		mw := b.repoMWs[i]
		if mw == nil {
			return nil, fmt.Errorf("build service: repo middleware at index %d is nil", i)
		}
		repo = mw(repo)
		if repo == nil {
			return nil, fmt.Errorf("build service: repo middleware at index %d returned nil", i)
		}
	}

	// Apply factory middlewares (same wrapping logic as repo)
	f := b.factory
	for i := len(b.factoryMWs) - 1; i >= 0; i-- {
		mw := b.factoryMWs[i]
		if mw == nil {
			return nil, fmt.Errorf("build service: factory middleware at index %d is nil", i)
		}
		f = mw(f)
		if f == nil {
			return nil, fmt.Errorf("build service: factory middleware at index %d returned nil", i)
		}
	}

	var svc Service[T, CreateParams, UpdateParams, ID] = &hookedService[T, CreateParams, UpdateParams, ID]{
		repo:         repo,
		factory:      f,
		beforeCreate: append([]HookFunc[T](nil), b.beforeCreate...),
		afterCreate:  append([]HookFunc[T](nil), b.afterCreate...),
		beforeUpdate: append([]UpdateHookFunc[T](nil), b.beforeUpdate...),
		afterUpdate:  append([]AfterUpdateHookFunc[T](nil), b.afterUpdate...),
		beforeDelete: append([]DeleteHookFunc[ID](nil), b.beforeDelete...),
		afterDelete:  append([]DeleteHookFunc[ID](nil), b.afterDelete...),
	}

	// Apply service middlewares (execute in the order they were added)
	for i := len(b.serviceMWs) - 1; i >= 0; i-- {
		mw := b.serviceMWs[i]
		if mw == nil {
			return nil, fmt.Errorf("build service: service middleware at index %d is nil", i)
		}
		svc = mw(svc)
		if svc == nil {
			return nil, fmt.Errorf("build service: service middleware at index %d returned nil", i)
		}
	}

	return svc, nil
}

// hookedService wraps service and executes hooks in a predictable order.
// NOTE: Transaction control is delegated to the caller via bun.IDB.
type hookedService[T any, CreateParams any, UpdateParams any, ID pkg.IDLike] struct {
	repo    repository.Repository[T, ID]
	factory factory.Factory[T, CreateParams, UpdateParams]

	beforeCreate []HookFunc[T]
	afterCreate  []HookFunc[T]

	beforeUpdate []UpdateHookFunc[T]
	afterUpdate  []AfterUpdateHookFunc[T]

	beforeDelete []DeleteHookFunc[ID]
	afterDelete  []DeleteHookFunc[ID]
}

func (s *hookedService[T, CreateParams, UpdateParams, ID]) Create(
	ctx context.Context,
	tx bun.IDB,
	params CreateParams,
) (*T, error) {
	obj, err := s.factory.Create(ctx, params)
	if err != nil {
		return nil, errors.Wrap(err, "service.create: factory.Create failed")
	}
	for i, h := range s.beforeCreate {
		if err := h(ctx, tx, obj); err != nil {
			return nil, errors.Wrapf(err, "service.create: beforeCreate[%d] failed", i)
		}
	}
	if err := s.repo.Save(ctx, tx, obj); err != nil {
		return nil, errors.Wrap(err, "service.create: repo.Save failed")
	}
	for i, h := range s.afterCreate {
		if err := h(ctx, tx, obj); err != nil {
			return nil, errors.Wrapf(err, "service.create: afterCreate[%d] failed", i)
		}
	}
	return obj, nil
}

func (s *hookedService[T, CreateParams, UpdateParams, ID]) Get(
	ctx context.Context,
	tx bun.IDB,
	id ID,
	args GetterArgs,
) (*T, error) {
	return s.repo.Get(ctx, tx, id, args)
}

func (s *hookedService[T, CreateParams, UpdateParams, ID]) List(
	ctx context.Context,
	tx bun.IDB,
	f filters.ListFilter,
) (filters.ListResponse[T], error) {
	return s.repo.List(ctx, tx, f)
}

func (s *hookedService[T, CreateParams, UpdateParams, ID]) Update(
	ctx context.Context,
	tx bun.IDB,
	id ID,
	args GetterArgs,
	params UpdateParams,
) (*T, error) {
	item, err := s.repo.Get(ctx, tx, id, args)
	var v T
	if err != nil {
		return nil, fmt.Errorf("service.update: repo.Get (%T) failed: %w", v, err)
	}

	cols, err := s.factory.Update(ctx, item, params)
	if err != nil {
		// errors.Wrap(err, "service.update: factory.Update failed")
		return nil, fmt.Errorf("service.update: factory.Update (%T) failed: %w", v, err)
	}

	// Defensive copy to avoid hook side-effects on underlying slice.
	colsCopy := append([]string(nil), cols...)

	for i, h := range s.beforeUpdate {
		colsCopy, err = h(ctx, tx, item, append([]string(nil), colsCopy...))
		if err != nil {
			// return nil, errors.Wrapf(err, "service.update: beforeUpdate[%d] failed", i)
			return nil, fmt.Errorf("service.update: beforeUpdate(%T)[%d] failed: %w", v, i, err)
		}
	}

	if err := s.repo.Update(ctx, tx, id, item, colsCopy...); err != nil {
		return nil, fmt.Errorf("service.update: repo.Update(%T) failed: %w", v, err)
	}

	for i, h := range s.afterUpdate {
		// afterUpdate is informational/side-effect only, must not alter persisted cols
		if err := h(ctx, tx, item, append([]string(nil), colsCopy...)); err != nil {
			return nil, fmt.Errorf("service.update: afterUpdate(%T)[%d] failed: %w", v, i, err)
		}
	}

	return item, nil
}

func (s *hookedService[T, CreateParams, UpdateParams, ID]) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	for i, h := range s.beforeDelete {
		if err := h(ctx, tx, id); err != nil {
			return errors.Wrapf(err, "service.delete: beforeDelete[%d] failed", i)
		}
	}
	if err := s.repo.Delete(ctx, tx, id); err != nil {
		return errors.Wrap(err, "service.delete: repo.Delete failed")
	}
	for i, h := range s.afterDelete {
		if err := h(ctx, tx, id); err != nil {
			return errors.Wrapf(err, "service.delete: afterDelete[%d] failed", i)
		}
	}
	return nil
}

var (
	_ Service[string, string, string, string] = (*hookedService[string, string, string, string])(nil)
)
