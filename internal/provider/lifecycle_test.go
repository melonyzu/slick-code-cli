package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/internal/provider"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// fakeProvider requires an API key and tracks lifecycle hook calls.
type fakeProvider struct {
	name types.Provider

	activatedWith *auth.Credential
	deactivated   bool
	healthErr     error
}

func (f *fakeProvider) Name() types.Provider                          { return f.name }
func (f *fakeProvider) Models(context.Context) ([]types.Model, error) { return nil, nil }
func (f *fakeProvider) AuthMethods() []auth.Method                    { return []auth.Method{auth.MethodAPIKey} }
func (f *fakeProvider) NewFlow(method auth.Method) (auth.Flow, error) { return nil, nil }
func (f *fakeProvider) Activate(_ context.Context, c auth.Credential) error {
	f.activatedWith = &c
	return nil
}
func (f *fakeProvider) Deactivate(context.Context) error  { f.deactivated = true; return nil }
func (f *fakeProvider) CheckHealth(context.Context) error { return f.healthErr }

// refreshableProvider adds auth.Refresher to fakeProvider.
type refreshableProvider struct{ fakeProvider }

func (r *refreshableProvider) Refresh(_ context.Context, current auth.Credential) (auth.Credential, error) {
	current.Secret = "renewed"
	current.ExpiresAt = time.Now().Add(time.Hour)
	return current, nil
}

// openProvider needs no authentication at all: it implements only the
// base Provider interface.
type openProvider struct{}

func (openProvider) Name() types.Provider                          { return types.ProviderOllama }
func (openProvider) Models(context.Context) ([]types.Model, error) { return nil, nil }

func newLifecycle(t *testing.T, providers ...provider.Provider) (*provider.Lifecycle, *auth.MemoryStore) {
	t.Helper()

	registry := provider.NewRegistry()
	for _, p := range providers {
		if err := registry.Register(p); err != nil {
			t.Fatal(err)
		}
	}

	store := auth.NewMemoryStore()
	return provider.NewLifecycle(registry, auth.NewManager(store, nil), nil), store
}

func TestActivateWithoutLoginFails(t *testing.T) {
	p := &fakeProvider{name: types.ProviderOpenAI}
	lc, _ := newLifecycle(t, p)

	_, err := lc.Activate(context.Background(), types.ProviderOpenAI)
	if types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("want authentication error, got %v", err)
	}
	if len(lc.Active()) != 0 {
		t.Fatal("failed activation must not mark the provider active")
	}
}

func TestActivatePassesCredentialAndTracksState(t *testing.T) {
	p := &fakeProvider{name: types.ProviderOpenAI}
	lc, store := newLifecycle(t, p)

	cred := auth.Credential{Provider: types.ProviderOpenAI, Method: auth.MethodAPIKey, Secret: "sk-live"}
	if err := store.Set(cred); err != nil {
		t.Fatal(err)
	}

	if _, err := lc.Activate(context.Background(), types.ProviderOpenAI); err != nil {
		t.Fatal(err)
	}
	if p.activatedWith == nil || p.activatedWith.Secret.Reveal() != "sk-live" {
		t.Fatal("provider was not activated with the stored credential")
	}
	if got := lc.Active(); len(got) != 1 || got[0] != types.ProviderOpenAI {
		t.Fatalf("active set = %v", got)
	}

	if err := lc.CheckHealth(context.Background(), types.ProviderOpenAI); err != nil {
		t.Fatalf("healthy provider reported: %v", err)
	}

	if err := lc.Deactivate(context.Background(), types.ProviderOpenAI); err != nil {
		t.Fatal(err)
	}
	if !p.deactivated {
		t.Fatal("Deactivate hook was not called")
	}
	if len(lc.Active()) != 0 {
		t.Fatal("deactivated provider still marked active")
	}
}

func TestActivateRefreshesExpiredSession(t *testing.T) {
	p := &refreshableProvider{fakeProvider{name: types.ProviderAnthropic}}
	lc, store := newLifecycle(t, p)

	expired := auth.Credential{
		Provider:  types.ProviderAnthropic,
		Method:    auth.MethodBrowserOAuth,
		Secret:    "stale",
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if err := store.Set(expired); err != nil {
		t.Fatal(err)
	}

	if _, err := lc.Activate(context.Background(), types.ProviderAnthropic); err != nil {
		t.Fatal(err)
	}
	if p.activatedWith.Secret.Reveal() != "renewed" {
		t.Fatal("expired session was not refreshed before activation")
	}

	stored, err := store.Get(types.ProviderAnthropic)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Secret.Reveal() != "renewed" {
		t.Fatal("refreshed credential was not persisted")
	}
}

func TestActivateExpiredWithoutRefresherFails(t *testing.T) {
	p := &fakeProvider{name: types.ProviderAnthropic}
	lc, store := newLifecycle(t, p)

	expired := auth.Credential{
		Provider:  types.ProviderAnthropic,
		Method:    auth.MethodBrowserOAuth,
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if err := store.Set(expired); err != nil {
		t.Fatal(err)
	}

	_, err := lc.Activate(context.Background(), types.ProviderAnthropic)
	if types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("want authentication error for expired session, got %v", err)
	}
}

func TestActivateProviderWithoutAuthenticator(t *testing.T) {
	lc, _ := newLifecycle(t, openProvider{})

	if _, err := lc.Activate(context.Background(), types.ProviderOllama); err != nil {
		t.Fatalf("provider without Authenticator must activate freely: %v", err)
	}
}

func TestDeactivateInactiveFails(t *testing.T) {
	lc, _ := newLifecycle(t, openProvider{})

	err := lc.Deactivate(context.Background(), types.ProviderOllama)
	if types.KindOf(err) != types.ErrorKindValidation {
		t.Fatalf("want validation error, got %v", err)
	}
}
