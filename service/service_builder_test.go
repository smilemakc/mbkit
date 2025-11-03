package service

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/smilemakc/mbkit/factory"
	"github.com/smilemakc/mbkit/filters"
	"github.com/smilemakc/mbkit/repository"

	"github.com/uptrace/bun"
)

type ID uint64

func (i ID) String() string { return "" } // satisfies pkg.IDLike; adjust if needed.

// entity under test
type User struct {
	ID   ID
	Name string
}

// fake tx implements bun.IDB minimally for tests; we never call its methods directly here.
type fakeTx struct{ bun.IDB }

// fake repository that records calls
type fakeRepo struct {
	saveCalls   []string
	updateCalls []string
	deleteCalls []string
	getCalls    []string
	listCalls   int

	obj *User
}

func (r *fakeRepo) Save(ctx context.Context, tx bun.IDB, obj *User) error {
	r.saveCalls = append(r.saveCalls, obj.Name)
	r.obj = obj
	return nil
}
func (r *fakeRepo) Update(ctx context.Context, tx bun.IDB, id ID, obj *User, cols ...string) error {
	r.updateCalls = append(r.updateCalls, strings.Join(cols, ","))
	r.obj = obj
	return nil
}
func (r *fakeRepo) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	r.deleteCalls = append(r.deleteCalls, "ok")
	return nil
}
func (r *fakeRepo) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*User, error) {
	r.getCalls = append(r.getCalls, "ok")
	if r.obj == nil {
		return &User{ID: id, Name: "init"}, nil
	}
	return r.obj, nil
}
func (r *fakeRepo) List(ctx context.Context, tx bun.IDB, f filters.ListFilter) (filters.ListResponse[User], error) {
	r.listCalls++
	return filters.ListResponse[User]{Items: []User{*r.obj}}, nil
}

// Ensure fakeRepo implements repository.Repository[User, ID]
var _ repository.Repository[User, ID] = (*fakeRepo)(nil)

// fake factory with configurable behaviors
type fakeFactory struct {
	onCreate func(ctx context.Context, p string) (*User, error)
	onUpdate func(ctx context.Context, u *User, p string) ([]string, error)
}

func (f *fakeFactory) Create(ctx context.Context, params string) (*User, error) {
	return f.onCreate(ctx, params)
}
func (f *fakeFactory) Update(ctx context.Context, u *User, params string) ([]string, error) {
	return f.onUpdate(ctx, u, params)
}

var _ factory.Factory[User, string, string] = (*fakeFactory)(nil)

type testService = Service[User, string, string, ID]

func TestBuild_ErrorsOnMissingDeps(t *testing.T) {
	b := NewServiceBuilder[User, string, string, ID]()
	if _, err := b.Build(); err == nil {
		t.Fatal("expected error without repo and factory")
	}
	b = NewServiceBuilder[User, string, string, ID]().WithRepository(&fakeRepo{})
	if _, err := b.Build(); err == nil {
		t.Fatal("expected error without factory")
	}
}

func TestCreate_HooksOrderAndErrorPropagation(t *testing.T) {
	repo := &fakeRepo{}
	var order []string

	f := &fakeFactory{
		onCreate: func(ctx context.Context, p string) (*User, error) {
			order = append(order, "factory.create")
			return &User{Name: p}, nil
		},
		onUpdate: nil, // not used here
	}

	svc, err := NewServiceBuilder[User, string, string, ID]().
		WithRepository(repo).
		WithFactory(f).
		BeforeCreate(
			func(ctx context.Context, tx bun.IDB, u *User) error {
				order = append(order, "before1")
				u.Name += "_b1"
				return nil
			},
		).
		BeforeCreate(
			func(ctx context.Context, tx bun.IDB, u *User) error {
				order = append(order, "before2")
				return nil
			},
		).
		AfterCreate(
			func(ctx context.Context, tx bun.IDB, u *User) error {
				order = append(order, "after1")
				return nil
			},
		).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	got, err := svc.Create(context.Background(), &fakeTx{}, "alice")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if got.Name != "alice_b1" {
		t.Fatalf("unexpected name: %s", got.Name)
	}
	wantOrder := []string{"factory.create", "before1", "before2", "after1"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("order mismatch:\n got=%v\nwant=%v", order, wantOrder)
	}
	if len(repo.saveCalls) != 1 {
		t.Fatalf("save not called exactly once")
	}
}

func TestUpdate_BeforeAfterHooksAndColsPropagation(t *testing.T) {
	repo := &fakeRepo{obj: &User{ID: 1, Name: "init"}}
	f := &fakeFactory{
		onCreate: nil,
		onUpdate: func(ctx context.Context, u *User, p string) ([]string, error) {
			u.Name = p
			return []string{"name"}, nil
		},
	}

	var seenCols [][]string
	var afterCalled bool

	svc, err := NewServiceBuilder[User, string, string, ID]().
		WithRepository(repo).
		WithFactory(f).
		BeforeUpdate(
			func(ctx context.Context, tx bun.IDB, u *User, cols []string) ([]string, error) {
				seenCols = append(seenCols, append([]string(nil), cols...))
				return append(cols, "updated_at"), nil
			},
		).
		AfterUpdate(
			func(ctx context.Context, tx bun.IDB, u *User, cols []string) error {
				afterCalled = true
				if !reflect.DeepEqual(cols, []string{"name", "updated_at"}) {
					return errors.New("afterUpdate saw wrong cols")
				}
				return nil
			},
		).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	got, err := svc.Update(context.Background(), &fakeTx{}, 1, nil, "bob")
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if got.Name != "bob" {
		t.Fatalf("name not updated: %s", got.Name)
	}
	if !afterCalled {
		t.Fatalf("afterUpdate not called")
	}
	if len(repo.updateCalls) != 1 || repo.updateCalls[0] != "name,updated_at" {
		t.Fatalf("repo.Update called with wrong cols: %v", repo.updateCalls)
	}
	if len(seenCols) != 1 || !reflect.DeepEqual(seenCols[0], []string{"name"}) {
		t.Fatalf("beforeUpdate saw wrong initial cols: %v", seenCols)
	}
}

func TestDelete_HooksOrder(t *testing.T) {
	repo := &fakeRepo{}
	var order []string

	svc, err := NewServiceBuilder[User, string, string, ID]().
		WithRepository(repo).
		WithFactory(
			&fakeFactory{
				onCreate: func(ctx context.Context, p string) (*User, error) { return &User{Name: p}, nil },
				onUpdate: func(ctx context.Context, u *User, p string) ([]string, error) { return nil, nil },
			},
		).
		BeforeDelete(
			func(ctx context.Context, tx bun.IDB, id ID) error {
				order = append(
					order, "before",
				)
				return nil
			},
		).
		AfterDelete(func(ctx context.Context, tx bun.IDB, id ID) error { order = append(order, "after"); return nil }).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if err := svc.Delete(context.Background(), &fakeTx{}, 42); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if want := []string{"before", "after"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order mismatch: got=%v want=%v", order, want)
	}
}

func TestMiddlewares_OrderAcrossLayers(t *testing.T) {
	repo := &fakeRepo{}
	f := &fakeFactory{
		onCreate: func(ctx context.Context, p string) (*User, error) { return &User{Name: p}, nil },
		onUpdate: func(ctx context.Context, u *User, p string) ([]string, error) { return []string{"name"}, nil },
	}

	var trace []string

	// repo middleware
	rm1 := func(next repository.Repository[User, ID]) repository.Repository[User, ID] {
		return repository.Func[User, ID]{
			SaveFunc: func(ctx context.Context, tx bun.IDB, obj *User) error {
				trace = append(trace, "rm1")
				return next.Save(ctx, tx, obj)
			},
			UpdateFunc: next.Update,
			DeleteFunc: next.Delete,
			GetFunc:    next.Get,
			ListFunc:   next.List,
		}
	}
	rm2 := func(next repository.Repository[User, ID]) repository.Repository[User, ID] {
		return repository.Func[User, ID]{
			SaveFunc: func(ctx context.Context, tx bun.IDB, obj *User) error {
				trace = append(trace, "rm2")
				return next.Save(ctx, tx, obj)
			},
			UpdateFunc: next.Update,
			DeleteFunc: next.Delete,
			GetFunc:    next.Get,
			ListFunc:   next.List,
		}
	}

	// factory middleware
	fm1 := func(next factory.Factory[User, string, string]) factory.Factory[User, string, string] {
		return factory.Func[User, string, string]{
			CreateFunc: func(ctx context.Context, p string) (*User, error) {
				trace = append(trace, "fm1")
				return next.Create(ctx, p)
			},
			UpdateFunc: next.Update,
		}
	}
	fm2 := func(next factory.Factory[User, string, string]) factory.Factory[User, string, string] {
		return factory.Func[User, string, string]{
			CreateFunc: func(ctx context.Context, p string) (*User, error) {
				trace = append(trace, "fm2")
				return next.Create(ctx, p)
			},
			UpdateFunc: next.Update,
		}
	}

	// service middleware
	sm1 := func(next testService) testService {
		return Func[User, string, string, ID]{
			CreateFunc: func(ctx context.Context, tx bun.IDB, p string) (*User, error) {
				trace = append(trace, "sm1")
				return next.Create(ctx, tx, p)
			},
			GetFunc:    next.Get,
			ListFunc:   next.List,
			UpdateFunc: next.Update,
			DeleteFunc: next.Delete,
		}
	}
	sm2 := func(next testService) testService {
		return Func[User, string, string, ID]{
			CreateFunc: func(ctx context.Context, tx bun.IDB, p string) (*User, error) {
				trace = append(trace, "sm2")
				return next.Create(ctx, tx, p)
			},
			GetFunc:    next.Get,
			ListFunc:   next.List,
			UpdateFunc: next.Update,
			DeleteFunc: next.Delete,
		}
	}

	svc, err := NewServiceBuilder[User, string, string, ID]().
		WithRepository(repo).
		WithFactory(f).
		WithRepoMiddleware(rm1).
		WithRepoMiddleware(rm2).
		WithFactoryMiddleware(fm1).
		WithFactoryMiddleware(fm2).
		WithServiceMiddleware(sm1).
		WithServiceMiddleware(sm2).
		Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if _, err := svc.Create(context.Background(), &fakeTx{}, "x"); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Expectation:
	// service middlewares execute outer->inner as added: sm1 -> sm2
	// factory middlewares wrap the factory used by Create: fm1 -> fm2
	// repo middlewares wrap Save: rm1 -> rm2
	want := []string{"sm1", "sm2", "fm1", "fm2", "rm1", "rm2"}
	if !reflect.DeepEqual(trace, want) {
		t.Fatalf("trace mismatch:\n got=%v\nwant=%v", trace, want)
	}
}

var _ repository.Repository[User, ID] = repository.Func[User, ID]{}
var _ factory.Factory[User, string, string] = factory.Func[User, string, string]{}
var _ Service[User, string, string, ID] = Func[User, string, string, ID]{}
