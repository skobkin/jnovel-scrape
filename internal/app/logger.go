package app

import (
	"io"
	"log"
	"sync"
)

// Logger provides leveled logging onto stderr/stdout.
type Logger struct {
	mu  sync.Mutex
	log *log.Logger
}

// NewLogger builds a Logger that writes using the provided io.Writer.
func NewLogger(w io.Writer) *Logger {
	return &Logger{
		log: log.New(w, "", log.LstdFlags),
	}
}

func (l *Logger) output(level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.log.Printf(level+" "+format, args...)
}

// Infof prints informational message.
func (l *Logger) Infof(format string, args ...any) {
	l.output("INFO", format, args...)
}

// Warnf prints warning message.
func (l *Logger) Warnf(format string, args ...any) {
	l.output("WARN", format, args...)
}

// Errorf prints error message.
func (l *Logger) Errorf(format string, args ...any) {
	l.output("ERROR", format, args...)
}
