package filters

import (
	"fmt"

	"github.com/uptrace/bun"
)

type ListResponse[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

type Pager struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

func (p Pager) GetPage() int {
	if p.Page < 1 {
		return 1
	}
	return p.Page
}

func (p Pager) GetLimit() int {
	const defaultLimit = 10
	const maxLimit = 1000
	if p.Limit < 1 {
		return defaultLimit
	}
	if p.Limit > maxLimit {
		return maxLimit
	}
	return p.Limit
}

func (p Pager) GetOffset() int {
	return (p.GetPage() - 1) * p.GetLimit()
}

type LikePager interface {
	GetOffset() int
	GetLimit() int
	GetPage() int
}

type ListFilter interface {
	LikePager
	Filter
}

type BaseListFilter struct {
	Pager
	Orders []string
}

func (b BaseListFilter) WithOrder(order string) BaseListFilter {
	b.Orders = append(b.Orders, order)
	return b
}

func (b BaseListFilter) WithLimit(limit int) BaseListFilter {
	b.Pager.Limit = limit
	return b
}

func (b BaseListFilter) Apply(q *bun.SelectQuery) *bun.SelectQuery {
	query := q.Offset(b.GetOffset()).Limit(b.GetLimit())
	for _, order := range b.Orders {
		query = query.OrderExpr(order)
	}
	return query
}

func NewBaseListFilter(orders ...string) *BaseListFilter {
	return &BaseListFilter{Orders: orders}
}

func NewCreatedAtDescListFilter(modelName string) *BaseListFilter {
	return NewBaseListFilter(fmt.Sprintf("%s.created_at desc", modelName))
}

type CustomListFilter map[string]interface{}

func (c CustomListFilter) GetOffset() int {
	return (c.GetPage() - 1) * c.GetLimit()
}

func (c CustomListFilter) GetLimit() int {
	if p, ok := c["limit"]; ok {
		if v, ok := p.(int); ok {
			return v
		}
	}
	return 100
}

func (c CustomListFilter) GetPage() int {
	if p, ok := c["page"]; ok {
		if v, ok := p.(int); ok {
			return v
		}
	}
	return 1
}

func (c CustomListFilter) Apply(q *bun.SelectQuery) *bun.SelectQuery {
	for k, v := range c {
		if k == "page" || k == "limit" || k == "order_by" {
			continue
		}
		q = q.Where(fmt.Sprintf("%s", k), v)
	}
	if orderBy, ok := c["order_by"]; ok {
		q = q.OrderExpr(orderBy.(string))
	}
	return q
}

type customListFilter struct {
	inner   ListFilter
	wrapped func(q *bun.SelectQuery) *bun.SelectQuery
}

func (c customListFilter) Apply(q *bun.SelectQuery) *bun.SelectQuery {
	if c.wrapped != nil {
		return c.wrapped(q)
	}
	if lf, ok := c.inner.(ListFilter); ok {
		return lf.Apply(q)
	}
	return q
}

func (c customListFilter) GetPage() int   { return c.inner.GetPage() }
func (c customListFilter) GetLimit() int  { return c.inner.GetLimit() }
func (c customListFilter) GetOffset() int { return c.inner.GetOffset() }

// WrapWithCustomApply
func WrapWithCustomApply(original ListFilter, wrapFn func(q *bun.SelectQuery) *bun.SelectQuery) ListFilter {
	return &customListFilter{
		inner:   original,
		wrapped: wrapFn,
	}
}
