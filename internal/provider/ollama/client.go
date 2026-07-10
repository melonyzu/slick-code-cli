package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/melonyzu/slick-code-cli/internal/transport"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

const maxResponseBytes = 8 << 20

type client struct {
	http     *http.Client
	endpoint string
	logger   *slog.Logger
}

func newClient(httpClient *http.Client, endpoint string, logger *slog.Logger) *client {
	return &client{http: httpClient, endpoint: endpoint, logger: logger}
}

func (c *client) do(ctx context.Context, method, path string, body []byte, model string) (*http.Response, error) {
	newRequest := func() (*http.Request, error) {
		request, err := http.NewRequest(method, c.endpoint+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		return request, nil
	}
	started := time.Now()
	response, err := transport.DoWithRetry(ctx, c.http, newRequest)
	if err != nil {
		c.logger.Warn("ollama request failed", "operation", path, "endpoint", c.endpoint,
			"duration", time.Since(started))
		return nil, translateTransportError(err, c.endpoint)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		c.logger.Warn("ollama request rejected", "operation", path, "status", response.StatusCode,
			"duration", time.Since(started))
		return nil, translateHTTPError(response, model)
	}
	c.logger.Debug("ollama request completed", "operation", path, "status", response.StatusCode,
		"duration", time.Since(started))
	return response, nil
}

func (c *client) doJSON(ctx context.Context, method, path string, request, response any, model string) error {
	var body []byte
	var err error
	if request != nil {
		body, err = json.Marshal(request)
		if err != nil {
			return types.WrapError(types.ErrorKindInternal, "ollama: encode request", err)
		}
	}
	httpResponse, err := c.do(ctx, method, path, body, model)
	if err != nil {
		return err
	}
	defer httpResponse.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxResponseBytes+1))
	if err != nil {
		return types.WrapError(types.ErrorKindNetwork, "ollama: read response", err)
	}
	if len(payload) > maxResponseBytes {
		return ollamaProviderError("response exceeds 8 MiB limit", nil)
	}
	if err := json.Unmarshal(payload, response); err != nil {
		return types.WrapError(types.ErrorKindProvider, "ollama: decode response", err)
	}
	return nil
}

func (c *client) tags(ctx context.Context) (apiTagsResponse, error) {
	var response apiTagsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/tags", nil, &response, ""); err != nil {
		return apiTagsResponse{}, err
	}
	return response, nil
}

func (c *client) models(ctx context.Context) ([]types.Model, error) {
	tags, err := c.tags(ctx)
	if err != nil {
		return nil, err
	}
	models := make([]types.Model, 0, len(tags.Models))
	for _, tagged := range tags.Models {
		name := tagged.exactName()
		if name == "" {
			continue
		}
		var shown apiShowResponse
		if err := c.doJSON(ctx, http.MethodPost, "/api/show", apiShowRequest{Model: name}, &shown, name); err != nil {
			return nil, err
		}
		if !supportsChat(shown.Capabilities) {
			continue
		}
		models = append(models, toModel(name, shown.Capabilities))
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

func (c *client) model(ctx context.Context, model string) (types.Model, error) {
	tags, err := c.tags(ctx)
	if err != nil {
		return types.Model{}, err
	}
	for _, tagged := range tags.Models {
		if tagged.exactName() == model {
			var shown apiShowResponse
			if err := c.doJSON(ctx, http.MethodPost, "/api/show", apiShowRequest{Model: model}, &shown, model); err != nil {
				return types.Model{}, err
			}
			if !supportsChat(shown.Capabilities) {
				return types.Model{}, &types.Error{
					Kind: types.ErrorKindUnsupportedCapability, Provider: types.ProviderOllama,
					Message: "Ollama model " + model + " does not support chat completion",
				}
			}
			return toModel(model, shown.Capabilities), nil
		}
	}
	return types.Model{}, modelNotFound(model)
}

func (c *client) chat(ctx context.Context, request apiChatRequest) (apiChatResponse, error) {
	var response apiChatResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/chat", request, &response, request.Model); err != nil {
		return apiChatResponse{}, err
	}
	return response, nil
}

func (c *client) chatStream(ctx context.Context, request apiChatRequest) (*http.Response, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, types.WrapError(types.ErrorKindInternal, "ollama: encode request", err)
	}
	return c.do(ctx, http.MethodPost, "/api/chat", body, request.Model)
}
