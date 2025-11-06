package service

import (
	"context"
	"testing"

	"github.com/smilemakc/mbkit/domain/filters"
	"github.com/uptrace/bun"
)

type sid uint64

func (s sid) String() string { return "" }

type ent struct{ V int }
type txStub struct{ bun.IDB }

func TestServiceFunc_Delegation(t *testing.T) {
	sf := Func[ent, int, int, sid]{
		CreateFunc: func(ctx context.Context, tx bun.IDB, p int) (*ent, error) { return &ent{V: p}, nil },
		GetFunc:    func(ctx context.Context, tx bun.IDB, id sid, _ GetterArgs) (*ent, error) { return &ent{V: 1}, nil },
		ListFunc: func(ctx context.Context, tx bun.IDB, f filters.ListFilter) (filters.ListResponse[ent], error) {
			return filters.ListResponse[ent]{Items: []ent{{V: 2}}}, nil
		},
		UpdateFunc: func(ctx context.Context, tx bun.IDB, id sid, args GetterArgs, p int) (*ent, error) {
			return &ent{V: p}, nil
		},
		DeleteFunc: func(ctx context.Context, tx bun.IDB, id sid) error { return nil },
	}

	tx := &txStub{}
	if e, err := sf.Create(context.Background(), tx, 5); err != nil || e.V != 5 {
		t.Fatalf("Create failed: %v", err)
	}
	if e, err := sf.Get(context.Background(), tx, sid(1), nil); err != nil || e.V != 1 {
		t.Fatalf("Get failed: %v", err)
	}
	if lr, err := sf.List(context.Background(), tx, filters.BaseListFilter{}); err != nil || len(lr.Items) != 1 {
		t.Fatalf("List failed: %v", err)
	}
	if e, err := sf.Update(context.Background(), tx, sid(1), nil, 9); err != nil || e.V != 9 {
		t.Fatalf("Update failed: %v", err)
	}
	if err := sf.Delete(context.Background(), tx, sid(1)); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}

func TestServiceFunc_NotImplemented(t *testing.T) {
	var sf Func[ent, int, int, sid]
	if _, err := sf.Create(context.Background(), &txStub{}, 0); err == nil {
		t.Fatal("expected error on Create")
	}
	if _, err := sf.Get(context.Background(), &txStub{}, sid(0), nil); err == nil {
		t.Fatal("expected error on Get")
	}
	if _, err := sf.List(context.Background(), &txStub{}, filters.BaseListFilter{}); err == nil {
		t.Fatal("expected error on List")
	}
	if _, err := sf.Update(context.Background(), &txStub{}, sid(0), nil, 0); err == nil {
		t.Fatal("expected error on Update")
	}
	if err := sf.Delete(context.Background(), &txStub{}, sid(0)); err == nil {
		t.Fatal("expected error on Delete")
	}
}
