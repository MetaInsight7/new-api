package model

import "unicode"

type sensitiveWordAutomatonOutput struct {
	word       string
	runeLength int
}

type sensitiveWordAutomatonNode struct {
	transitions map[rune]int
	failure     int
	outputs     []sensitiveWordAutomatonOutput
}

type sensitiveWordAutomaton struct {
	nodes []sensitiveWordAutomatonNode
}

func newSensitiveWordAutomaton(words []string) *sensitiveWordAutomaton {
	automaton := &sensitiveWordAutomaton{
		nodes: []sensitiveWordAutomatonNode{{transitions: make(map[rune]int)}},
	}
	for _, word := range words {
		wordRunes := []rune(word)
		state := 0
		for _, character := range wordRunes {
			nextState, exists := automaton.nodes[state].transitions[character]
			if !exists {
				nextState = len(automaton.nodes)
				automaton.nodes[state].transitions[character] = nextState
				automaton.nodes = append(automaton.nodes, sensitiveWordAutomatonNode{
					transitions: make(map[rune]int),
				})
			}
			state = nextState
		}
		automaton.nodes[state].outputs = append(
			automaton.nodes[state].outputs,
			sensitiveWordAutomatonOutput{word: word, runeLength: len(wordRunes)},
		)
	}

	queue := make([]int, 0, len(automaton.nodes))
	for _, state := range automaton.nodes[0].transitions {
		queue = append(queue, state)
	}
	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]
		for character, nextState := range automaton.nodes[state].transitions {
			queue = append(queue, nextState)
			failureState := automaton.nodes[state].failure
			for failureState != 0 {
				if _, exists := automaton.nodes[failureState].transitions[character]; exists {
					break
				}
				failureState = automaton.nodes[failureState].failure
			}
			if fallbackState, exists := automaton.nodes[failureState].transitions[character]; exists {
				automaton.nodes[nextState].failure = fallbackState
			}
			fallbackOutputs := automaton.nodes[automaton.nodes[nextState].failure].outputs
			automaton.nodes[nextState].outputs = append(automaton.nodes[nextState].outputs, fallbackOutputs...)
		}
	}
	return automaton
}

// scan visits matches in text order. Returning false from visit stops the
// automaton before it reads any remaining input runes.
func (automaton *sensitiveWordAutomaton) scan(text []rune, visit func(word string, startRune, endRune int) bool) {
	if automaton == nil || len(automaton.nodes) == 0 {
		return
	}
	state := 0
	for position, character := range text {
		character = unicode.ToLower(character)
		for {
			nextState, exists := automaton.nodes[state].transitions[character]
			if exists {
				state = nextState
				break
			}
			if state == 0 {
				break
			}
			state = automaton.nodes[state].failure
		}
		for _, output := range automaton.nodes[state].outputs {
			startRune := position - output.runeLength + 1
			if !visit(output.word, startRune, position+1) {
				return
			}
		}
	}
}
