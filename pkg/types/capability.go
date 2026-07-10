package types

// Capability identifies a feature a model supports. Capabilities are
// discovered per model via Model.Capabilities rather than assumed from
// the provider.
type Capability string

// Known capabilities.
const (
	CapabilityChat            Capability = "chat"
	CapabilityStreaming       Capability = "streaming"
	CapabilityTools           Capability = "tools"
	CapabilityVision          Capability = "vision"
	CapabilityReasoning       Capability = "reasoning"
	CapabilityEmbeddings      Capability = "embeddings"
	CapabilityImageGeneration Capability = "image_generation"
	CapabilityFileEditing     Capability = "file_editing"
)

// CapabilitySet is the set of capabilities a model supports.
type CapabilitySet map[Capability]bool

// NewCapabilitySet returns a CapabilitySet containing the given
// capabilities.
func NewCapabilitySet(caps ...Capability) CapabilitySet {
	set := make(CapabilitySet, len(caps))
	for _, c := range caps {
		set[c] = true
	}
	return set
}

// Has reports whether the set contains c.
func (s CapabilitySet) Has(c Capability) bool {
	return s[c]
}
