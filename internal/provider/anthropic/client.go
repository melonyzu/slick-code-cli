package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/internal/transport"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// apiVersion is the anthropic-version header value this package targets.
const apiVersion = "2023-06-01"

// modelsPageSize is how many models each discovery page requests.
const modelsPageSize = 100

// client performs authenticated calls against the Anthropic API.
type client struct {
	http    *http.Client
	baseURL string
	key     auth.Secret
}

func newClient(httpClient *http.Client, baseURL string, key auth.Secret) *client {
	return &client{http: httpClient, baseURL: baseURL, key: key}
}

// do sends an API request with retries, translating any non-2xx
// response into a domain error. For streaming requests the returned
// body is the live SSE stream; the caller owns closing it.
func (c *client) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	newRequest := func() (*http.Request, error) {
		var payload *bytes.Reader
		if body != nil {
			payload = bytes.NewReader(body)
		} else {
			payload = bytes.NewReader(nil)
		}

		req, err := http.NewRequest(method, c.baseURL+path, payload)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", c.key.Reveal())
		req.Header.Set("anthropic-version", apiVersion)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	}

	resp, err := transport.DoWithRetry(ctx, c.http, newRequest)
	if err != nil {
		return nil, translateTransportError(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, translateHTTPError(resp)
	}
	return resp, nil
}

// doJSON sends an API request and decodes its JSON response into out.
func (c *client) doJSON(ctx context.Context, method, path string, body []byte, out any) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return types.WrapError(types.ErrorKindProvider, "anthropic: decode response", err)
	}
	return nil
}

// messages performs a non-streaming Messages API call.
func (c *client) messages(ctx context.Context, req apiRequest) (apiResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return apiResponse{}, types.WrapError(types.ErrorKindInternal, "anthropic: encode request", err)
	}

	var resp apiResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/messages", body, &resp); err != nil {
		return apiResponse{}, err
	}
	return resp, nil
}

// messagesStream begins a streaming Messages API call and returns the
// live response; the caller consumes and closes the body.
func (c *client) messagesStream(ctx context.Context, req apiRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, types.WrapError(types.ErrorKindInternal, "anthropic: encode request", err)
	}
	return c.do(ctx, http.MethodPost, "/v1/messages", body)
}

// models lists every model available to the account, following
// pagination.
func (c *client) models(ctx context.Context) ([]types.Model, error) {
	var (
		all   []types.Model
		after string
	)
	for {
		page, err := c.modelsPage(ctx, modelsPageSize, after)
		if err != nil {
			return nil, err
		}
		for _, m := range page.Data {
			all = append(all, toModel(m))
		}
		if !page.HasMore || len(page.Data) == 0 {
			return all, nil
		}
		after = page.Data[len(page.Data)-1].ID
	}
}

// modelsPage fetches one page of the model listing.
func (c *client) modelsPage(ctx context.Context, limit int, after string) (apiModelPage, error) {
	query := url.Values{"limit": {strconv.Itoa(limit)}}
	if after != "" {
		query.Set("after_id", after)
	}

	var page apiModelPage
	path := fmt.Sprintf("/v1/models?%s", query.Encode())
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &page); err != nil {
		return apiModelPage{}, err
	}
	return page, nil
}
