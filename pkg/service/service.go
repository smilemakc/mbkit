package service

import (
	"context"
	"errors"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/pkg/factory"
	"github.com/smilemakc/mbkit/pkg/filters"
	"github.com/smilemakc/mbkit/pkg/repository"

	"github.com/uptrace/bun"
)

type GetterArgs = map[string]any

type Creator[T any, CreateParams any] interface {
	Create(ctx context.Context, tx bun.IDB, params CreateParams) (*T, error)
}

type Updater[T any, UpdateParams any, ID pkg.IDLike] interface {
	Update(ctx context.Context, tx bun.IDB, id ID, args GetterArgs, params UpdateParams) (*T, error)
}

// Service defines business logic service with Service operations.
type Service[T any, CreateParams any, UpdateParams any, ID pkg.IDLike] interface {
	Creator[T, CreateParams]
	Updater[T, UpdateParams, ID]
	repository.Getter[T, ID]
	repository.Lister[T]
	repository.Deleter[ID]
	// Create(ctx context.Context, tx bun.IDB, params CreateParams) (*T, error)
	// Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error)
	// List(ctx context.Context, tx bun.IDB, filter filters.ListFilter) (filters.ListResponse[T], error)
	// Update(ctx context.Context, tx bun.IDB, id ID, args GetterArgs, params UpdateParams) (*T, error)
	// Delete(ctx context.Context, tx bun.IDB, id ID) error
}

// BaseService is a default implementation of Service.
type BaseService[T any, CreateParams any, UpdateParams any, ID pkg.IDLike] struct {
	repo    repository.Repository[T, ID]
	factory factory.Factory[T, CreateParams, UpdateParams]
}

// NewService creates a new service with a given repository and factory.
func NewService[T any, CreateParams any, UpdateParams any, ID pkg.IDLike](
	f factory.Factory[T, CreateParams, UpdateParams],
	r repository.Repository[T, ID],
) *BaseService[T, CreateParams, UpdateParams, ID] {
	return &BaseService[T, CreateParams, UpdateParams, ID]{repo: r, factory: f}
}

func (s *BaseService[T, CreateParams, UpdateParams, ID]) Create(
	ctx context.Context,
	tx bun.IDB,
	params CreateParams,
) (*T, error) {
	obj, err := s.factory.Create(ctx, params)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, tx, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (s *BaseService[T, CreateParams, UpdateParams, ID]) Get(
	ctx context.Context,
	tx bun.IDB,
	id ID,
	args GetterArgs,
) (*T, error) {
	return s.repo.Get(ctx, tx, id, args)
}

func (s *BaseService[T, CreateParams, UpdateParams, ID]) List(
	ctx context.Context,
	tx bun.IDB,
	f filters.ListFilter,
) (filters.ListResponse[T], error) {
	return s.repo.List(ctx, tx, f)
}

func (s *BaseService[T, CreateParams, UpdateParams, ID]) Update(
	ctx context.Context,
	tx bun.IDB,
	id ID,
	args GetterArgs,
	params UpdateParams,
) (*T, error) {
	item, err := s.repo.Get(ctx, tx, id, args)
	if err != nil {
		return nil, err
	}
	cols, err := s.factory.Update(ctx, item, params)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, tx, id, item, cols...); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *BaseService[T, CreateParams, UpdateParams, ID]) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	return s.repo.Delete(ctx, tx, id)
}

// Func is a functional adapter for Service.
// Provide only the funcs you need; others return "not implemented".
type Func[T any, CreateParams any, UpdateParams any, ID pkg.IDLike] struct {
	CreateFunc func(ctx context.Context, tx bun.IDB, params CreateParams) (*T, error)
	GetFunc    func(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error)
	ListFunc   func(ctx context.Context, tx bun.IDB, f filters.ListFilter) (filters.ListResponse[T], error)
	UpdateFunc func(ctx context.Context, tx bun.IDB, id ID, args GetterArgs, params UpdateParams) (*T, error)
	DeleteFunc func(ctx context.Context, tx bun.IDB, id ID) error
}

func errNI(name string) error { return errors.New(name + " not implemented") }

// Create implements Service.
func (s Func[T, CreateParams, UpdateParams, ID]) Create(ctx context.Context, tx bun.IDB, params CreateParams) (
	*T,
	error,
) {
	if s.CreateFunc == nil {
		return nil, errNI("Service.Create")
	}
	return s.CreateFunc(ctx, tx, params)
}

// Get implements Service.
func (s Func[T, CreateParams, UpdateParams, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (
	*T,
	error,
) {
	if s.GetFunc == nil {
		return nil, errNI("Service.Get")
	}
	return s.GetFunc(ctx, tx, id, args)
}

// List implements Service.
func (s Func[T, CreateParams, UpdateParams, ID]) List(
	ctx context.Context,
	tx bun.IDB,
	f filters.ListFilter,
) (filters.ListResponse[T], error) {
	if s.ListFunc == nil {
		return filters.ListResponse[T]{}, errNI("Service.List")
	}
	return s.ListFunc(ctx, tx, f)
}

// Update implements Service.
func (s Func[T, CreateParams, UpdateParams, ID]) Update(
	ctx context.Context,
	tx bun.IDB,
	id ID,
	args GetterArgs,
	params UpdateParams,
) (*T, error) {
	if s.UpdateFunc == nil {
		return nil, errNI("Service.Update")
	}
	return s.UpdateFunc(ctx, tx, id, args, params)
}

// Delete implements Service.
func (s Func[T, CreateParams, UpdateParams, ID]) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	if s.DeleteFunc == nil {
		return errNI("Service.Delete")
	}
	return s.DeleteFunc(ctx, tx, id)
}

var (
	_ Service[string, string, string, string] = (*Func[string, string, string, string])(nil)
	_ Service[string, string, string, string] = (*BaseService[string, string, string, string])(nil)
)
