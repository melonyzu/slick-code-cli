package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

func translateHTTPError(response *http.Response) error {
	defer response.Body.Close()
	message := fmt.Sprintf("request failed with status %d", response.StatusCode)
	var payload apiError
	if body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20)); err == nil &&
		json.Unmarshal(body, &payload) == nil && payload.Error.Message != "" {
		message = payload.Error.Message
	}
	return &types.Error{
		Kind: kindForStatus(response.StatusCode), Provider: types.ProviderOpenAI, Message: message,
	}
}

func translateTransportError(err error) error {
	kind := types.ErrorKindNetwork
	switch {
	case errors.Is(err, context.Canceled):
		kind = types.ErrorKindCanceled
	case errors.Is(err, context.DeadlineExceeded):
		kind = types.ErrorKindTimeout
	}
	return &types.Error{
		Kind: kind, Provider: types.ProviderOpenAI, Message: "request failed", Err: err,
	}
}

func kindForStatus(status int) types.ErrorKind {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return types.ErrorKindAuthentication
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return types.ErrorKindTimeout
	case status == http.StatusConflict:
		return types.ErrorKindConflict
	case status == http.StatusTooManyRequests:
		return types.ErrorKindRateLimit
	case status >= 400 && status < 500:
		return types.ErrorKindValidation
	default:
		return types.ErrorKindProvider
	}
}
