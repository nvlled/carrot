package carrot

import (
	"fmt"
	"log"
)

type LogFn func(script *Script, format string, args ...any)

var logFn LogFn = logNone

func logNone(script *Script, format string, args ...any) {}

func logSome(script *Script, format string, args ...any) {
	log.Printf(fmt.Sprintf("[coroutine-%v] ", script.mainInvoker.id)+format, args...)
}

func SetLogging(enable bool) {
	if enable {
		logFn = logSome
	} else {
		logFn = logNone
	}
}
