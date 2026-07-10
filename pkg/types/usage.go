package types

// Usage reports the token consumption of a single provider request.
type Usage struct {
	// InputTokens is the number of tokens in the prompt.
	InputTokens int

	// OutputTokens is the number of tokens generated in the response.
	OutputTokens int
}

// TotalTokens returns the combined input and output token count.
func (u Usage) TotalTokens() int {
	return u.InputTokens + u.OutputTokens
}
