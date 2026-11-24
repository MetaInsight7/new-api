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
import {
  ACCOUNT_PASSWORD_MAX_LENGTH,
  ACCOUNT_PASSWORD_MIN_LENGTH,
  countPasswordCharacters,
  isPredictableAccountPassword,
} from '@/lib/password-policy'

const LOWERCASE_PATTERN = /\p{Ll}/u
const UPPERCASE_PATTERN = /\p{Lu}/u
const DIGIT_PATTERN = /\p{N}/u
const SYMBOL_PATTERN = /[\p{P}\p{S}]/u

// Length and whole-password predictability decide whether a password is
// accepted. Character-mix rules remain strength hints and never block one.
export const PASSWORD_RECOMMENDED_LENGTH = 15
const PASSWORD_GOOD_LENGTH = 12
const PASSPHRASE_LENGTH = 20

export type PasswordStrengthRuleId =
  | 'length'
  | 'long'
  | 'case'
  | 'digit'
  | 'symbol'
export type PasswordStrengthScore = 0 | 1 | 2 | 3 | 4
export type PasswordStrengthLabel =
  | 'Password strength empty'
  | 'Password strength weak'
  | 'Password strength fair'
  | 'Password strength good'
  | 'Password strength strong'
export type PasswordConfirmationState = 'empty' | 'match' | 'mismatch'
export type PasswordLengthState = 'empty' | 'too-short' | 'valid' | 'too-long'

type EvaluatedPasswordRule = {
  id: PasswordStrengthRuleId
  met: boolean
}

export type PasswordStrengthResult = {
  score: PasswordStrengthScore
  labelKey: PasswordStrengthLabel
  guessable: boolean
  meetsRequirements: boolean
  characterCount: number
  lengthState: PasswordLengthState
  rules: EvaluatedPasswordRule[]
}

const STRENGTH_LABELS: Record<PasswordStrengthScore, PasswordStrengthLabel> = {
  0: 'Password strength empty',
  1: 'Password strength weak',
  2: 'Password strength fair',
  3: 'Password strength good',
  4: 'Password strength strong',
}

// Length decides most of the score (OWASP ASVS 5.0 6.2.1 recommends 15+
// characters); mixing character kinds only helps shorter passwords.
function getPasswordScore(
  characterCount: number,
  acceptable: boolean,
  variety: number
): PasswordStrengthScore {
  if (characterCount === 0) return 0
  if (!acceptable) return 1
  if (characterCount < PASSWORD_GOOD_LENGTH) return variety >= 2 ? 2 : 1
  if (characterCount < PASSWORD_RECOMMENDED_LENGTH) {
    return variety >= 3 ? 3 : 2
  }
  return characterCount >= PASSPHRASE_LENGTH || variety >= 2 ? 4 : 3
}

export function evaluatePasswordStrength(
  password: string
): PasswordStrengthResult {
  const characterCount = countPasswordCharacters(password)
  let lengthState: PasswordLengthState = 'valid'
  if (characterCount === 0) {
    lengthState = 'empty'
  } else if (characterCount < ACCOUNT_PASSWORD_MIN_LENGTH) {
    lengthState = 'too-short'
  } else if (characterCount > ACCOUNT_PASSWORD_MAX_LENGTH) {
    lengthState = 'too-long'
  }
  const meetsLengthRequirement = lengthState === 'valid'
  const hasLowercase = LOWERCASE_PATTERN.test(password)
  const hasUppercase = UPPERCASE_PATTERN.test(password)
  const hasDigit = DIGIT_PATTERN.test(password)
  const hasSymbol = SYMBOL_PATTERN.test(password)
  const variety = [hasLowercase, hasUppercase, hasDigit, hasSymbol].filter(
    Boolean
  ).length
  const guessable = isPredictableAccountPassword(password)
  const meetsRequirements = meetsLengthRequirement && !guessable
  const score = getPasswordScore(
    characterCount,
    meetsRequirements && !guessable,
    variety
  )

  return {
    score,
    labelKey: STRENGTH_LABELS[score],
    guessable,
    meetsRequirements,
    characterCount,
    lengthState,
    rules: [
      { id: 'length', met: meetsLengthRequirement },
      { id: 'long', met: characterCount >= PASSWORD_RECOMMENDED_LENGTH },
      { id: 'case', met: hasLowercase && hasUppercase },
      { id: 'digit', met: hasDigit },
      { id: 'symbol', met: hasSymbol },
    ],
  }
}

export function getPasswordConfirmationState(
  password: string,
  confirmation: string
): PasswordConfirmationState {
  if (!confirmation) return 'empty'
  return password === confirmation ? 'match' : 'mismatch'
}
