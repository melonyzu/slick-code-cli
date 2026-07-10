package types

// EmbedRequest is a provider-agnostic request to embed one or more
// inputs.
type EmbedRequest struct {
	// Model is the ID of the embedding model to use.
	Model string

	// Inputs is the list of texts to embed.
	Inputs []string
}

// Embedding is the vector for one input of an EmbedRequest.
type Embedding struct {
	// Index is the position of the input this vector corresponds to.
	Index int

	// Vector is the embedding itself.
	Vector []float64
}

// EmbedResponse carries the embeddings for an EmbedRequest.
type EmbedResponse struct {
	// Embeddings holds one vector per input, in input order.
	Embeddings []Embedding

	// Usage reports the request's token consumption.
	Usage Usage
}
