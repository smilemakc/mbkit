package policy

import "fmt"

var (
	ErrUnauthorized = fmt.Errorf("unauthorized")
	ErrForbidden    = fmt.Errorf("forbidden")
	ErrNotFound     = fmt.Errorf("not found")
	ErrBadRequest   = fmt.Errorf("bad request")
	ErrPermission   = fmt.Errorf("do not have permission")
)
