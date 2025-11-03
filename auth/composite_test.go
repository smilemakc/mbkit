package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/uptrace/bun"
)

type stubACL struct {
	readErr, listErr, createErr, updateErr, deleteErr error
}

func (s stubACL) CanRead(ctx context.Context, tx bun.IDB, id int) error   { return s.readErr }
func (s stubACL) CanList(ctx context.Context, tx bun.IDB) error           { return s.listErr }
func (s stubACL) CanCreate(ctx context.Context, tx bun.IDB) error         { return s.createErr }
func (s stubACL) CanUpdate(ctx context.Context, tx bun.IDB, id int) error { return s.updateErr }
func (s stubACL) CanDelete(ctx context.Context, tx bun.IDB, id int) error { return s.deleteErr }

func TestCompositeAccessControl_AllPass(t *testing.T) {
	c := NewCompositeAccessControl[int](
		stubACL{},
		stubACL{},
	)
	if err := c.CanRead(context.Background(), nil, 1); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := c.CanList(context.Background(), nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := c.CanCreate(context.Background(), nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := c.CanUpdate(context.Background(), nil, 1); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := c.CanDelete(context.Background(), nil, 1); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCompositeAccessControl_FailFast(t *testing.T) {
	firstErr := errors.New("first")
	c := NewCompositeAccessControl[int](
		stubACL{readErr: firstErr},
		stubACL{readErr: errors.New("second")},
	)
	err := c.CanRead(context.Background(), nil, 1)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected wrapped 'permission denied', got %v", err)
	}
	if !errors.Is(err, firstErr) {
		t.Fatalf("expected to keep first error, got %v", err)
	}
}
