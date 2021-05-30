package slog

import (
	"encoding/json"
	"fmt"
	"github.com/rismaster/allris-common/config"
	"log"
)

func init() {
	log.SetFlags(0)
}

type LogMessage struct {
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
}

func logIt(level string, message string, data ...interface{}) {
	if level == "DEBUG" {
		return
	}
	logMsg := LogMessage{
		Message:  fmt.Sprintf(message, data...),
		Severity: level,
	}
	if config.Debug {
		fmt.Printf("%s: %s\n", logMsg.Severity, logMsg.Message)
	} else {
		_, err := json.Marshal(logMsg)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func Err(level string, message string, data ...interface{}) {
	logMsg := LogMessage{
		Message:  fmt.Sprintf(message, data...),
		Severity: level,
	}

	if config.Debug {
		log.Fatalf("%s: %s\n", level, logMsg.Message)
	} else {
		_, err := json.Marshal(logMsg)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func Debug(message string, data ...interface{}) {
	logIt("DEBUG", message, data...)
}

func Info(message string, data ...interface{}) {
	logIt("INFO", message, data...)
}

func Warn(message string, data ...interface{}) {
	logIt("WARNING", message, data...)
}

func Error(message string, data ...interface{}) {
	logIt("ERROR", message, data...)
}

func Fatal(message string, data ...interface{}) {
	Err("CRITICAL", message, data...)
}
