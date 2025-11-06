package endpoint

import (
	"context"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/pkg/policy"
	"github.com/uptrace/bun"
)

func PolicyRead[ID pkg.IDLike, R any](pol policy.Policy[R, ID], extract IDExtractor[ID]) EndpointMiddleware {
	return func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			id, err := extract(args)
			if err != nil {
				return nil, err
			}
			if err := pol.Check(ctx, tx, policy.ActRead, policy.ObjID[ID]{Val: id}); err != nil {
				return nil, err
			}
			return next(ctx, tx, args)
		}
	}
}

func PolicyUpdate[ID pkg.IDLike, R any](pol policy.Policy[R, ID], extract IDExtractor[ID]) EndpointMiddleware {
	return func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			id, err := extract(args)
			if err != nil {
				return nil, err
			}
			if err := pol.Check(ctx, tx, policy.ActUpdate, policy.ObjID[ID]{Val: id}); err != nil {
				return nil, err
			}
			if err := pol.Validate(ctx, tx, policy.ActUpdate, args); err != nil {
				return nil, err
			}
			return next(ctx, tx, args)
		}
	}
}

func PolicyDelete[ID pkg.IDLike, R any](pol policy.Policy[R, ID], extract IDExtractor[ID]) EndpointMiddleware {
	return func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			id, err := extract(args)
			if err != nil {
				return nil, err
			}
			if err := pol.Check(ctx, tx, policy.ActDelete, policy.ObjID[ID]{Val: id}); err != nil {
				return nil, err
			}
			return next(ctx, tx, args)
		}
	}
}

func PolicyCreate[R any, ID pkg.IDLike](pol policy.Policy[R, ID]) EndpointMiddleware {
	return func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			if err := pol.Validate(ctx, tx, policy.ActCreate, args); err != nil {
				return nil, err
			}
			return next(ctx, tx, args)
		}
	}
}
