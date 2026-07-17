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

import { getAppSurface } from './surface.constants';

describe('getAppSurface', () => {
  test.each([
    ['/', 'public'],
    ['/pricing', 'public'],
    ['/console', 'product'],
    ['/console/log', 'product'],
    ['/console/token', 'product'],
    ['/console/support', 'product'],
    ['/console/channel', 'admin-semi'],
    ['/console/channel/12', 'admin-semi'],
    ['/console/models', 'admin-semi'],
    ['/console/deployment', 'admin-semi'],
    ['/console/subscription', 'admin-semi'],
    ['/console/redemption', 'admin-semi'],
    ['/console/user', 'admin-semi'],
    ['/console/setting', 'admin-semi'],
    ['/console/violation-audit', 'admin-semi'],
  ])('%s maps to %s', (pathname, expected) => {
    expect(getAppSurface(pathname)).toBe(expected);
  });

  test('does not treat similarly prefixed routes as admin pages', () => {
    expect(getAppSurface('/console/channel-health')).toBe('product');
    expect(getAppSurface('/console/user-guide')).toBe('product');
  });
});
