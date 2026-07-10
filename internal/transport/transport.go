// Package transport provides the HTTP client shared by provider
// implementations. It centralizes cross-cutting concerns such as
// timeouts and the User-Agent header; it has no knowledge of any
// specific provider's API.
package transport

import (
	"net/http"
	"time"

	"github.com/melonyzu/slick-code-cli/pkg/version"
)

// DefaultTimeout is the request timeout applied when a client is created via
// NewClient.
const DefaultTimeout = 60 * time.Second

// NewClient returns an *http.Client configured with Slick Code's default
// timeout and a transport that identifies the CLI to providers via the
// User-Agent header.
func NewClient() *http.Client {
	return &http.Client{
		Timeout:   DefaultTimeout,
		Transport: &userAgentTransport{base: http.DefaultTransport},
	}
}

type userAgentTransport struct {
	base http.RoundTripper
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", "slickcode/"+version.Version)
	return t.base.RoundTrip(req)
}
