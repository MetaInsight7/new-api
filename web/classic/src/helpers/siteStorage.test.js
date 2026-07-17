/*
Copyright (C) 2025 QuantumNous

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

import { afterEach, describe, expect, test } from 'bun:test';
import { JSDOM } from 'jsdom';
import {
  getStoredValue,
  getStoredJSON,
  getStoredUser,
  getUserGroupFromLocalStorage,
  getUserIdFromLocalStorage,
  isAdmin,
  isRoot,
  setStoredValue,
  setStoredJSON,
} from './siteStorage';

const dom = new JSDOM('<!doctype html><html><body></body></html>', {
  url: 'https://example.test',
});
globalThis.localStorage = dom.window.localStorage;

afterEach(() => {
  localStorage.clear();
});

describe('site storage helpers', () => {
  test('returns normalized user information from valid storage', () => {
    localStorage.setItem(
      'user',
      JSON.stringify({ id: 42, group: 'pro', role: 100 }),
    );

    expect(getStoredUser()).toEqual({ id: 42, group: 'pro', role: 100 });
    expect(getUserIdFromLocalStorage()).toBe(42);
    expect(getUserGroupFromLocalStorage()).toBe('pro');
    expect(isAdmin()).toBe(true);
    expect(isRoot()).toBe(true);
  });

  test('treats malformed user storage as logged out', () => {
    localStorage.setItem('user', '{not-valid-json');

    expect(getStoredUser()).toBeNull();
    expect(getUserIdFromLocalStorage()).toBe(-1);
    expect(getUserGroupFromLocalStorage()).toBeUndefined();
    expect(isAdmin()).toBe(false);
    expect(isRoot()).toBe(false);
  });

  test('rejects valid JSON that is not a user object', () => {
    for (const value of ['true', '42', '"user"', '[]']) {
      localStorage.setItem('user', value);
      expect(getStoredUser()).toBeNull();
    }
  });

  test('provides safe storage wrappers for unavailable browser storage', () => {
    expect(setStoredValue('safe-key', 'value')).toBe(true);
    expect(getStoredValue('safe-key')).toBe('value');
  });

  test('uses fallbacks for malformed JSON cache values', () => {
    localStorage.setItem('broken-cache', '{not-valid-json');
    expect(getStoredJSON('broken-cache', { fallback: true })).toEqual({
      fallback: true,
    });
  });

  test('serializes JSON cache values consistently', () => {
    expect(setStoredJSON('json-cache', { enabled: true })).toBe(true);
    expect(getStoredJSON('json-cache')).toEqual({ enabled: true });
  });
});
