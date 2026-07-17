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

import { hasRoleAtLeast } from './auth';

describe('auth role helpers', () => {
  test('accepts numeric and numeric-string roles', () => {
    expect(hasRoleAtLeast({ role: 100 }, 100)).toBe(true);
    expect(hasRoleAtLeast({ role: '100' }, 100)).toBe(true);
    expect(hasRoleAtLeast({ role: '10' }, 100)).toBe(false);
  });

  test('rejects missing and malformed roles', () => {
    expect(hasRoleAtLeast(null, 10)).toBe(false);
    expect(hasRoleAtLeast({ role: 'admin' }, 10)).toBe(false);
    expect(hasRoleAtLeast({ role: Infinity }, 10)).toBe(false);
  });
});
