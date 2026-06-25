package logging

import (
	"io"
	"log"
	"os"
)

type Level int

const (
	LevelOff Level = iota
	LevelError
	LevelInfo
	LevelDebug
)

func ParseLevel(s string) Level {
	switch s {
	case "error":
		return LevelError
	case "info":
		return LevelInfo
	case "debug":
		return LevelDebug
	default:
		return LevelOff
	}
}

var (
	current = LevelOff
	logger  = log.New(io.Discard, "", log.LstdFlags)
)

// Init sets the active level and log destination. On any open failure it
// silently falls back to io.Discard — logging is a debugging aid, never a
// hard dependency.
func Init(level Level, path string) {
	current = level
	if level == LevelOff || path == "" {
		logger = log.New(io.Discard, "", log.LstdFlags)
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logger = log.New(io.Discard, "", log.LstdFlags)
		return
	}
	logger = log.New(f, "", log.LstdFlags)
}

func Errorf(format string, args ...any) { logAt(LevelError, "ERROR ", format, args...) }
func Infof(format string, args ...any)  { logAt(LevelInfo, "INFO ", format, args...) }
func Debugf(format string, args ...any) { logAt(LevelDebug, "DEBUG ", format, args...) }

func logAt(l Level, prefix, format string, args ...any) {
	if current < l || current == LevelOff {
		return
	}
	logger.Printf(prefix+format, args...)
}
