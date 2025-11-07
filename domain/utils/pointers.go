package utils

import (
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
)

func Ptr[T ~string | Number | bool | time.Time | uuid.UUID](val T) *T {
	return &val
}

func SafePtrString[T fmt.Stringer](ptr T) string {
	if reflect.ValueOf(ptr).IsNil() {
		return ""
	}
	return ptr.String()
}
