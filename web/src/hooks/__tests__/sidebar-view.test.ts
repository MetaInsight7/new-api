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
*/
import { describe, expect, it } from 'vitest'

import { ROLE } from '@/lib/roles'

import { filterSidebarGroupsByRole } from '../use-sidebar-view'

const navigation = [
  {
    id: 'general',
    title: 'General',
    items: [
      { title: 'Workspace', url: '/workspace' },
      { title: 'Available Models', url: '/available-models' },
      {
        title: 'Overview',
        url: '/dashboard/overview',
        requiredRole: ROLE.ADMIN,
      },
      {
        title: 'Dashboard',
        url: '/dashboard/models',
        requiredRole: ROLE.ADMIN,
      },
    ],
  },
  {
    id: 'admin',
    title: 'Admin',
    items: [{ title: 'Channels', url: '/channels' }],
  },
]

describe('root sidebar role visibility', () => {
  it('hides administrative dashboards and groups from regular users', () => {
    const groups = filterSidebarGroupsByRole(navigation, ROLE.USER)
    const items = groups.flatMap((group) => group.items)

    expect(groups.map((group) => group.id)).toEqual(['general'])
    expect(items.map((item) => item.title)).toEqual([
      'Workspace',
      'Available Models',
    ])
  })

  it('keeps dashboards and the admin group for administrators', () => {
    const groups = filterSidebarGroupsByRole(navigation, ROLE.ADMIN)
    const items = groups.flatMap((group) => group.items)

    expect(groups.map((group) => group.id)).toEqual(['general', 'admin'])
    expect(items.map((item) => item.title)).toEqual([
      'Workspace',
      'Available Models',
      'Overview',
      'Dashboard',
      'Channels',
    ])
  })
})
