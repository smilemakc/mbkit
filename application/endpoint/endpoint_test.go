package endpoint

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

type mockRequest struct{}

func (m *mockRequest) Context() context.Context             { return context.Background() }
func (m *mockRequest) PathParam(key string) (string, bool)  { return "", false }
func (m *mockRequest) QueryParam(key string) (string, bool) { return "", false }
func (m *mockRequest) Header(key string) string             { return "" }
func (m *mockRequest) BindJSON(dst any) error               { return nil }
func (m *mockRequest) BindForm(dst any) error               { return nil }
func (m *mockRequest) BindQuery(dst any) error              { return nil }

func TestEndpoint_Chain_NoMiddleware(t *testing.T) {
	called := false

	e := Endpoint{
		Handle: func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			called = true
			return "ok", nil
		},
	}

	final := e.Chain()
	out, err := final(context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "ok", out)
	assert.True(t, called, "handler должен быть вызван")
}

func TestEndpoint_Chain_WithMiddlewares(t *testing.T) {
	order := []string{}

	m1 := func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			order = append(order, "m1_before")
			out, err := next(ctx, tx, args)
			order = append(order, "m1_after")
			return out, err
		}
	}

	m2 := func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			order = append(order, "m2_before")
			out, err := next(ctx, tx, args)
			order = append(order, "m2_after")
			return out, err
		}
	}

	handler := func(ctx context.Context, tx bun.IDB, args any) (any, error) {
		order = append(order, "handler")
		return "ok", nil
	}

	e := Endpoint{
		Handle:      handler,
		Middlewares: []EndpointMiddleware{m1, m2},
	}

	final := e.Chain()
	_, _ = final(context.Background(), nil, nil)

	expected := []string{"m1_before", "m2_before", "handler", "m2_after", "m1_after"}
	assert.Equal(t, expected, order, "middlewares должны вызываться в правильном порядке")
}

func TestEndpoint_Chain_WithFinalize(t *testing.T) {
	order := []string{}

	m1 := func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			order = append(order, "m1")
			return next(ctx, tx, args)
		}
	}

	finalize := func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			order = append(order, "finalize_before")
			out, err := next(ctx, tx, args)
			order = append(order, "finalize_after")
			return out, err
		}
	}

	handler := func(ctx context.Context, tx bun.IDB, args any) (any, error) {
		order = append(order, "handler")
		return nil, nil
	}

	e := Endpoint{
		Handle:      handler,
		Middlewares: []EndpointMiddleware{m1},
		Finalize:    finalize,
	}

	final := e.Chain()
	_, _ = final(context.Background(), nil, nil)

	expected := []string{"finalize_before", "m1", "handler", "finalize_after"}
	assert.Equal(t, expected, order)
}

func TestEndpoint_Chain_ErrorPropagation(t *testing.T) {
	testErr := errors.New("boom")

	handler := func(ctx context.Context, tx bun.IDB, args any) (any, error) {
		return nil, testErr
	}

	m := func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			return next(ctx, tx, args)
		}
	}

	e := Endpoint{
		Handle:      handler,
		Middlewares: []EndpointMiddleware{m},
	}

	final := e.Chain()
	out, err := final(context.Background(), nil, nil)

	assert.Nil(t, out)
	assert.ErrorIs(t, err, testErr, "ошибка должна пробрасываться наружу")
}
