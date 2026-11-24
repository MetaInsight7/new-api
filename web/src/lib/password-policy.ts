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
import { z } from 'zod'

export const ACCOUNT_PASSWORD_MIN_LENGTH = 8
export const ACCOUNT_PASSWORD_MAX_LENGTH = 128
export const ACCOUNT_PASSWORD_LENGTH_ERROR =
  'Password must contain between 8 and 128 characters.'
export const ACCOUNT_PASSWORD_COMMON_ERROR =
  'Password is too common. Choose a less predictable password.'

const COMMON_ACCOUNT_PASSWORDS = [
  'password',
  'passw0rd',
  'qwerty',
  'letmein',
  'welcome',
  'admin',
  'iloveyou',
  'monkey',
  'dragon',
  'abc123',
  '111111',
  '123123',
  '123456',
]
const COMMON_PASSWORD_SUFFIX_PATTERN = /^[\p{N}\p{P}\p{S}\s]*$/u
const STRAIGHT_RUNS = [
  '0123456789',
  'abcdefghijklmnopqrstuvwxyz',
  'qwertyuiop',
  'asdfghjkl',
  'zxcvbnm',
]

// Match the backend's Unicode character count without modifying the password.
export function countPasswordCharacters(password: string): number {
  return [...password].length
}

export function isPredictableAccountPassword(password: string): boolean {
  const lowercase = password.toLowerCase()
  const isCommon = COMMON_ACCOUNT_PASSWORDS.some(
    (common) =>
      lowercase.startsWith(common) &&
      COMMON_PASSWORD_SUFFIX_PATTERN.test(lowercase.slice(common.length))
  )
  if (isCommon) return true

  const characters = [...lowercase]
  if (characters.length < 4) return false
  if (characters.every((character) => character === characters[0])) {
    return true
  }

  for (const run of STRAIGHT_RUNS) {
    const positions = characters.map((character) => run.indexOf(character))
    if (positions.includes(-1)) continue
    for (const step of [1, run.length - 1]) {
      const straight = positions.every(
        (position, index) =>
          index === 0 || position === (positions[index - 1] + step) % run.length
      )
      if (straight) return true
    }
  }
  return false
}

export function getAccountPasswordPolicyError(password: string) {
  const length = countPasswordCharacters(password)
  if (
    length < ACCOUNT_PASSWORD_MIN_LENGTH ||
    length > ACCOUNT_PASSWORD_MAX_LENGTH
  ) {
    return ACCOUNT_PASSWORD_LENGTH_ERROR
  }
  if (isPredictableAccountPassword(password)) {
    return ACCOUNT_PASSWORD_COMMON_ERROR
  }
  return null
}

export const accountPasswordSchema = z.string().superRefine((password, ctx) => {
  const message = getAccountPasswordPolicyError(password)
  if (message) {
    ctx.addIssue({ code: 'custom', message })
  }
})
