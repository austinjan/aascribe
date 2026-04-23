package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

type Logger struct {
	mu       sync.Mutex
	stderr   io.Writer
	file     *os.File
	verbose  bool
	store    string
	logPath  string
	fileOpen bool
}

func New(stderr io.Writer, storePath string, verbose bool) *Logger {
	l := &Logger{
		stderr:  stderr,
		verbose: verbose,
		store:   storePath,
		logPath: ActiveLogPath(storePath),
	}
	l.tryOpenFile()
	return l
}

func ActiveLogPath(storePath string) string {
	if storePath == "" {
		return ""
	}
	return filepath.Join(storePath, "logs", "aascribe.log")
}

func (l *Logger) Path() string {
	return l.logPath
}

func (l *Logger) Debug(message string, fields ...any) {
	if !l.verbose {
		return
	}
	l.log(LevelDebug, message, fields...)
}

func (l *Logger) Info(message string, fields ...any) {
	l.log(LevelInfo, message, fields...)
}

func (l *Logger) Warn(message string, fields ...any) {
	l.log(LevelWarn, message, fields...)
}

func (l *Logger) Error(message string, fields ...any) {
	l.log(LevelError, message, fields...)
}

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func (l *Logger) log(level Level, message string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	line := formatLine(level, message, fields...)
	if l.stderr != nil {
		_, _ = fmt.Fprintln(l.stderr, line)
	}
	if l.file != nil {
		_, _ = fmt.Fprintln(l.file, line)
	}
}

func (l *Logger) tryOpenFile() {
	if l.logPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.logPath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	l.file = file
	l.fileOpen = true
}

func formatLine(level Level, message string, fields ...any) string {
	parts := []string{
		"time=" + quoteIfNeeded(time.Now().UTC().Format(time.RFC3339)),
		"level=" + string(level),
		"msg=" + quoteIfNeeded(message),
	}

	for i := 0; i+1 < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok || key == "" {
			continue
		}
		value := fmt.Sprint(fields[i+1])
		if isSensitiveKey(key) {
			value = "[REDACTED]"
		}
		parts = append(parts, key+"="+quoteIfNeeded(value))
	}

	return strings.Join(parts, " ")
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "secret") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "api_key") ||
		strings.Contains(key, "apikey") ||
		strings.Contains(key, "password")
}

func quoteIfNeeded(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\n\r\"") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}
