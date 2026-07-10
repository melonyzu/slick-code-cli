package types

import (
	"errors"
	"fmt"
)

// ErrorKind classifies a failure into one of Slick Code's shared error
// categories. Providers map their own API errors into these kinds at the
// integration boundary, so the rest of the application never inspects
// provider-specific error shapes.
type ErrorKind string

// Shared error categories.
const (
	// ErrorKindAuthentication indicates missing or rejected credentials.
	ErrorKindAuthentication ErrorKind = "authentication"

	// ErrorKindRateLimit indicates the provider refused the request due
	// to rate or quota limits.
	ErrorKindRateLimit ErrorKind = "rate_limit"

	// ErrorKindInvalidConfig indicates unacceptable configuration values.
	ErrorKindInvalidConfig ErrorKind = "invalid_config"

	// ErrorKindUnsupportedCapability indicates an operation the selected
	// provider or model does not support.
	ErrorKindUnsupportedCapability ErrorKind = "unsupported_capability"

	// ErrorKindNetwork indicates a transport-level failure such as a
	// timeout or connection error.
	ErrorKindNetwork ErrorKind = "network"

	// ErrorKindProvider indicates the provider accepted the request but
	// failed to serve it (e.g. a 5xx response).
	ErrorKindProvider ErrorKind = "provider"

	// ErrorKindValidation indicates invalid input, such as a malformed
	// request or an unknown provider or model name.
	ErrorKindValidation ErrorKind = "validation"

	// ErrorKindPermissionDenied indicates an operation the current
	// permission policy does not allow, such as a tool call requiring
	// access the user has not granted.
	ErrorKindPermissionDenied ErrorKind = "permission_denied"

	// ErrorKindTimeout indicates an operation that was abandoned because
	// it exceeded its time budget.
	ErrorKindTimeout ErrorKind = "timeout"

	// ErrorKindCanceled indicates an operation stopped because its caller
	// canceled the associated context.
	ErrorKindCanceled ErrorKind = "canceled"

	// ErrorKindConflict indicates an operation refused because the state
	// it depends on changed underneath it, such as a file edit whose
	// target was modified after it was read.
	ErrorKindConflict ErrorKind = "conflict"

	// ErrorKindInternal indicates a bug or unclassified failure inside
	// Slick Code itself.
	ErrorKindInternal ErrorKind = "internal"
)

// Error is Slick Code's domain error: a classified failure, optionally
// attributed to a provider and optionally wrapping an underlying cause.
type Error struct {
	// Kind is the failure category.
	Kind ErrorKind

	// Provider is the provider the failure is attributed to, if any.
	Provider Provider

	// Message is a human-readable description of the failure.
	Message string

	// Err is the underlying cause, if any.
	Err error
}

// NewError returns an Error of the given kind.
func NewError(kind ErrorKind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

// WrapError returns an Error of the given kind wrapping err as its cause.
func WrapError(kind ErrorKind, message string, err error) *Error {
	return &Error{Kind: kind, Message: message, Err: err}
}

// Error implements the error interface.
func (e *Error) Error() string {
	msg := e.Message
	if e.Provider != "" {
		msg = fmt.Sprintf("%s: %s", e.Provider, msg)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", msg, e.Err)
	}
	return msg
}

// Unwrap returns the underlying cause, enabling errors.Is and errors.As
// to traverse it.
func (e *Error) Unwrap() error {
	return e.Err
}

// KindOf returns the ErrorKind of err if err is, or wraps, an *Error,
// and ErrorKindInternal otherwise. A nil err has no kind and returns "".
func KindOf(err error) ErrorKind {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return ErrorKindInternal
}
