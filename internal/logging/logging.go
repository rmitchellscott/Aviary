package logging

import (
	"fmt"
	"time"
)

// Logf logs a message with RFC3339 timestamp prefix
func Logf(format string, v ...interface{}) {
	fmt.Printf("[%s] "+format+"\n", append([]interface{}{time.Now().Format(time.RFC3339)}, v...)...)
}

// LogfWithUser logs a message with username prefix if user is provided
func LogfWithUser(username string, format string, v ...interface{}) {
	if username != "" {
		format = "[" + username + "] " + format
	}
	Logf(format, v...)
}