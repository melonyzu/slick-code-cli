package auth_test

import (
	"errors"
	"testing"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// brokenStore simulates an unavailable OS keyring.
type brokenStore struct{}

var errKeyringDown = errors.New("secret service unavailable")

func (brokenStore) Get(types.Provider) (auth.Credential, error) {
	return auth.Credential{}, auth.ErrNotFound
}
func (brokenStore) Set(auth.Credential) error   { return errKeyringDown }
func (brokenStore) Delete(types.Provider) error { return auth.ErrNotFound }

func TestEnvStoreGet(t *testing.T) {
	t.Setenv("SLICKCODE_ANTHROPIC_API_KEY", "sk-from-env")

	cred, err := auth.NewEnvStore().Get(types.ProviderAnthropic)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Secret.Reveal() != "sk-from-env" || cred.Method != auth.MethodAPIKey {
		t.Fatalf("unexpected credential: %+v", cred)
	}

	_, err = auth.NewEnvStore().Get(types.ProviderOpenAI)
	if !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("unset variable must be ErrNotFound, got %v", err)
	}
}

func TestEnvStoreDeleteExplainsItself(t *testing.T) {
	t.Setenv("SLICKCODE_ANTHROPIC_API_KEY", "sk-from-env")

	err := auth.NewEnvStore().Delete(types.ProviderAnthropic)
	if !errors.Is(err, auth.ErrReadOnly) {
		t.Fatalf("want ErrReadOnly in chain, got %v", err)
	}
}

func TestLayeredReadPrecedence(t *testing.T) {
	t.Setenv("SLICKCODE_ANTHROPIC_API_KEY", "sk-from-env")

	mem := auth.NewMemoryStore()
	if err := mem.Set(auth.Credential{Provider: types.ProviderAnthropic, Secret: "sk-from-mem"}); err != nil {
		t.Fatal(err)
	}

	layered := auth.NewLayered(nil, auth.NewEnvStore(), mem)
	cred, err := layered.Get(types.ProviderAnthropic)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Secret.Reveal() != "sk-from-env" {
		t.Fatal("earlier layer must win reads")
	}
}

func TestLayeredSetFallsThroughBrokenLayer(t *testing.T) {
	mem := auth.NewMemoryStore()
	layered := auth.NewLayered(nil, auth.NewEnvStore(), brokenStore{}, mem)

	cred := auth.Credential{Provider: types.ProviderAnthropic, Method: auth.MethodAPIKey, Secret: "sk-live"}
	if err := layered.Set(cred); err != nil {
		t.Fatalf("set must fall through to the memory layer: %v", err)
	}

	got, err := layered.Get(types.ProviderAnthropic)
	if err != nil {
		t.Fatal(err)
	}
	if got.Secret.Reveal() != "sk-live" {
		t.Fatal("credential not stored in fallback layer")
	}
}

func TestLayeredSetAllLayersFail(t *testing.T) {
	layered := auth.NewLayered(nil, auth.NewEnvStore(), brokenStore{})

	err := layered.Set(auth.Credential{Provider: types.ProviderAnthropic})
	if !errors.Is(err, errKeyringDown) {
		t.Fatalf("want last layer's error, got %v", err)
	}
}

func TestLayeredDelete(t *testing.T) {
	mem := auth.NewMemoryStore()
	if err := mem.Set(auth.Credential{Provider: types.ProviderAnthropic}); err != nil {
		t.Fatal(err)
	}

	layered := auth.NewLayered(nil, auth.NewEnvStore(), mem)
	if err := layered.Delete(types.ProviderAnthropic); err != nil {
		t.Fatal(err)
	}
	if err := layered.Delete(types.ProviderAnthropic); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("second delete must be ErrNotFound, got %v", err)
	}
}
