package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var (
	current            = INFO
	jsonMode           = false
	out      io.Writer = os.Stdout
	stdLog             = log.New(os.Stdout, "", 0)
)

func init() {
	if v := strings.ToUpper(strings.TrimSpace(os.Getenv("LOG_LEVEL"))); v != "" {
		switch v {
		case "DEBUG":
			current = DEBUG
		case "INFO":
			current = INFO
		case "WARN", "WARNING":
			current = WARN
		case "ERROR":
			current = ERROR
		}
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_FORMAT")), "json") {
		jsonMode = true
	}

	if logFile := strings.TrimSpace(os.Getenv("LOG_FILE")); logFile != "" {
		if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err == nil {
			if f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
				out = io.MultiWriter(os.Stdout, f)
			}
		}
	}

	stdLog.SetOutput(out)
}

func ts() string { return time.Now().Format("2006-01-02 15:04:05") }

func logf(l Level, format string, args ...any) {
	if l < current {
		return
	}
	level := [...]string{"DEBUG", "INFO", "WARN", "ERROR"}[l]
	msg := fmt.Sprintf(format, args...)

	if jsonMode {
		entry := map[string]any{
			"time":  time.Now().Format(time.RFC3339Nano),
			"level": level,
			"msg":   msg,
		}
		b, err := json.Marshal(entry)
		if err == nil {
			stdLog.Print(string(b))
			return
		}
	}

	stdLog.Printf("%s [%s] %s", ts(), level, msg)
}

func Debugf(format string, args ...any) { logf(DEBUG, format, args...) }
func Infof(format string, args ...any)  { logf(INFO, format, args...) }
func Warnf(format string, args ...any)  { logf(WARN, format, args...) }
func Errorf(format string, args ...any) { logf(ERROR, format, args...) }
