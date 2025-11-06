package filters

import (
	"fmt"

	"github.com/uptrace/bun"
)

type Filter interface {
	Apply(q *bun.SelectQuery) *bun.SelectQuery
}

type FilterFunc func(q *bun.SelectQuery) *bun.SelectQuery

func (f FilterFunc) Apply(q *bun.SelectQuery) *bun.SelectQuery {
	return f(q)
}

func CreatedAtOrderFilter(modelName string, direction string) FilterFunc {
	return func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Order(fmt.Sprintf("%s.created_at %s", modelName, direction))
	}
}

func CreatedAtDescOrderFilter(modelName string) FilterFunc {
	return CreatedAtOrderFilter(modelName, "desc")
}

func CreatedAtAscOrderFilter(modelName string) FilterFunc {
	return CreatedAtOrderFilter(modelName, "asc")
}
