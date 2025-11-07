package endpoint

import (
	"context"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/application/service"
	"github.com/smilemakc/mbkit/domain/authz"
	"github.com/smilemakc/mbkit/domain/filters"
	"github.com/smilemakc/mbkit/domain/policy"
	"github.com/uptrace/bun"
)

// CRUDBuilder builds CRUD endpoints with ACL, Policy, nesting, filtering and customization.
type CRUDBuilder[T any, ID pkg.IDLike, C any, U any, F filters.ListFilter] struct {
	idKey   string
	parser  pkg.IDParser[ID]
	service service.Service[T, C, U, ID]

	custom map[string]func(ep *Endpoint)
	mws    []EndpointMiddleware

	acl    authz.AccessControl[ID]
	policy policy.Policy[T, ID]

	only    map[string]struct{}
	exclude map[string]struct{}

	nested []struct {
		prefix    string
		endpoints []Endpoint
	}

	finalMws []EndpointMiddleware
}

// NewCRUDBuilder creates a new builder instance
func NewCRUDBuilder[T any, ID pkg.IDLike, C any, U any, F filters.ListFilter](
	idKey string,
	parser pkg.IDParser[ID],
	svc service.Service[T, C, U, ID],
) *CRUDBuilder[T, ID, C, U, F] {
	return &CRUDBuilder[T, ID, C, U, F]{
		idKey:   idKey,
		parser:  parser,
		service: svc,
		custom:  make(map[string]func(ep *Endpoint)),
		exclude: make(map[string]struct{}),
	}
}

// WithCustom overrides a specific CRUD endpoint.
func (b *CRUDBuilder[T, ID, C, U, F]) WithCustom(name string, fn func(ep *Endpoint)) *CRUDBuilder[T, ID, C, U, F] {
	b.custom[name] = fn
	return b
}

// WithMiddleware attaches global middlewares.
func (b *CRUDBuilder[T, ID, C, U, F]) WithMiddleware(mws ...EndpointMiddleware) *CRUDBuilder[T, ID, C, U, F] {
	b.mws = append(b.mws, mws...)
	return b
}

// WithFinal adds a finalizer function for the specified CRUD endpoint, enabling post-processing logic after its execution.
func (b *CRUDBuilder[T, ID, C, U, F]) WithFinal(name string, f Finalizer) *CRUDBuilder[T, ID, C, U, F] {
	b.custom[name] = func(ep *Endpoint) {
		ep.Finalize = NewFinalizer(f)
	}
	return b
}

// WithACL attaches ACL enforcement.
func (b *CRUDBuilder[T, ID, C, U, F]) WithACL(acl authz.AccessControl[ID]) *CRUDBuilder[T, ID, C, U, F] {
	b.acl = acl
	return b
}

// WithPolicy attaches a unified Policy.
func (b *CRUDBuilder[T, ID, C, U, F]) WithPolicy(pol policy.Policy[T, ID]) *CRUDBuilder[T, ID, C, U, F] {
	b.policy = pol
	return b
}

// Only allows only selected endpoints.
func (b *CRUDBuilder[T, ID, C, U, F]) Only(names ...string) *CRUDBuilder[T, ID, C, U, F] {
	b.only = make(map[string]struct{})
	for _, n := range names {
		b.only[n] = struct{}{}
	}
	return b
}

// Exclude removes endpoints from the set.
func (b *CRUDBuilder[T, ID, C, U, F]) Exclude(names ...string) *CRUDBuilder[T, ID, C, U, F] {
	for _, n := range names {
		b.exclude[n] = struct{}{}
	}
	return b
}

// WithNested adds nested endpoints under a given prefix.
func (b *CRUDBuilder[T, ID, C, U, F]) WithNested(prefix string, endpoints []Endpoint) *CRUDBuilder[T, ID, C, U, F] {
	b.nested = append(b.nested, struct {
		prefix    string
		endpoints []Endpoint
	}{prefix, endpoints})
	return b
}

// Build constructs all endpoints with ACL, Policy, customizations and nesting.
func (b *CRUDBuilder[T, ID, C, U, F]) Build() []Endpoint {
	eps := NewBaseEndpoints[T, ID, C, U, F](b.idKey, b.parser, b.service)
	var final []Endpoint

	for i := range eps {
		ep := eps[i]

		// filtering
		if b.only != nil {
			if _, ok := b.only[ep.Name]; !ok {
				continue
			}
		}
		if _, blocked := b.exclude[ep.Name]; blocked {
			continue
		}

		// global middlewares
		if len(b.mws) > 0 {
			ep.Middlewares = append(b.mws, ep.Middlewares...)
		}

		// ACL
		if b.acl != nil {
			var mw EndpointMiddleware
			switch ep.Name {
			case "List":
				mw = ACLMiddleware(b.acl, OpList, nil)
			case "Create":
				mw = ACLMiddleware(b.acl, OpCreate, nil)
			case "Get", "Delete":
				mw = ACLMiddleware(b.acl, OpRead, ExtractIDOnly[ID]())
			case "Update":
				mw = ACLMiddleware(b.acl, OpUpdate, ExtractFromTuple[ID]())
			}
			if mw != nil {
				ep.Middlewares = append([]EndpointMiddleware{mw}, ep.Middlewares...)
			}
		}

		// Policy
		if b.policy != nil {
			switch ep.Name {
			case "Get":
				ep.Middlewares = append([]EndpointMiddleware{
					func(next EndpointHandler) EndpointHandler {
						return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
							id := args.(ID)
							if err := b.policy.Check(ctx, tx, policy.ActRead, policy.ObjID[ID]{Val: id}); err != nil {
								return nil, err
							}
							return next(ctx, tx, args)
						}
					},
				}, ep.Middlewares...)
			case "List":
				ep.Middlewares = append([]EndpointMiddleware{
					func(next EndpointHandler) EndpointHandler {
						return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
							if sel, ok := args.(*bun.SelectQuery); ok {
								b.policy.Scope(ctx, tx, sel)
							}
							return next(ctx, tx, args)
						}
					},
				}, ep.Middlewares...)
			case "Create":
				ep.Middlewares = append([]EndpointMiddleware{
					func(next EndpointHandler) EndpointHandler {
						return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
							if err := b.policy.Validate(ctx, tx, policy.ActCreate, args); err != nil {
								return nil, err
							}
							return next(ctx, tx, args)
						}
					},
				}, ep.Middlewares...)
			case "Update":
				ep.Middlewares = append([]EndpointMiddleware{
					func(next EndpointHandler) EndpointHandler {
						return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
							arr := args.([]any)
							id := arr[0].(ID)
							if err := b.policy.Check(ctx, tx, policy.ActUpdate, policy.ObjID[ID]{Val: id}); err != nil {
								return nil, err
							}
							if err := b.policy.Validate(ctx, tx, policy.ActUpdate, arr[1]); err != nil {
								return nil, err
							}
							return next(ctx, tx, args)
						}
					},
				}, ep.Middlewares...)
			case "Delete":
				ep.Middlewares = append([]EndpointMiddleware{
					func(next EndpointHandler) EndpointHandler {
						return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
							id := args.(ID)
							if err := b.policy.Check(ctx, tx, policy.ActDelete, policy.ObjID[ID]{Val: id}); err != nil {
								return nil, err
							}
							return next(ctx, tx, args)
						}
					},
				}, ep.Middlewares...)
			}
		}

		// custom overrides
		if fn, ok := b.custom[ep.Name]; ok {
			fn(&ep)
		}

		final = append(final, ep)
	}

	// nested endpoints
	for _, n := range b.nested {
		final = append(final, PrefixEndpoints(n.prefix, n.endpoints)...)
	}

	return final
}
