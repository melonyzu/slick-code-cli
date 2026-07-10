// Package anthropic implements the Slick Code provider contracts for
// Anthropic's Messages API. All Anthropic-specific request, response,
// streaming, and error formats are translated into the domain model at
// this boundary and never escape the package.
package anthropic

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

const (
	defaultBaseURL = "https://api.anthropic.com"

	// baseURLEnv overrides the API endpoint, for proxies and testing.
	baseURLEnv = "SLICKCODE_ANTHROPIC_BASE_URL"
)

// Provider is the Anthropic provider. It is registered at bootstrap and
// holds no credentials until activated.
type Provider struct {
	httpClient *http.Client
	logger     *slog.Logger
	baseURL    string

	mu     sync.RWMutex
	client *client // non-nil while activated
}

// New returns an inactive Anthropic provider that will communicate
// through httpClient.
func New(httpClient *http.Client, logger *slog.Logger) *Provider {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	baseURL := os.Getenv(baseURLEnv)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Provider{
		httpClient: httpClient,
		logger:     logger,
		baseURL:    baseURL,
	}
}

// Name implements provider.Provider.
func (*Provider) Name() types.Provider {
	return types.ProviderAnthropic
}

// Models implements provider.Provider, listing the models the
// authenticated account can use.
func (p *Provider) Models(ctx context.Context) ([]types.Model, error) {
	c, err := p.activeClient()
	if err != nil {
		return nil, err
	}
	return c.models(ctx)
}

// AuthMethods implements provider.Authenticator.
func (*Provider) AuthMethods() []auth.Method {
	return []auth.Method{auth.MethodAPIKey}
}

// NewFlow implements provider.Authenticator.
func (*Provider) NewFlow(method auth.Method) (auth.Flow, error) {
	if method != auth.MethodAPIKey {
		return nil, &types.Error{
			Kind:     types.ErrorKindUnsupportedCapability,
			Provider: types.ProviderAnthropic,
			Message:  "anthropic supports only api_key authentication",
		}
	}
	return apiKeyFlow{}, nil
}

// Activate implements provider.Activator, building the API client from
// the session credential.
func (p *Provider) Activate(ctx context.Context, cred auth.Credential) error {
	if cred.Secret == "" {
		return &types.Error{
			Kind:     types.ErrorKindAuthentication,
			Provider: types.ProviderAnthropic,
			Message:  "credential holds no API key",
		}
	}

	p.mu.Lock()
	p.client = newClient(p.httpClient, p.baseURL, cred.Secret)
	p.mu.Unlock()
	return nil
}

// Deactivate implements provider.Deactivator, dropping the API client
// and with it the credential.
func (p *Provider) Deactivate(ctx context.Context) error {
	p.mu.Lock()
	p.client = nil
	p.mu.Unlock()
	return nil
}

// CheckHealth implements provider.HealthChecker with the cheapest
// authenticated call the API offers.
func (p *Provider) CheckHealth(ctx context.Context) error {
	c, err := p.activeClient()
	if err != nil {
		return err
	}
	_, err = c.modelsPage(ctx, 1, "")
	return err
}

// Complete implements provider.Completer.
func (p *Provider) Complete(ctx context.Context, req types.Request) (*types.Response, error) {
	c, err := p.activeClient()
	if err != nil {
		return nil, err
	}

	apiReq, err := toAPIRequest(req, false)
	if err != nil {
		return nil, err
	}

	apiResp, err := c.messages(ctx, apiReq)
	if err != nil {
		return nil, err
	}

	resp := toResponse(apiResp)
	return &resp, nil
}

// activeClient returns the API client, or a typed error when the
// provider has not been activated.
func (p *Provider) activeClient() (*client, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.client == nil {
		return nil, &types.Error{
			Kind:     types.ErrorKindInternal,
			Provider: types.ProviderAnthropic,
			Message:  "provider is not activated",
		}
	}
	return p.client, nil
}
