package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

type apiError struct {
	Error string `json:"error"`
}

func translateHTTPError(response *http.Response, model string) error {
	defer response.Body.Close()
	message := fmt.Sprintf("request failed with status %d", response.StatusCode)
	var payload apiError
	if body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20)); err == nil &&
		json.Unmarshal(body, &payload) == nil && payload.Error != "" {
		message = payload.Error
	}
	if model != "" && (response.StatusCode == http.StatusNotFound || strings.Contains(strings.ToLower(message), "not found")) {
		return modelNotFound(model)
	}
	return &types.Error{
		Kind: kindForStatus(response.StatusCode), Provider: types.ProviderOllama, Message: message,
	}
}

func translateTransportError(err error, endpoint string) error {
	kind := types.ErrorKindNetwork
	message := "Ollama is not reachable at " + endpoint + "; start it with: ollama serve"
	switch {
	case errors.Is(err, context.Canceled):
		kind, message = types.ErrorKindCanceled, "Ollama request canceled"
	case errors.Is(err, context.DeadlineExceeded):
		kind, message = types.ErrorKindTimeout, "Ollama request timed out"
	}
	return &types.Error{Kind: kind, Provider: types.ProviderOllama, Message: message, Err: err}
}

func kindForStatus(status int) types.ErrorKind {
	switch {
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

func modelNotFound(model string) error {
	return &types.Error{
		Kind: types.ErrorKindValidation, Provider: types.ProviderOllama,
		Message: fmt.Sprintf("Ollama model %q is not installed; run: ollama pull %s", model, model),
	}
}

func ollamaValidation(message string) error {
	return &types.Error{Kind: types.ErrorKindValidation, Provider: types.ProviderOllama, Message: message}
}

func ollamaProviderError(message string, err error) error {
	return &types.Error{Kind: types.ErrorKindProvider, Provider: types.ProviderOllama, Message: message, Err: err}
}
