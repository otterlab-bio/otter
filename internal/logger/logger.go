package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger
var logFile *os.File

// Init initializes the logger with console output only
func Init(verbose bool) {
	InitWithFile(verbose, "")
}

// InitWithFile initializes the logger with optional file logging
// If logFilePath is empty, only console output is used
// If logFilePath is provided, logs go to both console and file
func InitWithFile(verbose bool, logFilePath string) {
	Close()
	log = logrus.New()

	if verbose {
		log.SetLevel(logrus.DebugLevel)
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	} else {
		log.SetLevel(logrus.InfoLevel)
		log.SetFormatter(&logrus.TextFormatter{
			DisableTimestamp: true,
		})
	}

	// Default: console output only
	log.SetOutput(os.Stdout)

	// Add file logging if path is provided
	if logFilePath != "" {
		var err error
		logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// If file logging fails, still log to console
			log.Warnf("Failed to open log file %s: %v", logFilePath, err)
		} else {
			// MultiWriter: console + file
			log.SetOutput(io.MultiWriter(os.Stdout, logFile))
			log.Infof("Logging to file: %s", logFilePath)
		}
	}
}

// Close closes the log file if open
func Close() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	if log == nil {
		return logrus.NewEntry(logrus.New())
	}
	return log.WithFields(fields)
}

func Info(msg string) {
	if log == nil {
		fmt.Println("[INFO]", msg)
		return
	}
	log.Info(msg)
}

func Infof(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf("[INFO] "+format+"\n", args...)
		return
	}
	log.Infof(format, args...)
}

func Debug(msg string) {
	if log == nil {
		fmt.Println("[DEBUG]", msg)
		return
	}
	log.Debug(msg)
}

func Debugf(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf("[DEBUG] "+format+"\n", args...)
		return
	}
	log.Debugf(format, args...)
}

func Warn(msg string) {
	if log == nil {
		fmt.Println("[WARN]", msg)
		return
	}
	log.Warn(msg)
}

func Warnf(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf("[WARN] "+format+"\n", args...)
		return
	}
	log.Warnf(format, args...)
}

func Error(msg string) {
	if log == nil {
		fmt.Println("[ERROR]", msg)
		return
	}
	log.Error(msg)
}

func Errorf(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf("[ERROR] "+format+"\n", args...)
		return
	}
	log.Errorf(format, args...)
}

func Fatal(msg string) {
	if log == nil {
		fmt.Println("[FATAL]", msg)
		os.Exit(1)
		return
	}
	log.Fatal(msg)
}

func Fatalf(format string, args ...interface{}) {
	if log == nil {
		fmt.Printf("[FATAL] "+format+"\n", args...)
		os.Exit(1)
		return
	}
	log.Fatalf(format, args...)
}

func SetOutput(output io.Writer) {
	if log == nil {
		return
	}
	log.SetOutput(output)
}

func GetLogger() *logrus.Logger {
	return log
}
