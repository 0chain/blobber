package logging

import (
	"os"
	"sync"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger *zap.Logger
	once   sync.Once
)

func InitLogging(mode, logDir, logFile string) {
	once.Do(func() {
		initLogging(mode, logDir, logFile)
	})
}

func initLogging(mode, logDir, logFile string) {
	var logName = logDir + "/" + logFile

	var logWriter = getWriteSyncer(logName)

	var cfg zap.Config
	if mode != "development" {
		cfg = zap.NewProductionConfig()
		cfg.DisableCaller = true
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.LevelKey = "level"
		cfg.EncoderConfig.NameKey = "name"
		cfg.EncoderConfig.MessageKey = "msg"
		cfg.EncoderConfig.CallerKey = "caller"
		cfg.EncoderConfig.StacktraceKey = "stacktrace"
		if viper.GetBool("logging.console") {
			logWriter = zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), logWriter)
		}
	}
	if err := cfg.Level.UnmarshalText([]byte(viper.GetString("logging.level"))); err != nil {
		panic(err)
	}

	cfg.Encoding = "console"
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	mlcfg := zap.NewProductionConfig()
	if mode != "development" {
		mlcfg.Level.SetLevel(zapcore.ErrorLevel)
	} else {
		mlcfg.Level.SetLevel(zapcore.DebugLevel)
	}

	option := createOptionFromCores(createZapCore(logWriter, cfg))
	l, err := cfg.Build(option)
	if err != nil {
		panic(err)
	}

	Logger = l
}

func createZapCore(ws zapcore.WriteSyncer, conf zap.Config) zapcore.Core {
	enc := getEncoder(conf)
	return zapcore.NewCore(enc, ws, conf.Level)
}

func createOptionFromCores(cores ...zapcore.Core) zap.Option {
	return zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(cores...)
	})
}

func getEncoder(conf zap.Config) zapcore.Encoder {
	var enc zapcore.Encoder
	switch conf.Encoding {
	case "json":
		enc = zapcore.NewJSONEncoder(conf.EncoderConfig)
	case "console":
		enc = zapcore.NewConsoleEncoder(conf.EncoderConfig)
	default:
		panic("unknown encoding")
	}
	return enc
}

// SetOutput replaces existing Core with new, that writes to passed WriteSyncer.
func SetOutput(ws zapcore.WriteSyncer, conf zap.Config) zap.Option {
	var enc zapcore.Encoder
	switch conf.Encoding {
	case "json":
		enc = zapcore.NewJSONEncoder(conf.EncoderConfig)
	case "console":
		enc = zapcore.NewConsoleEncoder(conf.EncoderConfig)
	default:
		panic("unknown encoding")
	}

	return zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewCore(enc, ws, conf.Level)
	})
}

func getWriteSyncer(logName string) zapcore.WriteSyncer {
	var ioWriter = &lumberjack.Logger{
		Filename:   logName,
		MaxSize:    1024, // MB
		MaxBackups: 3,    // number of backups
		MaxAge:     28,   //days
		LocalTime:  true,
		Compress:   false, // disabled by default
	}
	_ = ioWriter.Rotate()
	return zapcore.AddSync(ioWriter)
}
