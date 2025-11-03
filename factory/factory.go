package factory

import (
	"context"

	"github.com/pkg/errors"
)

// Creator creates a new entity from the given input parameters.
//
// Returns:
//   - *T: the newly created entity
//   - error: if entity creation fails
type Creator[T any, CreateParams any] interface {
	Create(ctx context.Context, params CreateParams) (*T, error)
}

// Updater modifies the given entity in-place based on input parameters,
// and returns a list of field names that were updated.
//
// Returns:
//   - []string: list of updated field names
//   - error: if entity update fails
type Updater[T any, UpdateParams any] interface {
	Update(ctx context.Context, obj *T, params UpdateParams) ([]string, error)
}

// Factory defines entity creation and update logic with tracking updated fields.
//
// Type parameters:
//   - T:            the entity type (e.g., User, Order)
//   - CreateParams: type holding input parameters for creating a new entity
//   - UpdateParams: type holding input parameters for updating an existing entity
//
// The Factory is responsible for constructing new entities and applying updates
// while returning a list of modified fields. This allows fine-grained persistence
// and auditing of changes.
type Factory[T any, CreateParams any, UpdateParams any] interface {
	Creator[T, CreateParams]
	Updater[T, UpdateParams]
}

// Func is a functional adapter for Factory.
// Set only required funcs; others return "not implemented".
type Func[T any, CreateParams any, UpdateParams any] struct {
	// CreateFunc constructs a new entity from CreateParams
	CreateFunc func(ctx context.Context, params CreateParams) (*T, error)
	// UpdateFunc mutates entity from UpdateParams and returns changed columns
	UpdateFunc func(ctx context.Context, obj *T, params UpdateParams) ([]string, error)
}

func errNI(name string) error { return errors.New(name + " not implemented") }

// Create implements Factory.
func (f Func[T, CreateParams, UpdateParams]) Create(ctx context.Context, params CreateParams) (*T, error) {
	if f.CreateFunc == nil {
		return nil, errNI("Factory.Create")
	}
	return f.CreateFunc(ctx, params)
}

// Update implements Factory.
func (f Func[T, CreateParams, UpdateParams]) Update(ctx context.Context, obj *T, params UpdateParams) (
	[]string,
	error,
) {
	if f.UpdateFunc == nil {
		return nil, errNI("Factory.Update")
	}
	return f.UpdateFunc(ctx, obj, params)
}

var (
	_ Factory[string, string, string] = (*Func[string, string, string])(nil)
)

// Middleware defines a decorator for Factory implementations.
//
// A Factory middleware can be used to add cross-cutting behavior
// (e.g., logging, validation, metrics) around entity creation and update.
//
// Middlewares are typically composed in a chain:
//
//	var f Factory[User, CreateUserParams, UpdateUserParams]
//	f = MyFactory{}
//	f = LoggingMiddleware(f)
//	f = ValidationMiddleware(f)
//
// In this example, ValidationMiddleware runs before LoggingMiddleware.
type Middleware[T any, CreateParams any, UpdateParams any] func(
	next Factory[T, CreateParams, UpdateParams],
) Factory[T, CreateParams, UpdateParams]
