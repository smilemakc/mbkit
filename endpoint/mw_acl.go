package httpapi

import (
	"context"
	"fmt"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/auth"
	"github.com/smilemakc/mbkit/errors"
	"github.com/uptrace/bun"
)

func ExtractIDOnly[ID any]() IDExtractor[ID] {
	return func(args any) (ID, error) {
		id, ok := args.(ID)
		if !ok {
			var z ID
			return z, fmt.Errorf("%w: args has unexpected type %T", errors.ErrValidation, args)
		}
		return id, nil
	}
}
func ExtractFromTuple[ID any]() IDExtractor[ID] {
	return func(args any) (ID, error) {
		arr, ok := args.([]any)
		if !ok || len(arr) == 0 {
			var z ID
			return z, fmt.Errorf("%w: args has unexpected type %T", errors.ErrValidation, args)
		}
		id, ok := arr[0].(ID)
		if !ok {
			var z ID
			return z, fmt.Errorf("%w: args[0] has unexpected type %T", errors.ErrValidation, args)
		}
		return id, nil
	}
}

type ACLOperation string

const (
	OpList   ACLOperation = "list"
	OpCreate ACLOperation = "create"
	OpRead   ACLOperation = "read"
	OpUpdate ACLOperation = "update"
	OpDelete ACLOperation = "delete"
)

func callACL[ID pkg.IDLike](
	ctx context.Context,
	tx bun.IDB,
	acl auth.AccessControl[ID],
	op ACLOperation,
	id *ID,
) error {
	switch op {
	case OpList:
		return acl.CanList(ctx, tx)
	case OpCreate:
		return acl.CanCreate(ctx, tx)
	case OpRead:
		return acl.CanRead(ctx, tx, *id)
	case OpUpdate:
		return acl.CanUpdate(ctx, tx, *id)
	case OpDelete:
		return acl.CanDelete(ctx, tx, *id)
	}
	return nil
}

type IDExtractor[ID any] func(args any) (ID, error)

func ACLMiddleware[ID pkg.IDLike](
	acl auth.AccessControl[ID],
	op ACLOperation,
	extract IDExtractor[ID],
) EndpointMiddleware {
	return func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			var id *ID
			if extract != nil {
				val, err := extract(args)
				if err != nil {
					return nil, fmt.Errorf("extract id: %w", err)
				}
				id = &val
			}
			if err := callACL(ctx, tx, acl, op, id); err != nil {
				return nil, errors.ErrPermission
			}
			return next(ctx, tx, args)
		}
	}
}
