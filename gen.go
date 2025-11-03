package pkg

import (
	"fmt"
	"strconv"

	"github.com/google/uuid"
)

type Stringable interface {
	fmt.Stringer
}

type IDLike interface {
	~string | ~int | ~int64 | ~uint | ~uint64 | uuid.UUID
}

type IDExtractor[ID IDLike] = func(payload any) (ID, bool)

// IDParser defines a function to convert string to ID.
type IDParser[ID IDLike] = func(s string) (ID, error)

var UUIDIdParser IDParser[uuid.UUID] = func(s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, fmt.Errorf("emtpy value")
	}
	parse, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, err
	}
	if parse == uuid.Nil {
		return uuid.Nil, fmt.Errorf("invalid value")
	}
	return parse, err

}

var StringIdParser IDParser[string] = func(s string) (string, error) {
	return s, nil
}

var IntIdParser IDParser[int] = func(s string) (int, error) {
	return strconv.Atoi(s)
}

var Int64IdParser IDParser[int64] = func(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
