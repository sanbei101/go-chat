package logger

import (
	"os"

	"github.com/phuslu/log"
)

func InitLogger() {
	log.DefaultLogger = log.Logger{
		Level:  log.InfoLevel,
		Caller: 1,
		Writer: &log.IOWriter{Writer: os.Stdout},
	}
}
