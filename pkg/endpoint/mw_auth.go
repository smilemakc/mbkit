package endpoint

import (
	"context"

	"github.com/smilemakc/mbkit/pkg/policy"
	"github.com/smilemakc/mbkit/pkg/roles"
	"github.com/uptrace/bun"
)

// HasRoleCheck returns a predicate that checks whether current Principal has the given role.
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
				return nil, policy.ErrForbidden
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
				return nil, policy.ErrForbidden
			}
			return next(ctx, tx, args)
		}
	}
}
