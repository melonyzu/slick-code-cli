package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/internal/transport"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

const maxResponseBytes = 8 << 20

type client struct {
	http    *http.Client
	baseURL string
	key     auth.Secret
	logger  *slog.Logger
}

func newClient(httpClient *http.Client, baseURL string, key auth.Secret, logger *slog.Logger) *client {
	return &client{http: httpClient, baseURL: baseURL, key: key, logger: logger}
}

func (c *client) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	newRequest := func() (*http.Request, error) {
		request, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+c.key.Reveal())
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		return request, nil
	}

	started := time.Now()
	response, err := transport.DoWithRetry(ctx, c.http, newRequest)
	if err != nil {
		c.logger.Warn("openai request failed", "operation", path, "duration", time.Since(started))
		return nil, translateTransportError(err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		c.logger.Warn("openai request rejected", "operation", path, "status", response.StatusCode,
			"duration", time.Since(started))
		return nil, translateHTTPError(response)
	}
	c.logger.Debug("openai request completed", "operation", path, "status", response.StatusCode,
		"duration", time.Since(started))
	return response, nil
}

func (c *client) doJSON(ctx context.Context, method, path string, body []byte, target any) error {
	response, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return types.WrapError(types.ErrorKindNetwork, "openai: read response", err)
	}
	if len(payload) > maxResponseBytes {
		return openAIProviderError("response exceeds 8 MiB limit", nil)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return types.WrapError(types.ErrorKindProvider, "openai: decode response", err)
	}
	return nil
}

func (c *client) chat(ctx context.Context, request apiRequest) (apiResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return apiResponse{}, types.WrapError(types.ErrorKindInternal, "openai: encode request", err)
	}
	var response apiResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/chat/completions", body, &response); err != nil {
		return apiResponse{}, err
	}
	return response, nil
}

func (c *client) chatStream(ctx context.Context, request apiRequest) (*http.Response, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, types.WrapError(types.ErrorKindInternal, "openai: encode request", err)
	}
	return c.do(ctx, http.MethodPost, "/v1/chat/completions", body)
}

func (c *client) models(ctx context.Context) ([]types.Model, error) {
	var response apiModelList
	if err := c.doJSON(ctx, http.MethodGet, "/v1/models", nil, &response); err != nil {
		return nil, err
	}
	models := make([]types.Model, 0, len(response.Data))
	for _, model := range response.Data {
		if translated, ok := toModel(model); ok {
			models = append(models, translated)
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}
