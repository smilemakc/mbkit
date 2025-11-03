package errors

import "fmt"

var ErrPermission = fmt.Errorf("does not hahe permission")
var ErrValidation = fmt.Errorf("validation error")
