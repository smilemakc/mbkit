package httpapi

import (
	"context"

	"github.com/smilemakc/mbkit/errors"
	"github.com/smilemakc/mbkit/policy"
	"github.com/smilemakc/mbkit/roles"
	"github.com/uptrace/bun"
)

func HasRoleCheck(role roles.Role) func(ctx context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		p, ok := policy.GetPrincipal(ctx)
		if !ok || p == nil {
			return false, nil
		}
		return policy.HasRole(p, role), nil
	}
}

func RequireAuth() EndpointMiddleware {
	return func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			if p, ok := policy.GetPrincipal(ctx); !ok || p == nil {
				return nil, errors.ErrPermission
			}
			return next(ctx, tx, args)
		}
	}
}

func RequireRole(role roles.Role) EndpointMiddleware {
	return func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			ok, err := HasRoleCheck(role)(ctx)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, errors.ErrPermission
			}
			return next(ctx, tx, args)
		}
	}
}
