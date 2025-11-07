package port

import (
	"context"
	"errors"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/domain/filters"

	"github.com/uptrace/bun"
)

// GetterArgs is a map of arguments passed to the Get method.
// Keys are SQL conditions with placeholders, and values are their parameters.
type GetterArgs = map[string]any

// Saver defines the ability to persist an entity into storage.
type Saver[T any] interface {
	// Save inserts the entity into the database.
	Save(ctx context.Context, tx bun.IDB, item *T) error
}

// Getter defines the ability to fetch an entity by its identifier.
type Getter[T any, ID pkg.IDLike] interface {
	// Get fetches an entity by its primary key.
	Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error)
}

// Lister defines the ability to list entities using a filter.
type Lister[T any] interface {
	// List returns a paginated list of entities according to the provided filter.
	List(ctx context.Context, tx bun.IDB, filter filters.ListFilter) (filters.ListResponse[T], error)
}

// Updater defines the ability to update an existing entity.
type Updater[T any, ID pkg.IDLike] interface {
	// Update updates entity fields, restricting updated columns if provided.
	Update(ctx context.Context, tx bun.IDB, id ID, item *T, columns ...string) error
}

// Deleter defines the ability to remove an entity by its identifier.
type Deleter[ID pkg.IDLike] interface {
	// Delete removes the entity with the specified ID.
	Delete(ctx context.Context, tx bun.IDB, id ID) error
}

// BulkDeleter is a generic interface for deleting multiple records based on specified criteria.
type BulkDeleter[T any] interface {
	BulkDelete(ctx context.Context, tx bun.IDB, args GetterArgs) error
}

// Repository combines all CRUD operations into a single interface.
type Repository[T any, ID pkg.IDLike] interface {
	Saver[T]
	Getter[T, ID]
	Lister[T]
	Updater[T, ID]
	Deleter[ID]
}

// Middleware defines a decorator for Repository implementations.
type Middleware[T any, ID pkg.IDLike] func(next Repository[T, ID]) Repository[T, ID]

// Func is a functional adapter for Repository.
// Set only the funcs you need; others will return a "not implemented" error.
type Func[T any, ID pkg.IDLike] struct {
	// SaveFunc persists obj
	SaveFunc func(ctx context.Context, tx bun.IDB, obj *T) error
	// UpdateFunc persists selected columns of obj for id
	UpdateFunc func(ctx context.Context, tx bun.IDB, id ID, obj *T, cols ...string) error
	// DeleteFunc removes entity by id
	DeleteFunc func(ctx context.Context, tx bun.IDB, id ID) error
	// GetFunc fetches entity by id with optional args
	GetFunc func(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error)
	// ListFunc lists entities by filter
	ListFunc func(ctx context.Context, tx bun.IDB, f filters.ListFilter) (filters.ListResponse[T], error)
	// BulkDeleteFunc deletes multiple records based on specified criteria.
	BulkDeleteFunc func(ctx context.Context, tx bun.IDB, args GetterArgs) error
}

func errNI(name string) error { return errors.New(name + " not implemented") }

// Save implements Repository.
func (r Func[T, ID]) Save(ctx context.Context, tx bun.IDB, obj *T) error {
	if r.SaveFunc == nil {
		return errNI("Repository.Save")
	}
	return r.SaveFunc(ctx, tx, obj)
}

// Update implements Repository.
func (r Func[T, ID]) Update(ctx context.Context, tx bun.IDB, id ID, obj *T, cols ...string) error {
	if r.UpdateFunc == nil {
		return errNI("Repository.Update")
	}
	return r.UpdateFunc(ctx, tx, id, obj, cols...)
}

// Delete implements Repository.
func (r Func[T, ID]) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	if r.DeleteFunc == nil {
		return errNI("Repository.Delete")
	}
	return r.DeleteFunc(ctx, tx, id)
}

// Get implements Repository.
func (r Func[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	if r.GetFunc == nil {
		return nil, errNI("Repository.Get")
	}
	return r.GetFunc(ctx, tx, id, args)
}

// List implements Repository.
func (r Func[T, ID]) List(ctx context.Context, tx bun.IDB, f filters.ListFilter) (filters.ListResponse[T], error) {
	if r.ListFunc == nil {
		return filters.ListResponse[T]{}, errNI("Repository.List")
	}
	return r.ListFunc(ctx, tx, f)
}

// BulkDelete implements Repository.
func (r Func[T, ID]) BulkDelete(ctx context.Context, tx bun.IDB, args GetterArgs) error {
	if r.BulkDeleteFunc == nil {
		return errNI("Repository.BulkDelete")
	}
	return r.BulkDeleteFunc(ctx, tx, args)
}

var (
	_ Repository[any, string] = (*Func[any, string])(nil)
)

