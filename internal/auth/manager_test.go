package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

type fakePrompter struct {
	secret string
	notes  []string
}

func (p *fakePrompter) PromptSecret(string) (string, error) { return p.secret, nil }
func (p *fakePrompter) Notify(message string)               { p.notes = append(p.notes, message) }

type apiKeyFlow struct{}

func (apiKeyFlow) Method() auth.Method { return auth.MethodAPIKey }
func (apiKeyFlow) Exchange(_ context.Context, key string) (auth.Credential, error) {
	return auth.Credential{Secret: auth.Secret(key)}, nil
}

type deviceFlow struct{}

func (deviceFlow) Method() auth.Method { return auth.MethodDeviceCode }
func (deviceFlow) Start(context.Context) (auth.DeviceAuthorization, error) {
	return auth.DeviceAuthorization{UserCode: "ABCD-1234", VerificationURL: "https://example.com/device"}, nil
}
func (deviceFlow) Wait(context.Context) (auth.Credential, error) {
	return auth.Credential{Secret: "token", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func newManager() (*auth.Manager, *auth.MemoryStore) {
	store := auth.NewMemoryStore()
	return auth.NewManager(store, nil), store
}

func TestLoginAPIKeyStoresCredential(t *testing.T) {
	mgr, _ := newManager()
	ui := &fakePrompter{secret: "sk-test"}

	cred, err := mgr.Login(context.Background(), types.ProviderOpenAI, apiKeyFlow{}, ui)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Provider != types.ProviderOpenAI || cred.Method != auth.MethodAPIKey {
		t.Fatalf("credential not normalized: %+v", cred)
	}

	sess, err := mgr.Session(context.Background(), types.ProviderOpenAI)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Credential.Secret.Reveal() != "sk-test" {
		t.Fatal("stored secret does not match prompted key")
	}
	if !sess.Valid(time.Now()) {
		t.Fatal("non-expiring session must be valid")
	}
}

func TestLoginDeviceCodeNotifiesUser(t *testing.T) {
	mgr, _ := newManager()
	ui := &fakePrompter{}

	if _, err := mgr.Login(context.Background(), types.ProviderGoogle, deviceFlow{}, ui); err != nil {
		t.Fatal(err)
	}
	if len(ui.notes) != 1 {
		t.Fatalf("want one device-code instruction, got %v", ui.notes)
	}
}

func TestLoginNoneStoresMarkerWithoutSecret(t *testing.T) {
	mgr, _ := newManager()

	cred, err := mgr.Login(context.Background(), types.ProviderOllama, auth.NoneFlow{}, &fakePrompter{})
	if err != nil {
		t.Fatal(err)
	}
	if cred.Secret != "" || cred.Method != auth.MethodNone {
		t.Fatalf("none login must store no secret: %+v", cred)
	}
}

func TestLogout(t *testing.T) {
	mgr, _ := newManager()

	if _, err := mgr.Login(context.Background(), types.ProviderOpenAI, apiKeyFlow{}, &fakePrompter{secret: "k"}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Logout(context.Background(), types.ProviderOpenAI); err != nil {
		t.Fatal(err)
	}

	err := mgr.Logout(context.Background(), types.ProviderOpenAI)
	if !errors.Is(err, auth.ErrNotFound) || types.KindOf(err) != types.ErrorKindAuthentication {
		t.Fatalf("second logout: want typed not-logged-in error, got %v", err)
	}
}

func TestSessionsDiscovery(t *testing.T) {
	mgr, _ := newManager()

	if _, err := mgr.Login(context.Background(), types.ProviderOpenAI, apiKeyFlow{}, &fakePrompter{secret: "k"}); err != nil {
		t.Fatal(err)
	}

	sessions, err := mgr.Sessions(context.Background(),
		[]types.Provider{types.ProviderAnthropic, types.ProviderOpenAI, types.ProviderGoogle})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Provider() != types.ProviderOpenAI {
		t.Fatalf("want only the openai session, got %v", sessions)
	}
}

type fakeRefresher struct{}

func (fakeRefresher) Refresh(_ context.Context, current auth.Credential) (auth.Credential, error) {
	current.Secret = "renewed"
	current.ExpiresAt = time.Now().Add(time.Hour)
	return current, nil
}

func TestRefreshPersistsRenewedCredential(t *testing.T) {
	mgr, store := newManager()

	expired := auth.Credential{
		Provider:  types.ProviderAnthropic,
		Method:    auth.MethodBrowserOAuth,
		Secret:    "stale",
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	if err := store.Set(expired); err != nil {
		t.Fatal(err)
	}

	cred, err := mgr.Refresh(context.Background(), types.ProviderAnthropic, fakeRefresher{})
	if err != nil {
		t.Fatal(err)
	}
	if cred.Secret.Reveal() != "renewed" {
		t.Fatal("refresh did not renew the secret")
	}

	stored, err := store.Get(types.ProviderAnthropic)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Secret.Reveal() != "renewed" {
		t.Fatal("renewed credential was not persisted")
	}
}
