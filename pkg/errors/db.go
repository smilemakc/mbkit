package errors

import (
	"errors"
	"fmt"

	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
	notNullViolation    = "23502"
)

// Custom errors
var (
	ErrDuplicateKey = errors.New("unique constraint violation")
	ErrForeignKey   = errors.New("foreign key violation")
	ErrNotNull      = errors.New("not null violation")
)

// AsPGDriverError unwraps error into pgdriver.Error if possible
func AsPGDriverError(err error) *pgdriver.Error {
	var e pgdriver.Error
	if errors.As(err, &e) {
		return &e
	}
	return nil
}

// IsPGCodeError checks error by Postgres error code
func IsPGCodeError(err error, code string) bool {
	if v := AsPGDriverError(err); v != nil {
		return v.Field('C') == code
	}
	return false
}

// IsUniqueError checks if error is unique constraint violation
func IsUniqueError(err error) bool {
	return IsPGCodeError(err, uniqueViolation)
}

// IsForeignKeyError checks if error is foreign key violation
func IsForeignKeyError(err error) bool {
	return IsPGCodeError(err, foreignKeyViolation)
}

// IsNotNullError checks if error is not-null violation
func IsNotNullError(err error) bool {
	return IsPGCodeError(err, notNullViolation)
}

// ParseDBError converts pgdriver.Error into a wrapped custom error
func ParseDBError(err error) error {
	if v := AsPGDriverError(err); v != nil {
		code := v.Field('C')
		msg := v.Field('M')
		detail := v.Field('D')
		constraint := v.Field('n') // constraint name
		column := v.Field('c')     // column name

		switch code {
		case uniqueViolation:
			return fmt.Errorf("%w: constraint=%s, detail=%s", ErrDuplicateKey, constraint, detail)
		case foreignKeyViolation:
			return fmt.Errorf("%w: constraint=%s, detail=%s", ErrForeignKey, constraint, detail)
		case notNullViolation:
			return fmt.Errorf("%w: column=%s, detail=%s", ErrNotNull, column, detail)
		default:
			return fmt.Errorf("postgres error [%s]: %s", code, msg)
		}
	}
	return err
}

// FieldName tries to return column or constraint name from pgdriver.Error
func FieldName(err error) string {
	if v := AsPGDriverError(err); v != nil {
		if col := v.Field('c'); col != "" {
			return col
		}
		if cons := v.Field('n'); cons != "" {
			return cons
		}
	}
	return ""
}
