package openai

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

const (
	defaultBaseURL = "https://api.openai.com"
	baseURLEnv     = "SLICKCODE_OPENAI_BASE_URL"
)

// Provider implements OpenAI chat, streaming, model discovery, tools, and
// API-key authentication through Slick Code's shared provider contracts.
type Provider struct {
	httpClient *http.Client
	logger     *slog.Logger
	baseURL    string

	mu     sync.RWMutex
	client *client
}

// New returns an inactive OpenAI provider using httpClient. It validates the
// optional SLICKCODE_OPENAI_BASE_URL override before returning.
func New(httpClient *http.Client, logger *slog.Logger) (*Provider, error) {
	if httpClient == nil {
		return nil, types.NewError(types.ErrorKindInvalidConfig, "openai: HTTP client is required")
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	baseURL := strings.TrimSpace(os.Getenv(baseURLEnv))
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL, err := validateBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &Provider{httpClient: httpClient, logger: logger, baseURL: baseURL}, nil
}

// Name implements provider.Provider.
func (*Provider) Name() types.Provider { return types.ProviderOpenAI }

// Models implements provider.Provider using OpenAI's model listing.
func (p *Provider) Models(ctx context.Context) ([]types.Model, error) {
	c, err := p.activeClient()
	if err != nil {
		return nil, err
	}
	return c.models(ctx)
}

// AuthMethods implements provider.Authenticator.
func (*Provider) AuthMethods() []auth.Method { return []auth.Method{auth.MethodAPIKey} }

// NewFlow implements provider.Authenticator.
func (*Provider) NewFlow(method auth.Method) (auth.Flow, error) {
	if method != auth.MethodAPIKey {
		return nil, &types.Error{
			Kind: types.ErrorKindUnsupportedCapability, Provider: types.ProviderOpenAI,
			Message: "openai supports only api_key authentication",
		}
	}
	return apiKeyFlow{}, nil
}

// Activate implements provider.Activator and validates the stored session
// credential before constructing an authenticated API client.
func (p *Provider) Activate(_ context.Context, credential auth.Credential) error {
	if credential.Provider != types.ProviderOpenAI || credential.Method != auth.MethodAPIKey || credential.Secret == "" {
		return &types.Error{
			Kind: types.ErrorKindAuthentication, Provider: types.ProviderOpenAI,
			Message: "stored OpenAI session is not a valid API-key credential",
		}
	}
	p.mu.Lock()
	p.client = newClient(p.httpClient, p.baseURL, credential.Secret, p.logger)
	p.mu.Unlock()
	p.logger.Debug("openai provider activated")
	return nil
}

// Deactivate implements provider.Deactivator, dropping the in-memory client
// and its credential.
func (p *Provider) Deactivate(_ context.Context) error {
	p.mu.Lock()
	p.client = nil
	p.mu.Unlock()
	p.logger.Debug("openai provider deactivated")
	return nil
}

// CheckHealth implements provider.HealthChecker using model discovery.
func (p *Provider) CheckHealth(ctx context.Context) error {
	c, err := p.activeClient()
	if err != nil {
		return err
	}
	_, err = c.models(ctx)
	return err
}

// Complete implements provider.Completer.
func (p *Provider) Complete(ctx context.Context, request types.Request) (*types.Response, error) {
	c, err := p.activeClient()
	if err != nil {
		return nil, err
	}
	apiRequest, err := toAPIRequest(request, false)
	if err != nil {
		return nil, err
	}
	response, err := c.chat(ctx, apiRequest)
	if err != nil {
		return nil, err
	}
	translated, err := toResponse(response)
	if err != nil {
		return nil, err
	}
	return &translated, nil
}

func (p *Provider) activeClient() (*client, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.client == nil {
		return nil, &types.Error{
			Kind: types.ErrorKindAuthentication, Provider: types.ProviderOpenAI,
			Message: "OpenAI provider is not activated",
		}
	}
	return p.client, nil
}

func validateBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", types.NewError(types.ErrorKindInvalidConfig,
			"openai: SLICKCODE_OPENAI_BASE_URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	return strings.TrimRight(raw, "/"), nil
}
