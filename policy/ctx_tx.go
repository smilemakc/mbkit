package policy

import (
	"context"

	"github.com/uptrace/bun"
)

type txKey struct{}

func WithTx(ctx context.Context, tx bun.IDB) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}
func TxFrom(ctx context.Context) (bun.IDB, bool) {
	tx, ok := ctx.Value(txKey{}).(bun.IDB)
	return tx, ok
}
