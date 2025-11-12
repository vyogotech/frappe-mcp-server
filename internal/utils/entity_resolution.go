package utils

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// FuzzyMatch returns true if two strings are similar enough (Levenshtein distance, substring, or token set)
func FuzzyMatch(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == b {
		return true
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	// Token set match (ignoring order)
	tokensA := strings.Fields(a)
	tokensB := strings.Fields(b)
	if len(tokensA) > 1 && len(tokensB) > 1 {
		setA := make(map[string]struct{})
		setB := make(map[string]struct{})
		for _, t := range tokensA {
			setA[t] = struct{}{}
		}
		for _, t := range tokensB {
			setB[t] = struct{}{}
		}
		matchCount := 0
		for t := range setA {
			if _, ok := setB[t]; ok {
				matchCount++
			}
		}
		if matchCount == len(setA) || matchCount == len(setB) {
			return true
		}
	}
	// For short words, allow 1 typo or 1 missing/extra character
	runesA := []rune(a)
	runesB := []rune(b)
	if len(runesA) <= 5 && len(runesB) <= 5 {
		if LevenshteinDistanceRunes(runesA, runesB) <= 1 {
			return true
		}
		// Special case: common vowel swaps/transpositions
		if isVowelSwapOrTransposition(runesA, runesB) {
			return true
		}
		// Subsequence fallback for short words
		if isSubsequence(runesA, runesB) || isSubsequence(runesB, runesA) {
			return true
		}
	}
	// Levenshtein distance threshold: allow 1 typo per 5 chars
	maxDist := (utf8.RuneCountInString(a) + utf8.RuneCountInString(b)) / 10
	if maxDist < 1 {
		maxDist = 1
	}
	if LevenshteinDistanceRunes(runesA, runesB) <= maxDist {
		return true
	}
	// Try fuzzy match for each word
	for _, wa := range tokensA {
		for _, wb := range tokensB {
			rA := []rune(wa)
			rB := []rune(wb)
			if len(rA) <= 5 && len(rB) <= 5 && LevenshteinDistanceRunes(rA, rB) <= 1 {
				return true
			}
			if isVowelSwapOrTransposition(rA, rB) {
				return true
			}
			if LevenshteinDistanceRunes(rA, rB) <= maxDist {
				return true
			}
		}
	}
	return false
}

// isVowelSwapOrTransposition returns true if two short words differ by a common vowel swap or transposition
func isVowelSwapOrTransposition(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	vowels := "aeiou"
	diffs := 0
	for i := range a {
		if a[i] != b[i] {
			if strings.ContainsRune(vowels, a[i]) && strings.ContainsRune(vowels, b[i]) {
				diffs++
			} else {
				return false
			}
		}
	}
	return diffs == 1
}

// isSubsequence returns true if a is a subsequence of b
func isSubsequence(a, b []rune) bool {
	j := 0
	for i := 0; i < len(b) && j < len(a); i++ {
		if a[j] == b[i] {
			j++
		}
	}
	return j == len(a)
}

// LevenshteinDistance computes the edit distance between two strings (byte-based, for compatibility)
func LevenshteinDistance(a, b string) int {
	return LevenshteinDistanceRunes([]rune(a), []rune(b))
}

// LevenshteinDistanceRunes computes the edit distance between two rune slices
func LevenshteinDistanceRunes(a, b []rune) int {
	da := make([][]int, len(a)+1)
	for i := range da {
		da[i] = make([]int, len(b)+1)
	}
	for i := 0; i <= len(a); i++ {
		da[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		da[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			da[i][j] = min(
				da[i-1][j]+1,
				da[i][j-1]+1,
				da[i-1][j-1]+cost,
			)
		}
	}
	return da[len(a)][len(b)]
}

func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

// ResolveEntity attempts to resolve a user-friendly name to an internal ID from a list of candidates
func ResolveEntity(userInput string, candidates []string) (string, bool) {
	for _, candidate := range candidates {
		if FuzzyMatch(userInput, candidate) {
			// Debug log: print to stderr if DEBUG or RAW env is set
			if os.Getenv("DEBUG") == "1" || os.Getenv("RAW") == "1" {
				fmt.Fprintf(os.Stderr, "[DEBUG] ResolveEntity: matched '%s' to candidate '%s'\n", userInput, candidate)
			}
			return candidate, true
		}
	}
	if os.Getenv("DEBUG") == "1" || os.Getenv("RAW") == "1" {
		fmt.Fprintf(os.Stderr, "[DEBUG] ResolveEntity: no match for '%s' in candidates: %v\n", userInput, candidates)
	}
	return "", false
}
