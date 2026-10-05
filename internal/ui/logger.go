package ui

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type Level uint8

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

type Logger struct {
	writer   io.Writer
	clock    func() time.Time
	color    bool
	minLevel Level
	mu       sync.Mutex
}

func NewLogger(writer io.Writer, clock func() time.Time, color bool, minLevel Level) *Logger {
	return &Logger{
		writer:   writer,
		clock:    clock,
		color:    color,
		minLevel: minLevel,
	}
}

func (l *Logger) Debug(message string) {
	l.write(DebugLevel, "DEBUG", "36", message)
}

func (l *Logger) Verbose(message string) {
	l.write(DebugLevel, "DEBUG", "36", message)
}

func (l *Logger) Info(message string) {
	l.write(InfoLevel, "INFO", "32", message)
}

func (l *Logger) Warn(message string) {
	l.write(WarnLevel, "WARN", "33", message)
}

func (l *Logger) Error(message string) {
	l.write(ErrorLevel, "ERROR", "31", message)
}

func (l *Logger) Status(status BuildStatus) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(l.writer, RenderBuildStatus(status, l.color))
}

func (l *Logger) write(level Level, tag, colorCode, message string) {
	if level < l.minLevel {
		return
	}

	prefix := "[" + tag + "]"
	if l.color {
		prefix = "\x1b[" + colorCode + "m" + prefix + "\x1b[0m"
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.writer, "%s %s %s\n", l.clock().Format(time.RFC3339), prefix, message)
}
