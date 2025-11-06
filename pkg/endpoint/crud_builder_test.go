package endpoint

import (
	"context"
	"testing"

	"github.com/smilemakc/mbkit/pkg/filters"
	"github.com/smilemakc/mbkit/pkg/service"
	"github.com/smilemakc/mbkit/pkg/utils"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type dummyService struct{}

func (d dummyService) Create(ctx context.Context, tx bun.IDB, params any) (*string, error) {
	return utils.Ptr("created"), nil
}

func (d dummyService) Get(ctx context.Context, tx bun.IDB, id string, args service.GetterArgs) (*string, error) {
	return utils.Ptr("got"), nil
}

func (d dummyService) List(ctx context.Context, tx bun.IDB, filter filters.ListFilter) (filters.ListResponse[string], error) {
	return filters.ListResponse[string]{Items: []string{"a", "b"}}, nil
}

func (d dummyService) Update(ctx context.Context, tx bun.IDB, id string, args service.GetterArgs, params any) (*string, error) {
	return utils.Ptr("updated"), nil
}

func (d dummyService) Delete(ctx context.Context, tx bun.IDB, id string) error {
	return nil
}

func TestCRUDBuilder_MiddlewareOrder(t *testing.T) {
	var order []string

	logMw := func(label string) EndpointMiddleware {
		return func(next EndpointHandler) EndpointHandler {
			return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
				order = append(order, label)
				return next(ctx, tx, args)
			}
		}
	}

	builder := NewCRUDBuilder[string, string, any, any, filters.ListFilter](
		"id",
		func(s string) (string, error) { return s, nil },
		dummyService{},
	)

	builder.
		WithMiddleware(logMw("global")).
		WithFinal("Create", func(ctx context.Context, tx bun.IDB, res any, err error) (any, error) {
			if err != nil {
				return nil, err
			}
			order = append(order, "final")
			if v, ok := res.(string); ok {
				return v + " final", nil
			}
			return res, nil
		})

	endpoints := builder.Build()

	var createEp Endpoint
	for _, ep := range endpoints {
		if ep.Name == "Create" {
			createEp = ep
			break
		}
	}

	// подменяем основной handler на тестовый
	createEp.Handle = func(ctx context.Context, tx bun.IDB, args any) (any, error) {
		order = append(order, "handler")
		return "ok", nil
	}

	h := createEp.Chain()

	resp, err := h(context.Background(), nil, "data")
	require.NoError(t, err)
	require.IsType(t, "string", resp)
	require.Equal(t, "ok final", resp)
	require.Equal(t, []string{"global", "handler", "final"}, order)
}
