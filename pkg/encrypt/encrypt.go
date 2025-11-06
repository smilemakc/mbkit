package encrypt

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"

	"github.com/blaskovicz/go-cryptkeeper"
	"github.com/rs/zerolog/log"
)

type ICryptor interface {
	Encode(any) (string, error)
	Decode(string, any) error
}

type Crypt struct{}

func (c *Crypt) Decode(s string, v any) error {
	if v == nil || reflect.TypeOf(v).Kind() != reflect.Ptr {
		return fmt.Errorf("val must be a non-nil pointer")
	}

	decrypted, err := cryptkeeper.Decrypt(s)
	if err != nil {
		return fmt.Errorf("decrypt error: %w", err)
	}

	target := reflect.ValueOf(v).Elem()
	targetType := target.Kind()

	switch targetType {
	case reflect.String:
		target.SetString(decrypted)
		return nil
	case reflect.Slice:
		if target.Type().Elem().Kind() == reflect.Uint8 {
			target.SetBytes([]byte(decrypted))
			return nil
		}
	}

	if json.Valid([]byte(decrypted)) {
		return json.Unmarshal([]byte(decrypted), v)
	}

	return fmt.Errorf("unsupported target type %s, and data is not valid JSON", targetType)
}

func (c *Crypt) Encode(v any) (string, error) {
	if v == nil {
		return "", fmt.Errorf("cannot encode nil value")
	}

	switch val := v.(type) {
	case string:
		return cryptkeeper.Encrypt(val)
	case []byte:
		return cryptkeeper.Encrypt(string(val))
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return cryptkeeper.Encrypt(string(data))
	}
}

var defaultCryptor ICryptor
var once sync.Once

func Cryptor() ICryptor {
	once.Do(func() {
		defaultCryptor = &Crypt{}
	})
	return defaultCryptor

}

func init() {
	passPhrase := os.Getenv("MB_PASSPHRASE") // must be 16, 24, or 32 bytes
	if passPhrase != "" {
		err := cryptkeeper.SetCryptKey([]byte(passPhrase))
		if err != nil {
			log.Fatal().Err(err).Msg("init cryptor")
		}
	}

}
