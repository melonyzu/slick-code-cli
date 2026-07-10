package openai

import (
	"context"
	"strings"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

type apiKeyFlow struct{}

func (apiKeyFlow) Method() auth.Method { return auth.MethodAPIKey }

func (apiKeyFlow) Exchange(_ context.Context, key string) (auth.Credential, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return auth.Credential{}, &types.Error{
			Kind: types.ErrorKindValidation, Provider: types.ProviderOpenAI,
			Message: "API key is empty",
		}
	}
	return auth.Credential{
		Provider: types.ProviderOpenAI,
		Method:   auth.MethodAPIKey,
		Secret:   auth.Secret(key),
	}, nil
}
