package core

import (
	"log/slog"
	"net/http"
	"slices"
	"testing"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestProvidersRegistersOpenAI(t *testing.T) {
	t.Setenv("SLICKCODE_OLLAMA_BASE_URL", "")
	t.Setenv("OLLAMA_HOST", "")
	registry, err := providerRegistry(http.DefaultClient, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	names := registry.List()
	if !slices.Contains(names, types.ProviderOpenAI) {
		t.Fatalf("registered providers = %v", names)
	}
	if !slices.Contains(names, types.ProviderOllama) {
		t.Fatalf("registered providers = %v", names)
	}
}
