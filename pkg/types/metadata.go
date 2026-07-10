package types

// Metadata carries optional, provider-agnostic key/value annotations on
// domain values. Keys and values are plain strings so the domain model
// stays serializable and free of provider-specific structures.
type Metadata map[string]string
