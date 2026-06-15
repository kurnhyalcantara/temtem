// Package apperror defines transport-agnostic application errors.
// Usecases and repositories return *Error values; the delivery layer
// (via the error-mapping interceptor and the gateway error handler)
// translates them to gRPC and HTTP responses. This package must stay free
// of transport imports.
package apperror

import "fmt"

type Code string

const (
	CodeInvalidArgument  Code = "INVALID_ARGUMENT"
	CodeUnauthenticated  Code = "UNAUTHENTICATED"
	CodePermissionDenied Code = "PERMISSION_DENIED"
	CodeNotFound         Code = "NOT_FOUND"
	CodeConflict         Code = "CONFLICT"
	CodeInternal         Code = "INTERNAL"
)

type Error struct {
	Code    Code
	Message string
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap attaches a cause that is preserved for logs/errors.Is but never
// exposed to clients (only Code and Message cross the transport boundary).
func Wrap(err error, code Code, message string) *Error {
	return &Error{Code: code, Message: message, cause: err}
}

func Invalid(message string) *Error         { return New(CodeInvalidArgument, message) }
func Unauthenticated(message string) *Error { return New(CodeUnauthenticated, message) }
func NotFound(message string) *Error        { return New(CodeNotFound, message) }
func Conflict(message string) *Error        { return New(CodeConflict, message) }
func Internal(err error) *Error             { return Wrap(err, CodeInternal, "internal error") }
