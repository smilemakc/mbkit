package policy

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pkg "github.com/smilemakc/mbkit"
	"github.com/uptrace/bun"
)

// ComposeAll represents a group of policies where ALL must pass.
type ComposeAll[R any, ID pkg.IDLike] struct {
	Policies []Policy[R, ID]
}

// Check requires all policies to pass; otherwise, it returns the first error.
func (c ComposeAll[R, ID]) Check(ctx context.Context, tx bun.IDB, act Action, id ObjID[ID]) error {
	for _, p := range c.Policies {
		if err := p.Check(ctx, tx, act, id); err != nil {
			return err
		}
	}
	return nil
}

// Scope applies the first available policy scope as a fallback.
func (c ComposeAll[R, ID]) Scope(ctx context.Context, tx bun.IDB, q *bun.SelectQuery) *bun.SelectQuery {
	for _, p := range c.Policies {
		q = p.Scope(ctx, tx, q)
	}
	return q
}

// Validate checks all policies for the given payload.
func (c ComposeAll[R, ID]) Validate(ctx context.Context, tx bun.IDB, act Action, payload any) error {
	for _, p := range c.Policies {
		if err := p.Validate(ctx, tx, act, payload); err != nil {
			return err
		}
	}
	return nil
}

// NewAll is a factory function for creating ComposeAll.
func NewAll[R any, ID pkg.IDLike](policies ...Policy[R, ID]) ComposeAll[R, ID] {
	return ComposeAll[R, ID]{Policies: policies}
}

// ComposeAny represents a group of policies where AT LEAST ONE must pass.
type ComposeAny[R any, ID pkg.IDLike] struct {
	Policies []Policy[R, ID]
}

// Check requires at least one policy to pass.
// Returns the last error if all policies fail.
func (c ComposeAny[R, ID]) Check(ctx context.Context, tx bun.IDB, act Action, id ObjID[ID]) error {
	if len(c.Policies) == 0 {
		return ErrForbidden
	}
	var last error
	for _, p := range c.Policies {
		if err := p.Check(ctx, tx, act, id); err == nil {
			return nil
		} else {
			last = err
		}
	}
	if last != nil {
		return last
	}
	return ErrForbidden
}

// Scope applies the first available policy scope as a fallback.
func (c ComposeAny[R, ID]) Scope(ctx context.Context, tx bun.IDB, q *bun.SelectQuery) *bun.SelectQuery {
	if len(c.Policies) == 0 {
		return q.Where("1=0")
	}

	return q.WhereGroup(" OR ", func(or *bun.SelectQuery) *bun.SelectQuery {
		for _, p := range c.Policies {
			subQ := p.Scope(ctx, tx, tx.NewSelect())
			or = or.WhereOr(fmt.Sprintf("EXISTS (%s)", subQ))
		}
		return or
	})
}

// Validate checks all policies, returning success if at least one passes.
func (c ComposeAny[R, ID]) Validate(ctx context.Context, tx bun.IDB, act Action, payload any) error {
	var lastErr error
	for _, p := range c.Policies {
		if err := p.Validate(ctx, tx, act, payload); err == nil {
			// at least one policy passed
			return nil
		} else {
			lastErr = err
		}
	}
	// all policies failed
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no valid policies")
}

// NewAny is a factory function for creating ComposeAny.
func NewAny[R any, ID pkg.IDLike](policies ...Policy[R, ID]) ComposeAny[R, ID] {
	return ComposeAny[R, ID]{Policies: policies}
}

// Compile-time checks to ensure both structs implement Policy
var (
	_ Policy[any, uuid.UUID] = (*ComposeAll[any, uuid.UUID])(nil)
	_ Policy[any, uuid.UUID] = (*ComposeAny[any, uuid.UUID])(nil)
)
