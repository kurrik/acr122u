package acr122u

import (
	"io"
	"os"
	"strings"

	"github.com/ebfe/scard"
	"github.com/rs/zerolog"
)

type LogLevel zerolog.Level

const (
	LogTrace = LogLevel(zerolog.TraceLevel)
	LogDebug = LogLevel(zerolog.DebugLevel)
	LogInfo  = LogLevel(zerolog.InfoLevel)
	LogWarn  = LogLevel(zerolog.WarnLevel)
	LogError = LogLevel(zerolog.ErrorLevel)
	LogFatal = LogLevel(zerolog.FatalLevel)
	LogPanic = LogLevel(zerolog.PanicLevel)
)

var (
	JSONLogger    io.Writer = os.Stderr
	ConsoleLogger           = zerolog.ConsoleWriter{Out: os.Stderr}
)

func formatStateFlag(sf scard.StateFlag) string {
	var stateStrings []string

	if sf == 0 {
		stateStrings = append(stateStrings, "StateUnaware")
	}
	if sf&scard.StateIgnore != 0 {
		stateStrings = append(stateStrings, "StateIgnore")
	}
	if sf&scard.StateChanged != 0 {
		stateStrings = append(stateStrings, "StateChanged")
	}
	if sf&scard.StateUnknown != 0 {
		stateStrings = append(stateStrings, "StateUnknown")
	}
	if sf&scard.StateUnavailable != 0 {
		stateStrings = append(stateStrings, "StateUnavailable")
	}
	if sf&scard.StateEmpty != 0 {
		stateStrings = append(stateStrings, "StateEmpty")
	}
	if sf&scard.StatePresent != 0 {
		stateStrings = append(stateStrings, "StatePresent")
	}
	if sf&scard.StateAtrmatch != 0 {
		stateStrings = append(stateStrings, "StateAtrmatch")
	}
	if sf&scard.StateExclusive != 0 {
		stateStrings = append(stateStrings, "StateExclusive")
	}
	if sf&scard.StateInuse != 0 {
		stateStrings = append(stateStrings, "StateInuse")
	}
	if sf&scard.StateMute != 0 {
		stateStrings = append(stateStrings, "StateMute")
	}
	if sf&scard.StateUnpowered != 0 {
		stateStrings = append(stateStrings, "StateUnpowered")
	}
	return strings.Join(stateStrings, " & ")
}
