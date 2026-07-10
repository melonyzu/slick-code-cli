// Package projectcontext builds a bounded, provider-independent project
// snapshot for assistant requests.
package projectcontext

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Estimator approximates model tokens without binding the context engine to a
// provider tokenizer.
type Estimator interface {
	Estimate(text string) int
}

// HeuristicEstimator estimates tokens from Unicode words, punctuation, and
// long identifier fragments. It is deterministic and intentionally
// conservative for source code.
type HeuristicEstimator struct{}

// Estimate implements Estimator.
func (HeuristicEstimator) Estimate(text string) int {
	if text == "" {
		return 0
	}
	words, punctuation, identifierExtra := 0, 0, 0
	inWord, wordRunes := false, 0
	flush := func() {
		if !inWord {
			return
		}
		words++
		if wordRunes > 8 {
			identifierExtra += (wordRunes - 1) / 8
		}
		inWord, wordRunes = false, 0
	}
	for _, r := range text {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_':
			inWord = true
			wordRunes++
		case unicode.IsSpace(r):
			flush()
		default:
			flush()
			punctuation++
		}
	}
	flush()

	// Non-ASCII text generally carries more tokenizer pieces than English.
	nonASCII := utf8.RuneCountInString(text) - len(strings.Map(func(r rune) rune {
		if r > unicode.MaxASCII {
			return -1
		}
		return r
	}, text))
	return max(1, words+identifierExtra+(punctuation+1)/2+(nonASCII+2)/3)
}
