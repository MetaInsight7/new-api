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

import { describe, expect, test } from 'bun:test';
import {
  headerModuleRequiresAuth,
  isHeaderModuleEnabled,
  normalizeHeaderNavModules,
} from './navigationConfig';

describe('header navigation config', () => {
  test('normalizes legacy booleans and preserves auth gates', () => {
    const modules = normalizeHeaderNavModules(
      JSON.stringify({ pricing: false, rankings: { enabled: true, requireAuth: true } }),
    );

    expect(isHeaderModuleEnabled(modules, 'pricing')).toBe(false);
    expect(isHeaderModuleEnabled(modules, 'rankings')).toBe(true);
    expect(headerModuleRequiresAuth(modules, 'rankings')).toBe(true);
  });

  test('falls back safely for malformed settings', () => {
    const modules = normalizeHeaderNavModules('{bad');
    expect(isHeaderModuleEnabled(modules, 'home')).toBe(true);
    expect(headerModuleRequiresAuth(modules, 'pricing')).toBe(false);
  });
});
