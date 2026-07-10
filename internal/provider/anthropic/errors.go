package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// apiError is the error payload the API wraps failures in:
// {"type":"error","error":{"type":"...","message":"..."}}.
type apiError struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// translateHTTPError converts a non-2xx API response into a domain
// error, consuming and closing the body.
func translateHTTPError(resp *http.Response) error {
	defer resp.Body.Close()

	message := fmt.Sprintf("request failed with status %d", resp.StatusCode)
	var payload apiError
	if body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)); err == nil {
		if json.Unmarshal(body, &payload) == nil && payload.Error.Message != "" {
			message = payload.Error.Message
		}
	}

	return &types.Error{
		Kind:     kindForStatus(resp.StatusCode),
		Provider: types.ProviderAnthropic,
		Message:  message,
	}
}

// translateTransportError classifies request-level failures that never
// produced a response.
func translateTransportError(err error) error {
	kind := types.ErrorKindNetwork
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &types.Error{
		Kind:     kind,
		Provider: types.ProviderAnthropic,
		Message:  "request failed",
		Err:      err,
	}
}

// kindForStatus maps HTTP status codes onto domain error kinds.
func kindForStatus(status int) types.ErrorKind {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return types.ErrorKindAuthentication
	case status == http.StatusTooManyRequests:
		return types.ErrorKindRateLimit
	case status >= 400 && status < 500:
		return types.ErrorKindValidation
	default:
		return types.ErrorKindProvider
	}
}
