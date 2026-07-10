package anthropic

import (
	"context"
	"strings"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// apiKeyFlow wraps a user-supplied Anthropic API key in a credential.
// The key is verified against the API on first use, not here, so login
// works offline and never sends the key anywhere except Anthropic.
type apiKeyFlow struct{}

// Method implements auth.Flow.
func (apiKeyFlow) Method() auth.Method { return auth.MethodAPIKey }

// Exchange implements auth.APIKeyFlow.
func (apiKeyFlow) Exchange(_ context.Context, key string) (auth.Credential, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return auth.Credential{}, &types.Error{
			Kind:     types.ErrorKindValidation,
			Provider: types.ProviderAnthropic,
			Message:  "API key is empty",
		}
	}

	return auth.Credential{
		Provider: types.ProviderAnthropic,
		Method:   auth.MethodAPIKey,
		Secret:   auth.Secret(key),
	}, nil
}
