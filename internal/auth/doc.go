// Package auth is Slick Code's provider-independent authentication
// framework. It defines the supported authentication methods, the flow
// contracts providers implement for each method, secure credential
// storage, and the Manager that drives login, logout, session
// discovery, validation, and refresh.
//
// The package knows how to drive each authentication *kind* (API key,
// browser OAuth, device code, none) but nothing about any specific
// provider: providers supply flows, this package supplies the
// choreography, so the login experience is identical across providers.
package auth
