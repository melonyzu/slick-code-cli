package provider

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// The interfaces below are the optional operation contracts a provider
// may satisfy in addition to Provider. Callers discover support by type
// assertion — most conveniently via Capability — following the same
// pattern as net/http's optional interfaces (http.Pusher, http.Flusher).
// This keeps dispatch extensible: adding an operation means adding an
// interface, not extending a switch.

// Completer generates a complete response to a conversation in a single
// exchange.
type Completer interface {
	// Complete translates req into the provider's API, performs the
	// exchange, and translates the result back into the domain model.
	Complete(ctx context.Context, req types.Request) (*types.Response, error)
}

// Streamer generates a response incrementally.
type Streamer interface {
	// Stream begins a streamed exchange for req. The returned sequence
	// yields events in arrival order, ending with either a DoneEvent or
	// a non-nil error; when an event's error is non-nil the event is
	// invalid and the stream is over. Iteration must stop when ctx is
	// canceled.
	Stream(ctx context.Context, req types.Request) (iter.Seq2[types.StreamEvent, error], error)
}

// ModelValidator is implemented by providers that can confirm a configured
// model is available before the REPL starts.
type ModelValidator interface {
	// ValidateModel reports a classified error when model cannot be used. The
	// exact model string is supplied without provider-independent rewriting.
	ValidateModel(ctx context.Context, model string) error
}

// Embedder converts inputs into embedding vectors.
type Embedder interface {
	// Embed translates req into the provider's API and returns one
	// vector per input.
	Embed(ctx context.Context, req types.EmbedRequest) (*types.EmbedResponse, error)
}

// ImageGenerator generates images from text prompts.
type ImageGenerator interface {
	// GenerateImage translates req into the provider's API and returns
	// the generated images.
	GenerateImage(ctx context.Context, req types.ImageGenerationRequest) (*types.ImageGenerationResponse, error)
}

// Capability returns the provider registered under name if it implements
// the capability interface T, and a typed unsupported-capability error
// otherwise:
//
//	streamer, err := provider.Capability[provider.Streamer](registry, name)
func Capability[T any](r *Registry, name types.Provider) (T, error) {
	var zero T

	p, err := r.Get(name)
	if err != nil {
		return zero, err
	}

	t, ok := p.(T)
	if !ok {
		// %T on (*T)(nil) rather than a zero T: a zero interface
		// value would print "<nil>" instead of the interface name.
		capability := strings.TrimPrefix(fmt.Sprintf("%T", (*T)(nil)), "*")
		return zero, &types.Error{
			Kind:     types.ErrorKindUnsupportedCapability,
			Provider: name,
			Message:  "provider does not implement " + capability,
		}
	}
	return t, nil
}
