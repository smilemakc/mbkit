package policy

import (
	"context"
	"fmt"

	pkg "github.com/smilemakc/mbkit"
	"github.com/uptrace/bun"
)

// JoinSpec represents a specification for a JOIN clause in a SQL query.
type JoinSpec struct{ Join string }

// RelationPath defines the structure for representing a relational SQL query path with table, joins, and owner expression.
type RelationPath struct {
	FromTable string     // FromTable represents the name of the table to start the query from.
	Joins     []JoinSpec // Joins defines the sequence of JOIN clauses to traverse in the SQL query path (like "JOIN files f ON f.id = t.file_id").
	OwnerExpr string     // OwnerExpr represents the SQL expression used to identify the owner in the relation path (like "f.user_id").
	Alias     string     // Alias represents an optional alias name for the resulting table in the relational SQL query path (on default is "t").
}

// OwnerByJoinPolicy defines a policy for linking ownership based on a join path and a user ID parsing function.
type OwnerByJoinPolicy[ID pkg.IDLike, Owner pkg.IDLike] struct {
	Path        RelationPath                 // Path defines the relational SQL query path with table, joins, and owner expression
	ParseUserID pkg.IDParser[Owner]          // ParseUserID converts a user ID string into an Owner type or returns an error if parsing fails.
	ExtractID   func(payload any) (ID, bool) // ExtractID extracts the ID from the payload or returns false if the payload does not contain an ID.
}

func (p OwnerByJoinPolicy[ID, Owner]) alias() string {
	alias := p.Path.Alias
	if alias == "" {
		alias = "t"
	}
	return alias
}

// buildBaseQuery constructs a base SQL query with the table and joins specified in the RelationPath configuration.
func (p OwnerByJoinPolicy[ID, Owner]) buildBaseQuery(tx bun.IDB) *bun.SelectQuery {
	q := tx.NewSelect().TableExpr(p.Path.FromTable + " AS " + p.alias())
	for _, j := range p.Path.Joins {
		q = q.Join(j.Join)
	}
	return q
}

// getCurrentUserID retrieves and parses the current user ID from the context or returns ErrUnauthorized if unavailable.
func (p OwnerByJoinPolicy[ID, Owner]) getCurrentUserID(ctx context.Context) (Owner, error) {
	sub, ok := GetPrincipal(ctx)
	if !ok || sub == nil {
		return *new(Owner), ErrUnauthorized
	}
	return p.ParseUserID(sub.UserID)
}

// Check checks if the current user has access to the resource with the given ID.
func (p OwnerByJoinPolicy[ID, Owner]) Check(ctx context.Context, tx bun.IDB, _ Action, id ObjID[ID]) error {
	owner, err := p.getCurrentUserID(ctx)
	if err != nil {
		return err
	}
	exists, err := p.buildBaseQuery(tx).
		Where(fmt.Sprintf("%s.id = ?", p.alias()), id.Val).
		Where(p.Path.OwnerExpr+" = ?", owner).
		Limit(1).
		Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

// Scope applies the policy to the given query.
func (p OwnerByJoinPolicy[ID, Owner]) Scope(ctx context.Context, tx bun.IDB, q *bun.SelectQuery) *bun.SelectQuery {
	owner, err := p.getCurrentUserID(ctx)
	if err != nil {
		return q.Where("1=0")
	}
	return p.buildBaseQuery(tx).Where(p.Path.OwnerExpr+" = ?", owner)
}

// Validate validates the policy.
func (p OwnerByJoinPolicy[ID, Owner]) Validate(ctx context.Context, tx bun.IDB, action Action, payload any) error {
	if action != ActCreate && action != ActUpdate {
		return nil
	}
	refID, ok := p.ExtractID(payload)
	if !ok {
		return nil
	}
	return p.Check(ctx, tx, action, ObjID[ID]{Val: refID})
}
