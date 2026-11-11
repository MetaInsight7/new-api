/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type SensitiveWordTextMatch = { start: number; end: number }

export const SENSITIVE_WORD_MAX_CHARACTERS = 200
export const SENSITIVE_WORD_MAX_WORDS = 10_000

export type SensitiveWordDraftIssue =
  | { kind: 'word_too_long'; line: number; max: number }
  | { kind: 'too_many_words'; line: number; max: number }

export type SensitiveWordDraftAnalysis = {
  words: string[]
  issue: SensitiveWordDraftIssue | null
}

type NormalizedText = {
  value: string
  starts: number[]
  ends: number[]
}

// Textarea selections use UTF-16 offsets. Lowercasing one Unicode character
// can expand into multiple code units, so preserve its original selection
// range while constructing the searchable representation.
function normalizeWithOffsets(text: string): NormalizedText {
  let value = ''
  const starts: number[] = []
  const ends: number[] = []

  for (let offset = 0; offset < text.length;) {
    const codePoint = text.codePointAt(offset)
    if (codePoint === undefined) break

    const character = String.fromCodePoint(codePoint)
    const nextOffset = offset + character.length
    const normalized = character.toLowerCase()
    value += normalized
    for (let index = 0; index < normalized.length; index += 1) {
      starts.push(offset)
      ends.push(nextOffset)
    }
    offset = nextOffset
  }

  return { value, starts, ends }
}

export function findSensitiveWordMatches(
  text: string,
  query: string
): SensitiveWordTextMatch[] {
  const normalizedQuery = query.trim().toLowerCase()
  if (!normalizedQuery) return []

  const normalizedText = normalizeWithOffsets(text)
  const matches: SensitiveWordTextMatch[] = []
  let from = 0
  while (from < normalizedText.value.length) {
    const index = normalizedText.value.indexOf(normalizedQuery, from)
    if (index < 0) break
    const endIndex = index + normalizedQuery.length - 1
    const start = normalizedText.starts[index]
    const end = normalizedText.ends[endIndex]
    if (start === undefined || end === undefined) break
    matches.push({ start, end })
    from = endIndex + 1
  }
  return matches
}

export function analyzeSensitiveWordDraft(
  text: string
): SensitiveWordDraftAnalysis {
  const words: string[] = []
  const seen = new Set<string>()
  let issue: SensitiveWordDraftIssue | null = null
  const lines = text.split(/\r?\n/)
  for (let index = 0; index < lines.length; index += 1) {
    const raw = lines[index]
    const word = raw.trim()
    if (!word) continue
    if (issue === null && [...word].length > SENSITIVE_WORD_MAX_CHARACTERS) {
      issue = {
        kind: 'word_too_long',
        line: index + 1,
        max: SENSITIVE_WORD_MAX_CHARACTERS,
      }
    }
    const key = word.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    words.push(word)
    if (issue === null && words.length > SENSITIVE_WORD_MAX_WORDS) {
      issue = {
        kind: 'too_many_words',
        line: index + 1,
        max: SENSITIVE_WORD_MAX_WORDS,
      }
    }
  }
  return { words, issue }
}

export function parseSensitiveWordDraftWords(text: string): string[] {
  return analyzeSensitiveWordDraft(text).words
}

export function getNextSensitiveWordMatchIndex(
  currentIndex: number,
  matchCount: number,
  backwards = false
) {
  if (matchCount <= 0) return 0
  const index = ((currentIndex % matchCount) + matchCount) % matchCount
  return backwards
    ? (index - 1 + matchCount) % matchCount
    : (index + 1) % matchCount
}
