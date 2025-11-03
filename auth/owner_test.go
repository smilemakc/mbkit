package auth

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/smilemakc/gptunnel/internal/e"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// helper to build bun DB with sqlmock
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

func withUserID[ID any](ctx context.Context, id any) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func TestOwnerAccessControl_CanList_Create_ContextUser(t *testing.T) {
	acl := NewOwnerAccessControl[int]("user_id", "items")

	// success when user id present
	ctx := withUserID[int](context.Background(), 42)
	if err := acl.CanList(ctx, nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := acl.CanCreate(ctx, nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	// missing user id
	if err := acl.CanList(context.Background(), nil); !errors.Is(err, e.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// wrong type in context
	ctxWrong := withUserID[string](context.Background(), "oops")
	if err := acl.CanCreate(ctxWrong, nil); err == nil {
		t.Fatalf("expected error for wrong type, got nil")
	}
}

func TestOwnerAccessControl_CanRead_Update_Delete_DB(t *testing.T) {
	bunDB, mock, cleanup := newMockBunDB(t)
	defer cleanup()

	acl := NewOwnerAccessControl[int]("user_id", "items")

	ctx := withUserID[int](context.Background(), 7)
	// allow case: Exists returns a row
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT * FROM "items" WHERE (id = 123) AND (user_id = 7) LIMIT 1)`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	if err := acl.CanRead(ctx, bunDB, 123); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}

	// deny case: no rows
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT * FROM "items" WHERE (id = 123) AND (user_id = 7) LIMIT 1)`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(0))

	if err := acl.CanUpdate(ctx, bunDB, 123); !errors.Is(err, e.ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}

	// sql error case
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT * FROM "items" WHERE (id = 123) AND (user_id = 7) LIMIT 1)`)).
		WillReturnError(errors.New("db is down"))

	if err := acl.CanDelete(ctx, bunDB, 123); err == nil {
		t.Fatalf("expected sql error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOwnerOrAdminAccessControl_RoleBypass(t *testing.T) {
	// roleCheck returns true, so no db access is needed
	acl := NewOwnerOrAdminAccessControl[int]("user_id", "items", func(ctx context.Context) (bool, error) {
		return true, nil
	})
	// provide empty ctx, should still allow
	if err := acl.CanList(context.Background(), nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := acl.CanCreate(context.Background(), nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := acl.CanRead(context.Background(), nil, 1); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := acl.CanUpdate(context.Background(), nil, 1); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := acl.CanDelete(context.Background(), nil, 1); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestOwnerOrAdminAccessControl_RoleCheckError(t *testing.T) {
	acl := NewOwnerOrAdminAccessControl[int]("user_id", "items", func(ctx context.Context) (bool, error) {
		return false, errors.New("role svc err")
	})
	if err := acl.CanList(context.Background(), nil); err == nil || !regexp.MustCompile(`role check failed`).MatchString(err.Error()) {
		t.Fatalf("expected wrapped role check error, got %v", err)
	}
}

func TestOwnerOrAdminAccessControl_FallbackToOwner(t *testing.T) {
	bunDB, mock, cleanup := newMockBunDB(t)
	defer cleanup()

	// roleCheck false -> fallback to owner check
	acl := NewOwnerOrAdminAccessControl[int]("user_id", "items", func(ctx context.Context) (bool, error) {
		return false, nil
	})

	ctx := withUserID[int](context.Background(), 9)

	// allow read
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT * FROM "items" WHERE (id = 55) AND (user_id = 9) LIMIT 1)`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	if err := acl.CanRead(ctx, bunDB, 55); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}

	// deny update
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT * FROM "items" WHERE (id = 55) AND (user_id = 9) LIMIT 1)`)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(0))

	if err := acl.CanUpdate(ctx, bunDB, 55); !errors.Is(err, e.ErrPermission) {
		t.Fatalf("expected ErrPermission, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestOwnerAccessControl_CanRead(t *testing.T) {
	table := "users"
	field := "user_id"

	t.Run("owner has access", func(t *testing.T) {
		db, mock, cleanup := newMockBunDB(t)
		defer cleanup()

		query := `SELECT EXISTS (SELECT * FROM "users" WHERE (id = 'entity-id') AND (user_id = 'user-id') LIMIT 1)`
		mock.ExpectQuery(regexp.QuoteMeta(query)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		ctx := context.WithValue(context.Background(), userIDKey, "user-id")

		a := NewOwnerAccessControl[string](field, table)

		err := a.CanRead(ctx, db, "entity-id")
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("stranger denied", func(t *testing.T) {
		db, mock, cleanup := newMockBunDB(t)
		defer cleanup()

		query := `SELECT EXISTS (SELECT * FROM "users" WHERE (id = 'entity-id') AND (user_id = 'other-user') LIMIT 1)`
		mock.ExpectQuery(regexp.QuoteMeta(query)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		ctx := context.WithValue(context.Background(), userIDKey, "other-user")

		a := NewOwnerAccessControl[string](field, table)

		err := a.CanRead(ctx, db, "entity-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), e.ErrPermission.Error())
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no user in context", func(t *testing.T) {
		db, _, cleanup := newMockBunDB(t)
		defer cleanup()

		ctx := context.Background()

		a := NewOwnerAccessControl[string](field, table)

		err := a.CanRead(ctx, db, "entity-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), e.ErrNotFound.Error())
	})
}
