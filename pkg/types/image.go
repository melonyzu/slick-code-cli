package types

// ImageGenerationRequest is a provider-agnostic request to generate
// images from a text prompt.
type ImageGenerationRequest struct {
	// Model is the ID of the image model to use.
	Model string

	// Prompt describes the image to generate.
	Prompt string

	// Count is the number of images to generate. Zero means one.
	Count int
}

// ImageGenerationResponse carries the generated images.
type ImageGenerationResponse struct {
	// Images holds the generated images.
	Images []ImageRef

	// Usage reports the request's token consumption, where the
	// provider reports it.
	Usage Usage
}
