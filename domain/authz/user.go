package authz

import (
	"context"

	pkg "github.com/smilemakc/mbkit"
)

// UserIDExtractor extracts the user ID from the context.
type UserIDExtractor[ID pkg.IDLike] func(ctx context.Context) (ID, error)
