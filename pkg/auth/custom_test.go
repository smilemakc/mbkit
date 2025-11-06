package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/uptrace/bun"
)

type noopDB struct{ bun.IDB }

func TestPredicateAccessControl_Delegates(t *testing.T) {
	called := struct {
		r, l, c, u, d bool
	}{}

	wantErr := errors.New("boom")

	acl := NewPredicateACL[int](
		func(ctx context.Context, tx bun.IDB, id int) error { called.r = true; return nil },
		func(ctx context.Context, tx bun.IDB) error { called.l = true; return nil },
		func(ctx context.Context, tx bun.IDB) error { called.c = true; return nil },
		func(ctx context.Context, tx bun.IDB, id int) error { called.u = true; return nil },
		func(ctx context.Context, tx bun.IDB, id int) error { called.d = true; return nil },
	)

	if err := acl.CanRead(context.Background(), noopDB{}, 1); err != nil || !called.r {
		t.Fatalf("read not delegated, err=%v called=%v", err, called)
	}
	if err := acl.CanList(context.Background(), noopDB{}); err != nil || !called.l {
		t.Fatalf("list not delegated, err=%v called=%v", err, called)
	}
	if err := acl.CanCreate(context.Background(), noopDB{}); err != nil || !called.c {
		t.Fatalf("create not delegated, err=%v called=%v", err, called)
	}
	if err := acl.CanUpdate(context.Background(), noopDB{}, 2); err != nil || !called.u {
		t.Fatalf("update not delegated, err=%v called=%v", err, called)
	}
	if err := acl.CanDelete(context.Background(), noopDB{}, 3); err != nil || !called.d {
		t.Fatalf("delete not delegated, err=%v called=%v", err, called)
	}

	// now ensure errors propagate
	aclErr := NewPredicateACL[int](
		func(ctx context.Context, tx bun.IDB, id int) error { return wantErr },
		func(ctx context.Context, tx bun.IDB) error { return wantErr },
		func(ctx context.Context, tx bun.IDB) error { return wantErr },
		func(ctx context.Context, tx bun.IDB, id int) error { return wantErr },
		func(ctx context.Context, tx bun.IDB, id int) error { return wantErr },
	)

	if err := aclErr.CanRead(context.Background(), noopDB{}, 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected error propagation, got %v", err)
	}
}
