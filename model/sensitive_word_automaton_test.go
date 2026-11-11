package model

import (
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

func TestSensitiveWordAutomatonFindsOverlappingAndUnicodeWords(t *testing.T) {
	automaton := newSensitiveWordAutomaton([]string{"he", "she", "hers", "敏感词"})
	type match struct {
		word       string
		start, end int
	}
	matches := make([]match, 0)
	text := []rune("ushers和敏感词")
	automaton.scan(text, func(word string, startRune, endRune int) bool {
		matches = append(matches, match{word: word, start: startRune, end: endRune})
		return true
	})

	require.ElementsMatch(t, []match{
		{word: "she", start: 1, end: 4},
		{word: "he", start: 2, end: 4},
		{word: "hers", start: 2, end: 6},
		{word: "敏感词", start: 7, end: 10},
	}, matches)
}

func TestSensitiveWordAutomatonStopsAtCallbackBoundary(t *testing.T) {
	automaton := newSensitiveWordAutomaton([]string{"observe", "block", "later"})
	visited := make([]string, 0)
	automaton.scan([]rune("observe block later"), func(word string, _, _ int) bool {
		visited = append(visited, word)
		return word != "block"
	})

	require.Equal(t, []string{"observe", "block"}, visited)
}

func TestSensitiveWordAutomatonMatchesCaseFoldedRunes(t *testing.T) {
	automaton := newSensitiveWordAutomaton([]string{"mixed"})
	visited := make([]string, 0)
	text := []rune("MiXeD")
	for index := range text {
		text[index] = unicode.ToLower(text[index])
	}
	automaton.scan(text, func(word string, _, _ int) bool {
		visited = append(visited, word)
		return true
	})

	require.Equal(t, []string{"mixed"}, visited)
}
