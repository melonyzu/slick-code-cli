package auth_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

func TestSecretNeverLeaksThroughFormatting(t *testing.T) {
	const raw = "sk-super-secret-value"

	cred := auth.Credential{
		Provider:     types.ProviderAnthropic,
		Method:       auth.MethodAPIKey,
		Secret:       auth.Secret(raw),
		RefreshToken: auth.Secret(raw),
	}

	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		rendered := fmt.Sprintf(format, cred)
		if strings.Contains(rendered, raw) {
			t.Errorf("format %s leaked the secret: %s", format, rendered)
		}
	}

	if cred.Secret.Reveal() != raw {
		t.Error("Reveal must return the actual value")
	}
}

func TestSecretNeverLeaksThroughLogs(t *testing.T) {
	const raw = "sk-super-secret-value"

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	cred := auth.Credential{Secret: auth.Secret(raw)}
	logger.Info("login", "credential", cred, "secret", cred.Secret)

	if strings.Contains(buf.String(), raw) {
		t.Errorf("slog output leaked the secret: %s", buf.String())
	}
}

func TestCredentialExpired(t *testing.T) {
	now := time.Now()

	if (auth.Credential{}).Expired(now) {
		t.Error("credential without expiry must never expire")
	}
	if (auth.Credential{ExpiresAt: now.Add(time.Hour)}).Expired(now) {
		t.Error("future expiry must not be expired")
	}
	if !(auth.Credential{ExpiresAt: now.Add(-time.Hour)}).Expired(now) {
		t.Error("past expiry must be expired")
	}
}
