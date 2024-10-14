// Description: This file implements the logger package, which is responsible for setting up the logging system.
package logger

import (
	"fmt"
	"os"
	"simple_file_server/pkg"
	"syscall"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *logrus.Logger

// checkFilePermissions checks write permissions for the file
func checkFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File does not exist, it can be created
		}
		return fmt.Errorf("failed to get file information: %w", err)
	}

	// Check that the file is writable by the current user
	if info.Mode().Perm()&(1<<(uint(7))) == 0 {
		return fmt.Errorf("the current user does not have write permissions for the file: %s", path)
	}

	return nil
}

// LogSetup configures logging
func LogSetup(config pkg.Logging) {
	Logger = logrus.New()

	// Set umask for correct permissions on created files
	oldUmask := syscall.Umask(0022) // Removes write permissions for group and others

	// Restore old umask after function execution
	defer syscall.Umask(oldUmask)

	// Check access permissions
	if err := checkFilePermissions(config.LogFile); err != nil {
		Logger.Fatalf("File permissions check failed: %v", err)
	}

	// Open or create log file with permissions 0644 (rw-r--r--)
	file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		Logger.Fatalf("Failed to open or create log file: %v", err)
	}
	file.Close()
	
	Logger.SetOutput(&lumberjack.Logger{
		Filename: 	config.LogFile,
		MaxSize:    config.LogMaxSize,
		MaxBackups: config.LogMaxFiles,
		MaxAge:     config.LogMaxAge,
		Compress:   true,
	})

	// Set logging level
	var notifyLevel logrus.Level
	switch config.LogSeverity {
		case "debug": notifyLevel = logrus.DebugLevel
		case "info": notifyLevel = logrus.InfoLevel
		case "warning": notifyLevel = logrus.WarnLevel
		case "error": notifyLevel = logrus.ErrorLevel
		case "fatal": notifyLevel = logrus.FatalLevel
		case "trace": notifyLevel = logrus.TraceLevel
		default: notifyLevel = logrus.InfoLevel
	}
	Logger.SetFormatter(&logrus.JSONFormatter{})
	Logger.SetLevel(notifyLevel)
	Logger.Printf("Logger set minimum severity is '%s'", notifyLevel.String())
	
	// Set permissions for the log file
	if err := os.Chmod(config.LogFile, 0644); err != nil {
		Logger.Fatalf("Failed to open or create log file: %v", err)
	}

	// Ensure correct permissions for rotated files
	syscall.Umask(0022)
}
