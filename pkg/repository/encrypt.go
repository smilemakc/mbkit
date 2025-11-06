package repository

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"

	pkg "github.com/smilemakc/mbkit"
	"github.com/smilemakc/mbkit/pkg/encrypt"
	"github.com/smilemakc/mbkit/pkg/filters"
	"github.com/uptrace/bun"
)

// CryptorRepository wraps a base Repository and transparently encrypts/decrypts fields
// marked with `encrypt:"true"` struct tags.
type CryptorRepository[T any, ID pkg.IDLike] struct {
	repo    Repository[T, ID]
	cryptor encrypt.ICryptor
}

// NewCryptorRepository creates a new CryptorRepository wrapping an existing Repository.
func NewCryptorRepository[T any, ID pkg.IDLike](repo Repository[T, ID], cryptor encrypt.ICryptor) *CryptorRepository[T, ID] {
	return &CryptorRepository[T, ID]{repo: repo, cryptor: cryptor}
}

func (c *CryptorRepository[T, ID]) Save(ctx context.Context, tx bun.IDB, item *T) error {
	if err := c.encrypt(item); err != nil {
		return err
	}
	return c.repo.Save(ctx, tx, item)
}

func (c *CryptorRepository[T, ID]) Get(ctx context.Context, tx bun.IDB, id ID, args GetterArgs) (*T, error) {
	obj, err := c.repo.Get(ctx, tx, id, args)
	if err != nil {
		return nil, err
	}
	if err := c.decrypt(obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (c *CryptorRepository[T, ID]) List(ctx context.Context, tx bun.IDB, f filters.ListFilter) (filters.ListResponse[T], error) {
	res, err := c.repo.List(ctx, tx, f)
	if err != nil {
		return res, err
	}
	for i := range res.Items {
		if err := c.decrypt(&res.Items[i]); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (c *CryptorRepository[T, ID]) Update(ctx context.Context, tx bun.IDB, id ID, item *T, columns ...string) error {
	if err := c.encrypt(item); err != nil {
		return err
	}
	return c.repo.Update(ctx, tx, id, item, columns...)
}

func (c *CryptorRepository[T, ID]) Delete(ctx context.Context, tx bun.IDB, id ID) error {
	return c.repo.Delete(ctx, tx, id)
}

func (c *CryptorRepository[T, ID]) encrypt(item *T) error {
	return processStruct(reflect.ValueOf(item).Elem(), c.cryptor, true)
}

func (c *CryptorRepository[T, ID]) decrypt(item *T) error {
	return processStruct(reflect.ValueOf(item).Elem(), c.cryptor, false)
}

func safeDecrypt(cryptor encrypt.ICryptor, in string, out *string) error {
	if in == "" {
		return nil
	}
	_, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		*out = in
		return nil
	}
	return cryptor.Decode(in, out)
}
func processStruct(v reflect.Value, cryptor encrypt.ICryptor, isEncrypt bool) error {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// recurse into nested structs
		if field.Kind() == reflect.Struct {
			if err := processStruct(field, cryptor, isEncrypt); err != nil {
				return err
			}
			continue
		}

		// only process tagged fields
		if tag := fieldType.Tag.Get("encrypt"); tag == "true" {
			switch field.Kind() {
			case reflect.String:
				if isEncrypt {
					encoded, err := cryptor.Encode(field.String())
					if err != nil {
						return fmt.Errorf("encrypt %s: %w", fieldType.Name, err)
					}
					field.SetString(encoded)
				} else {
					var decoded string
					if err := cryptor.Decode(field.String(), &decoded); err != nil {
						return fmt.Errorf("decrypt %s: %w", fieldType.Name, err)
					}
					field.SetString(decoded)
				}
				// TODO: handle other field types
			}
		}
	}
	return nil
}
