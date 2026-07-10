package ollama

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
	defaultEndpoint = "http://127.0.0.1:11434"
	endpointEnv     = "SLICKCODE_OLLAMA_BASE_URL"
	ollamaHostEnv   = "OLLAMA_HOST"
)

// Provider implements local Ollama discovery, chat, streaming, tools, model
// validation, and unauthenticated lifecycle contracts.
type Provider struct {
	client   *client
	logger   *slog.Logger
	endpoint string

	mu     sync.RWMutex
	active bool
	models map[string]types.CapabilitySet
}

// New returns an inactive Ollama provider using httpClient. Endpoint priority
// is SLICKCODE_OLLAMA_BASE_URL, OLLAMA_HOST, then the standard local endpoint.
func New(httpClient *http.Client, logger *slog.Logger) (*Provider, error) {
	if httpClient == nil {
		return nil, types.NewError(types.ErrorKindInvalidConfig, "ollama: HTTP client is required")
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	endpoint, err := discoverEndpoint()
	if err != nil {
		return nil, err
	}
	return &Provider{
		client: newClient(httpClient, endpoint, logger), logger: logger, endpoint: endpoint,
		models: make(map[string]types.CapabilitySet),
	}, nil
}

// Name implements provider.Provider.
func (*Provider) Name() types.Provider { return types.ProviderOllama }

// Endpoint returns the resolved Ollama API endpoint.
func (p *Provider) Endpoint() string { return p.endpoint }

// AuthMethods implements provider.Authenticator with local/no authentication.
func (*Provider) AuthMethods() []auth.Method { return []auth.Method{auth.MethodNone} }

// NewFlow implements provider.Authenticator without credentials or storage.
func (*Provider) NewFlow(method auth.Method) (auth.Flow, error) {
	if method != auth.MethodNone {
		return nil, &types.Error{
			Kind: types.ErrorKindUnsupportedCapability, Provider: types.ProviderOllama,
			Message: "ollama supports only local unauthenticated access",
		}
	}
	return auth.NoneFlow{}, nil
}

// Activate implements provider.Activator. It validates that no credential was
// supplied and confirms the local endpoint is reachable.
func (p *Provider) Activate(ctx context.Context, credential auth.Credential) error {
	if credential.Secret != "" ||
		(credential.Provider != "" && credential.Provider != types.ProviderOllama) ||
		(credential.Method != "" && credential.Method != auth.MethodNone) {
		return &types.Error{
			Kind: types.ErrorKindAuthentication, Provider: types.ProviderOllama,
			Message: "Ollama must use local/no authentication without credentials",
		}
	}
	if err := p.CheckHealth(ctx); err != nil {
		return err
	}
	p.mu.Lock()
	p.active = true
	p.mu.Unlock()
	p.logger.Info("ollama provider activated", "endpoint", p.endpoint)
	return nil
}

// Deactivate implements provider.Deactivator.
func (p *Provider) Deactivate(_ context.Context) error {
	p.mu.Lock()
	p.active = false
	p.mu.Unlock()
	p.logger.Debug("ollama provider deactivated")
	return nil
}

// CheckHealth implements provider.HealthChecker using installed-model listing.
func (p *Provider) CheckHealth(ctx context.Context) error {
	_, err := p.client.tags(ctx)
	return err
}

// Models implements provider.Provider using installed Ollama models and each
// model's locally reported capabilities.
func (p *Provider) Models(ctx context.Context) ([]types.Model, error) {
	if err := p.requireActive(); err != nil {
		return nil, err
	}
	models, err := p.client.models(ctx)
	if err != nil {
		return nil, err
	}
	p.rememberModels(models)
	return models, nil
}

// ValidateModel implements provider.ModelValidator using an exact installed
// model name. No aliases or name translation are applied.
func (p *Provider) ValidateModel(ctx context.Context, model string) error {
	if err := p.requireActive(); err != nil {
		return err
	}
	if err := validateModelName(model); err != nil {
		return err
	}
	discovered, err := p.client.model(ctx, model)
	if err != nil {
		return err
	}
	p.rememberModels([]types.Model{discovered})
	return nil
}

// Complete implements provider.Completer.
func (p *Provider) Complete(ctx context.Context, request types.Request) (*types.Response, error) {
	if err := p.requireActive(); err != nil {
		return nil, err
	}
	apiRequest, err := toAPIRequest(request, false)
	if err != nil {
		return nil, err
	}
	if supported, known := p.modelSupportsTools(request.Model); known && !supported {
		apiRequest.Tools = nil
	}
	response, err := p.client.chat(ctx, apiRequest)
	if err != nil {
		return nil, err
	}
	translated, err := toResponse(response)
	if err != nil {
		return nil, err
	}
	return &translated, nil
}

func (p *Provider) rememberModels(models []types.Model) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, model := range models {
		p.models[model.ID] = model.Capabilities
	}
}

func (p *Provider) modelSupportsTools(model string) (supported, known bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	capabilities, ok := p.models[model]
	return capabilities.Has(types.CapabilityTools), ok
}

func (p *Provider) requireActive() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.active {
		return &types.Error{
			Kind: types.ErrorKindValidation, Provider: types.ProviderOllama,
			Message: "Ollama provider is not activated",
		}
	}
	return nil
}

func discoverEndpoint() (string, error) {
	endpoint := strings.TrimSpace(os.Getenv(endpointEnv))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv(ollamaHostEnv))
		if endpoint != "" && !strings.Contains(endpoint, "://") {
			endpoint = "http://" + endpoint
		}
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", types.NewError(types.ErrorKindInvalidConfig,
			"ollama: endpoint must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	return strings.TrimRight(endpoint, "/"), nil
}

func validateModelName(model string) error {
	if model == "" || strings.TrimSpace(model) != model {
		return &types.Error{
			Kind: types.ErrorKindValidation, Provider: types.ProviderOllama,
			Message: "Ollama model must be a non-empty exact model name",
		}
	}
	for _, char := range model {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("._-/:", char) {
			continue
		}
		return &types.Error{
			Kind: types.ErrorKindValidation, Provider: types.ProviderOllama,
			Message: "Ollama model contains an unsupported character",
		}
	}
	return nil
}
