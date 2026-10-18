package plex

import (
	"errors"
	"fmt"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string           { return e.Message }
func failure(code, message string) error { return &Error{Code: code, Message: message} }
func ErrorCode(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "INTERNAL"
}

// SafeMessage never serializes transport errors or upstream response bodies.
func SafeMessage(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Message
	}
	if errors.Is(err, contextCanceled) {
		return "Operation cancelled"
	}
	var databaseError interface{ Code() int }
	if errors.As(err, &databaseError) {
		return fmt.Sprintf("Local inventory database rejected an update (code %d). Existing inventory was preserved.", databaseError.Code())
	}
	return "Plex operation failed. Check server status and try again."
}

var contextCanceled = errors.New("job cancelled")
