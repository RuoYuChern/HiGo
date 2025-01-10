package base

import (
	"log"
	"os"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var GLogger *zap.SugaredLogger

func InitLog(conf *HiLogConf) error {
	syncer := initLogWriter(conf)
	encoder := initEncoder()
	level, perr := zapcore.ParseLevel(conf.Level)
	if perr != nil {
		log.Fatalf("ParseLevel:%s failed", conf.Level)
		level = zapcore.InfoLevel
	}
	highPriority := zap.LevelEnablerFunc(func(l zapcore.Level) bool {
		return (l >= level)
	})
	console := zapcore.Lock(os.Stdout)
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	var core zapcore.Core
	if conf.Env == "dev" {
		core = zapcore.NewTee(zapcore.NewCore(encoder, syncer, highPriority), zapcore.NewCore(consoleEncoder, console, highPriority))
	} else {
		core = zapcore.NewCore(encoder, syncer, highPriority)
	}
	development := zap.Development()
	GLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel), development).Sugar()
	return nil
}

func initLogWriter(c *HiLogConf) zapcore.WriteSyncer {
	logger := &lumberjack.Logger{
		Filename:   c.File,
		MaxSize:    c.MaxSize,
		MaxBackups: c.MaxBackups,
		MaxAge:     c.MaxAge,
		Compress:   false,
	}
	return zapcore.AddSync(logger)
}

func initEncoder() zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(cfg)
}
