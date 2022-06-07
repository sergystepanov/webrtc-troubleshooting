package webrtc

import (
	"fmt"
	"strconv"

	"github.com/pion/logging"
)

// customLogger satisfies the interface logging.LeveledLogger
// a logger is created per subsystem in Pion, so you can have custom
// behavior per subsystem (ICE, DTLS, SCTP...)
type customLogger struct {
	subsystem string
	level     logging.LogLevel
	log       LogFn
}

type LogFn func(tag string, format string, v ...any) string

func (c customLogger) logf(level logging.LogLevel, f string, args ...interface{}) {
	if c.level.Get() < level {
		return
	}
	c.log(c.subsystem, f, args...)
}

func (c customLogger) Trace(m string) { c.logf(logging.LogLevelTrace, "%s", m) }
func (c customLogger) Tracef(f string, args ...interface{}) {
	c.logf(logging.LogLevelTrace, f, args...)
}
func (c customLogger) Debug(m string) { c.logf(logging.LogLevelDebug, "%s", m) }
func (c customLogger) Debugf(f string, args ...interface{}) {
	c.logf(logging.LogLevelDebug, f, args...)
}
func (c customLogger) Info(m string)                       { c.logf(logging.LogLevelInfo, "%s", m) }
func (c customLogger) Infof(f string, args ...interface{}) { c.logf(logging.LogLevelInfo, f, args...) }
func (c customLogger) Warn(m string)                       { c.logf(logging.LogLevelWarn, "%s", m) }
func (c customLogger) Warnf(f string, args ...interface{}) { c.logf(logging.LogLevelWarn, f, args...) }
func (c customLogger) Error(m string)                      { c.logf(logging.LogLevelError, "%s", m) }
func (c customLogger) Errorf(f string, args ...interface{}) {
	c.logf(logging.LogLevelError, f, args...)
}

// CustomLoggerFactory satisfies the interface logging.LoggerFactory
// This allows us to create different loggers per subsystem. So we can
// add custom behavior
type CustomLoggerFactory struct {
	Level logging.LogLevel
	Log   LogFn
}

func NewLoggerFactory(lvl string, fn LogFn) CustomLoggerFactory {
	logger := CustomLoggerFactory{
		Level: logging.LogLevelTrace,
		Log:   fn,
	}
	if lvl != "" {
		if l, err := strconv.Atoi(lvl); err == nil {
			logger.Level = logging.LogLevel(l)
		}
	}
	return logger
}

func (c CustomLoggerFactory) NewLogger(subsystem string) logging.LeveledLogger {
	fmt.Printf("Creating logger for %s \n", subsystem)
	return customLogger{
		subsystem: subsystem,
		level:     c.Level,
		log:       c.Log,
	}
}
