package policy

import (
	"context"

	pkg "github.com/smilemakc/mbkit"
	"github.com/uptrace/bun"
)

// Action is a type of action.
type Action string

const (
	ActList   Action = "list"
	ActRead   Action = "read"
	ActCreate Action = "create"
	ActUpdate Action = "update"
	ActDelete Action = "delete"
)

// ObjID is a wrapper for ID type.
type ObjID[T any] struct{ Val T }

// Policy defines an interface for access control operations such as checks, scoping queries, and validating payloads.
type Policy[R any, ID pkg.IDLike] interface {
	// Check spot check (read/update/delete)
	Check(ctx context.Context, tx bun.IDB, act Action, id ObjID[ID]) error
	// Scope SQL-level selection restriction (list)
	Scope(ctx context.Context, tx bun.IDB, sel *bun.SelectQuery) *bun.SelectQuery
	// Validate validation of payloads (create/update) — link fields, etc.
	Validate(ctx context.Context, tx bun.IDB, act Action, payload any) error
}
