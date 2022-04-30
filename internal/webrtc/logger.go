package webrtc

import (
	"fmt"

	"github.com/pion/logging"
)

// customLogger satisfies the interface logging.LeveledLogger
// a logger is created per subsystem in Pion, so you can have custom
// behavior per subsystem (ICE, DTLS, SCTP...)
type customLogger struct {
	subsystem string
	logg      func(tag string, format string, v ...any) string
}

func (c customLogger) Trace(msg string)                          {}
func (c customLogger) Tracef(format string, args ...interface{}) {}
func (c customLogger) Debug(m string)                            { c.Debugf("%s", m) }
func (c customLogger) Debugf(f string, args ...interface{})      { c.logg(c.subsystem, f, args...) }
func (c customLogger) Info(m string)                             { c.Infof("%s", m) }
func (c customLogger) Infof(f string, args ...interface{})       { c.logg(c.subsystem, f, args...) }
func (c customLogger) Warn(m string)                             { c.Warnf("%s", m) }
func (c customLogger) Warnf(f string, args ...interface{})       { c.logg(c.subsystem, f, args...) }
func (c customLogger) Error(m string)                            { c.Errorf("%s", m) }
func (c customLogger) Errorf(f string, args ...interface{})      { c.logg(c.subsystem, f, args...) }

// CustomLoggerFactory satisfies the interface logging.LoggerFactory
// This allows us to create different loggers per subsystem. So we can
// add custom behavior
type CustomLoggerFactory struct {
	Logg func(tag string, format string, v ...any) string
}

func (c CustomLoggerFactory) NewLogger(subsystem string) logging.LeveledLogger {
	fmt.Printf("Creating logger for %s \n", subsystem)
	return customLogger{
		subsystem: subsystem,
		logg:      c.Logg,
	}
}
