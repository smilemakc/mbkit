package l

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	Log zerolog.Logger
)

func init() {
	output := zerolog.ConsoleWriter{Out: os.Stdout, NoColor: false}
	Log = zerolog.New(output).With().Caller().Timestamp().Logger()
	log.Logger = Log
}
