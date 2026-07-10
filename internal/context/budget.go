package projectcontext

// Budget tracks consumption against a fixed token ceiling.
type Budget struct {
	limit     int
	used      int
	estimator Estimator
}

// NewBudget returns a token budget. Non-positive limits create an exhausted
// budget rather than silently selecting a different policy.
func NewBudget(limit int, estimator Estimator) *Budget {
	if estimator == nil {
		estimator = HeuristicEstimator{}
	}
	return &Budget{limit: max(0, limit), estimator: estimator}
}

// Add consumes the estimated tokens for text if it fits.
func (b *Budget) Add(text string) bool {
	tokens := b.estimator.Estimate(text)
	if b.used+tokens > b.limit {
		return false
	}
	b.used += tokens
	return true
}

// Remaining returns the unconsumed token allowance.
func (b *Budget) Remaining() int {
	return max(0, b.limit-b.used)
}

// Used returns the consumed estimated tokens.
func (b *Budget) Used() int {
	return b.used
}

// Limit returns the configured token ceiling.
func (b *Budget) Limit() int {
	return b.limit
}
