package carrot

import (
	"fmt"
	"log"
)

type LogFn func(in *Control, format string, args ...any)

var logFn LogFn = logNone

func logNone(in *Control, format string, args ...any) {}

func logSome(in *Control, format string, args ...any) {
	log.Printf(fmt.Sprintf("[coroutine-%v] ", in.ID)+format, args...)
}

func SetLogging(enable bool) {
	if enable {
		logFn = logSome
	} else {
		logFn = logNone
	}
}
