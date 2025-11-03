package httpapi

import (
	"context"
	"fmt"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/auth"
	"github.com/smilemakc/mbkit/filters"
	"github.com/smilemakc/mbkit/internal/l"
	"github.com/smilemakc/mbkit/service"
	"github.com/uptrace/bun"
)

type Request interface {
	Context() context.Context

	PathParam(key string) (string, bool)
	QueryParam(key string) (string, bool)
	Header(key string) string

	BindJSON(dst any) error
	BindForm(dst any) error
	BindQuery(dst any) error
}

// EndpointHandler defines the function signature for handling an endpoint request.
// It receives a context, a database transaction, and the parsed arguments, returning a result or an error.
type EndpointHandler func(ctx context.Context, tx bun.IDB, args any) (result any, err error)

type Finalizer func(ctx context.Context, tx bun.IDB, res any, err error) (any, error)

func NewFinalizer(fn Finalizer) EndpointMiddleware {
	return func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			res, err := next(ctx, tx, args)
			return fn(ctx, tx, res, err)
		}
	}
}

// EndpointMiddleware represents a middleware function that wraps an EndpointHandler.
type EndpointMiddleware func(next EndpointHandler) EndpointHandler

// EndpointBinder is responsible for binding input data (from path, query, or body) into strongly typed arguments.
type EndpointBinder func(r Request) (args any, err error)

// Endpoint represents a single HTTP endpoint with configuration, binding, handler, and middleware.
type Endpoint struct {
	Name   string
	Method string
	Path   string

	Bind        EndpointBinder
	Handle      EndpointHandler
	Middlewares []EndpointMiddleware
	Finalize    EndpointMiddleware

	Status int // 0 => auto (200/201/204)
	Tags   []string
}

// Chain applies middlewares in reverse order and returns the final wrapped handler.
func (e Endpoint) Chain() EndpointHandler {
	h := e.Handle
	for i := len(e.Middlewares) - 1; i >= 0; i-- {
		h = e.Middlewares[i](h)
	}
	if e.Finalize != nil {
		h = e.Finalize(h)
	}
	return h
}

// EndpointBuilder is a builder for constructing Endpoint instances step-by-step with validation.
type EndpointBuilder struct {
	ep Endpoint
}

// NewEndpoint initializes a new EndpointBuilder with the provided endpoint name.
func NewEndpoint(name string) *EndpointBuilder {
	return &EndpointBuilder{ep: Endpoint{Name: name}}
}

// WithName sets the endpoint name.
func (b *EndpointBuilder) WithName(name string) *EndpointBuilder {
	b.ep.Name = name
	return b
}

// WithMethod sets the HTTP method for the endpoint.
func (b *EndpointBuilder) WithMethod(method string) *EndpointBuilder {
	b.ep.Method = method
	return b
}

// WithPath sets the endpoint path.
func (b *EndpointBuilder) WithPath(path string) *EndpointBuilder {
	b.ep.Path = path
	return b
}

// WithBind sets the endpoint binder, responsible for extracting arguments from the request.
func (b *EndpointBuilder) WithBind(binder EndpointBinder) *EndpointBuilder {
	b.ep.Bind = binder
	return b
}

// WithHandle sets the endpoint handler function.
func (b *EndpointBuilder) WithHandle(handler EndpointHandler) *EndpointBuilder {
	b.ep.Handle = handler
	return b
}

// Use appends middleware(s) to the endpoint.
func (b *EndpointBuilder) Use(mw ...EndpointMiddleware) *EndpointBuilder {
	b.ep.Middlewares = append(b.ep.Middlewares, mw...)
	return b
}

// WithStatus sets the custom HTTP status code for the endpoint.
func (b *EndpointBuilder) WithStatus(code int) *EndpointBuilder {
	b.ep.Status = code
	return b
}

// WithTags attaches tags for documentation or grouping purposes.
func (b *EndpointBuilder) WithTags(tags ...string) *EndpointBuilder {
	b.ep.Tags = append(b.ep.Tags, tags...)
	return b
}

func (b *EndpointBuilder) UseFinal(mw EndpointMiddleware) *EndpointBuilder {
	b.ep.Finalize = mw
	return b
}

// Build validates required fields and constructs the Endpoint instance.
func (b *EndpointBuilder) Build() (Endpoint, error) {
	if b.ep.Name == "" {
		return Endpoint{}, fmt.Errorf("endpoint name is required")
	}
	if b.ep.Method == "" {
		return Endpoint{}, fmt.Errorf("endpoint method is required")
	}
	if b.ep.Handle == nil {
		return Endpoint{}, fmt.Errorf("endpoint handle function is required")
	}
	return b.ep, nil
}

// HTTPService is an interface that represents a collection of HTTP endpoints.
type HTTPService interface {
	Endpoints() []Endpoint
}

func JustLogHandler(ctx context.Context, tx bun.IDB, args any) (result any, err error) {
	l.Log.Debug().Interface("args", args).Msg("JustLogHandler")
	return nil, nil
}

func JustCustomLogHandler(fn func(ctx context.Context, args any)) func(ctx context.Context, tx bun.IDB, args any) (result any, err error) {
	return func(ctx context.Context, tx bun.IDB, args any) (result any, err error) {
		fn(ctx, args)
		return nil, nil
	}
}

func PrefixEndpoints(prefix string, eps []Endpoint) []Endpoint {
	out := make([]Endpoint, len(eps))
	for i, e := range eps {
		e.Path = prefix + e.Path
		out[i] = e
	}
	return out
}

// NewBaseEndpoints generates a standard set of Service endpoints.
func NewBaseEndpoints[T any, ID pkg.IDLike, C any, U any, F filters.ListFilter](idKey string, parseID pkg.IDParser[ID], svc service.Service[T, C, U, ID]) []Endpoint {
	list := Endpoint{
		Name:   "List",
		Method: "GET",
		Path:   "",
		Bind:   BindQuery[F](),
		Handle: func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			return svc.List(ctx, tx, *args.(*F))
		},
	}
	get := Endpoint{
		Name:   "Get",
		Method: "GET",
		Path:   "/:" + idKey,
		Bind:   PathParam[ID](idKey, parseID), // extract id parma value from request
		Handle: func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			return svc.Get(ctx, tx, args.(ID), nil)
		},
	}
	create := Endpoint{
		Name:   "Create",
		Method: "POST",
		Path:   "",
		Bind:   BindJSON[C](),
		Handle: func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			return svc.Create(ctx, tx, *args.(*C))
		},
	}
	update := Endpoint{
		Name:   "Update",
		Method: "PATCH",
		Path:   "/:" + idKey,
		Bind:   CombineBind(PathParam[ID](idKey, parseID), BindJSON[U]()), // extract id param and json body
		Handle: func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			arr := args.([]any)
			id := arr[0].(ID)
			up := *arr[1].(*U)
			return svc.Update(ctx, tx, id, nil, up)
		}}
	del := Endpoint{
		Name:   "Delete",
		Method: "DELETE",
		Path:   "/:" + idKey, Status: 204,
		Bind: PathParam[ID](idKey, parseID),
		Handle: func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			return nil, svc.Delete(ctx, tx, args.(ID))
		}}
	return []Endpoint{list, get, create, update, del}
}

func NewProtectCRUD[ID pkg.IDLike](eps []Endpoint, acl auth.AccessControl[ID]) []Endpoint {
	for i := range eps {
		var mw EndpointMiddleware
		switch eps[i].Name {
		case "List":
			mw = ACLMiddleware(acl, OpList, nil)
		case "Create":
			mw = ACLMiddleware(acl, OpCreate, nil)
		case "Get", "Delete":
			mw = ACLMiddleware(acl, OpRead, ExtractIDOnly[ID]())
		case "Update":
			mw = ACLMiddleware(acl, OpUpdate, ExtractFromTuple[ID]())
		}

		if mw != nil {
			eps[i].Middlewares = append([]EndpointMiddleware{mw}, eps[i].Middlewares...)
		}
	}
	return eps
}
