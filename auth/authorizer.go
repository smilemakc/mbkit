package auth

import (
	"context"

	pkg "github.com/smilemakc/mbkit"
	"github.com/uptrace/bun"
)

// AccessControl defines permission checks that rely only on context
// (e.g., user identity, roles, claims).
//
// Implementations should return nil if the operation is permitted,
// or a non-nil error if access should be denied.
type AccessControl[ID pkg.IDLike] interface {
	CanRead(ctx context.Context, tx bun.IDB, id ID) error
	CanList(ctx context.Context, tx bun.IDB) error
	CanCreate(ctx context.Context, tx bun.IDB) error
	CanUpdate(ctx context.Context, tx bun.IDB, id ID) error
	CanDelete(ctx context.Context, tx bun.IDB, id ID) error
}
