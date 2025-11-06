package policy

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	pkg "github.com/smilemakc/mbkit"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type fakePolicy[R any, ID pkg.IDLike] struct {
	where string
}

func (p fakePolicy[R, ID]) Check(ctx context.Context, tx bun.IDB, act Action, id ObjID[ID]) error {
	return nil
}
func (p fakePolicy[R, ID]) Scope(ctx context.Context, tx bun.IDB, q *bun.SelectQuery) *bun.SelectQuery {
	return q.Where(p.where)
}
func (p fakePolicy[R, ID]) Validate(ctx context.Context, tx bun.IDB, act Action, payload any) error {
	return nil
}

func newMockBunDB(t *testing.T) (*bun.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New error: %v", err)
	}
	bunDB := bun.NewDB(db, pgdialect.New())
	cleanup := func() {
		_ = bunDB.Close()
		_ = db.Close()
	}
	return bunDB, mock, cleanup
}

func TestComposeAny_Scope_OR(t *testing.T) {
	db, _, cleanup := newMockBunDB(t)
	defer cleanup()

	p1 := fakePolicy[any, uuid.UUID]{where: "f.user_id = 123"}
	p2 := fakePolicy[any, uuid.UUID]{where: "f.team_id = 456"}

	anyPol := ComposeAny[any, uuid.UUID]{Policies: []Policy[any, uuid.UUID]{p1, p2}}

	ctx := context.Background()
	q := db.NewSelect().Table("files")
	q = anyPol.Scope(ctx, db, q)

	sql := q.String()

	require.Contains(t, sql, "f.user_id = 123")
	require.Contains(t, sql, "f.team_id = 456")
	require.Contains(t, sql, "OR")
	require.Equal(t, "SELECT * FROM \"files\" WHERE ((EXISTS (SELECT * WHERE (f.user_id = 123))) OR (EXISTS (SELECT * WHERE (f.team_id = 456))))", sql)
}

func TestComposeAny_Scope_EmptyPolicies(t *testing.T) {
	db, _, cleanup := newMockBunDB(t)
	defer cleanup()

	anyPol := ComposeAny[any, uuid.UUID]{Policies: nil}

	ctx := context.Background()
	q := db.NewSelect().Table("files")
	q = anyPol.Scope(ctx, db, q)

	sql := q.String()
	require.Contains(t, sql, "1=0")
	require.Equal(t, "SELECT * FROM \"files\" WHERE (1=0)", sql)
}
