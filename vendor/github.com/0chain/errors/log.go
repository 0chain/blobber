package errors

import (
	"runtime"

	"github.com/lithammer/shortuuid/v3"
	"go.uber.org/zap"
)

var (
	logger *zap.Logger
)

// InitLogger initialize logger
func InitLogger(l *zap.Logger) {
	logger = l
}

func generateTraceID(caller string, rawError string) string {

	// logger is not initialzed,ignore trace
	if logger == nil {
		return ""
	}

	traceid := shortuuid.New()
	logger.Error(caller, zap.String("traceid", traceid), zap.String("err", rawError))
	return traceid

}

func getCallerName(skip int) string {
	const skipOffset = 2 // skip getCallerName and Callers

	pc := make([]uintptr, 1)
	numFrames := runtime.Callers(skip+skipOffset, pc[:])
	if numFrames < 1 {
		return ""
	}

	caller := runtime.FuncForPC(pc[0] - 1)

	if caller == nil {
		return ""
	}

	return caller.Name()
}
