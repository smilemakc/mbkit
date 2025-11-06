package endpoint

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/domain/models"
	"github.com/smilemakc/mbkit/domain/trackable"
	"github.com/uptrace/bun"
)

func BindJSON[T any]() EndpointBinder {
	return func(r Request) (any, error) {
		var v T
		if err := r.BindJSON(&v); err != nil {
			return nil, err
		}
		return &v, nil
	}
}

func BindForm[T any]() EndpointBinder {
	return func(r Request) (args any, err error) {
		var v T
		if err := r.BindForm(&v); err != nil {
			return nil, err
		}
		return &v, nil
	}
}

func BindQuery[T any]() EndpointBinder {
	return func(r Request) (any, error) {
		var v T
		if err := r.BindQuery(&v); err != nil {
			return nil, err
		}
		return &v, nil
	}
}

func PathParam[ID any](key string, parse func(string) (ID, error)) EndpointBinder {
	return func(r Request) (any, error) {
		raw, ok := r.PathParam(key)
		if !ok || raw == "" {
			return nil, errors.New("missing path param: " + key)
		}
		return parse(raw)
	}
}

func CombineBind(binders ...EndpointBinder) EndpointBinder {
	return func(r Request) (any, error) {
		out := make([]any, 0, len(binders))
		for _, b := range binders {
			val, err := b(r)
			if err != nil {
				return nil, err
			}
			out = append(out, val)
		}
		return out, nil
	}
}

// ArgTransformer is a generic binder function that transforms input of type I into output of type O
type ArgTransformer[T any] func(*T, Request) (*T, error)

// BindWithTransformers combines an initial EndpointBinder with a sequence of ArgTransformer transformations to process a Request.
func BindWithTransformers[T any](binder EndpointBinder, nodes ...ArgTransformer[T]) EndpointBinder {
	return func(r Request) (any, error) {
		// initial binding
		current, err := binder(r)
		if err != nil {
			return nil, err
		}

		// Type assertion
		val, ok := current.(*T)
		if !ok {
			return nil, errors.New("type assertion failed in BindWithTransformers")
		}
		// transformations
		for _, node := range nodes {
			val, err = node(val, r)
			if err != nil {
				return nil, err
			}
		}

		return val, nil
	}
}

func NewTrackableArgTransformer[ID pkg.IDLike](trackableType trackable.Type, ctxKey string) EndpointMiddleware {
	return func(next EndpointHandler) EndpointHandler {
		return func(ctx context.Context, tx bun.IDB, args any) (any, error) {
			value := ctx.Value(ctxKey)
			if value == nil || value == "" || value == 0 || value == uuid.Nil {
				return nil, fmt.Errorf("missing context key %s", ctxKey)
			}
			trackableID, ok := value.(ID)
			if !ok {
				return nil, fmt.Errorf("unexpected trackable type %T", value)
			}
			v, ok := args.(models.TrackableDTO[ID])
			if !ok {
				return nil, fmt.Errorf("unexpected trackable type %T", args)
			}
			v.SetTrackable(trackableType, trackableID)
			args = v
			return next(ctx, tx, args)
		}
	}
}
