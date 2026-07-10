package auth

import "log/slog"

// redacted replaces secret values in all human-readable output.
const redacted = "[REDACTED]"

// Secret is a sensitive string, such as an API key or OAuth token. Its
// fmt and slog representations are always redacted, so a Secret embedded
// in any struct cannot leak through log statements or formatted errors.
// JSON marshaling deliberately keeps the real value: credential JSON is
// written exclusively to secure storage (see Store implementations),
// never to configuration files or logs.
type Secret string

// String implements fmt.Stringer, hiding the value from %v and %s.
func (Secret) String() string { return redacted }

// GoString implements fmt.GoStringer, hiding the value from %#v.
func (Secret) GoString() string { return redacted }

// LogValue implements slog.LogValuer, hiding the value from structured
// logs.
func (Secret) LogValue() slog.Value { return slog.StringValue(redacted) }

// Reveal returns the actual secret value. The deliberate method name
// makes every use of the raw value visible in code review.
func (s Secret) Reveal() string { return string(s) }
