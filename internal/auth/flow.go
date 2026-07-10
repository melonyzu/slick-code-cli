package auth

import "context"

// Flow is one attempt to authenticate with a provider using a specific
// method. A provider constructs a flow for each login; the Manager
// drives it. Every flow implements exactly one of the method-specific
// contracts below — APIKeyFlow, BrowserFlow, DeviceCodeFlow, or
// NoneFlow — which is how the Manager knows what choreography the flow
// needs. Adding a new authentication strategy means adding a new flow
// contract here and its choreography in Manager.Login; providers and the
// runtime are untouched.
type Flow interface {
	// Method returns the authentication method this flow performs.
	Method() Method
}

// APIKeyFlow exchanges a user-supplied API key for a credential.
type APIKeyFlow interface {
	Flow

	// Exchange validates the key's shape and wraps it in a Credential.
	// It must not perform network requests; validity against the
	// provider's API is established on first use.
	Exchange(ctx context.Context, key string) (Credential, error)
}

// BrowserFlow authenticates by having the user complete an OAuth
// consent flow in their browser.
type BrowserFlow interface {
	Flow

	// Start prepares the flow and returns the URL the user must open.
	Start(ctx context.Context) (url string, err error)

	// Wait blocks until the user completes the flow, then returns the
	// resulting credential. It must honor ctx cancellation.
	Wait(ctx context.Context) (Credential, error)
}

// DeviceCodeFlow authenticates with the OAuth device authorization
// grant: the user enters a short code on a verification page.
type DeviceCodeFlow interface {
	Flow

	// Start requests a device authorization and returns the details
	// the user needs to complete it.
	Start(ctx context.Context) (DeviceAuthorization, error)

	// Wait polls until the user approves the authorization, then
	// returns the resulting credential. It must honor ctx cancellation.
	Wait(ctx context.Context) (Credential, error)
}

// DeviceAuthorization is what the user needs to complete a device code
// flow.
type DeviceAuthorization struct {
	// UserCode is the short code the user enters on the verification
	// page.
	UserCode string

	// VerificationURL is the page where the user enters the code.
	VerificationURL string
}

// NoneFlow is the Flow for MethodNone: logging in records that the
// provider is used unauthenticated and stores no secret material.
type NoneFlow struct{}

// Method implements Flow.
func (NoneFlow) Method() Method { return MethodNone }

// Refresher renews an expired credential without user interaction.
// Providers whose credentials expire implement it alongside their other
// contracts; the Manager and provider lifecycle discover it by type
// assertion.
type Refresher interface {
	// Refresh exchanges the current credential (typically its refresh
	// token) for a renewed one.
	Refresh(ctx context.Context, current Credential) (Credential, error)
}

// Prompter supplies the user interaction flows need during login. It is
// implemented by internal/terminal.
type Prompter interface {
	// PromptSecret asks the user for a secret value, without echoing
	// it where the input device allows.
	PromptSecret(label string) (string, error)

	// Notify displays an instruction to the user, such as a URL to
	// open or a device code to enter.
	Notify(message string)
}
