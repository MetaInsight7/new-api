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
import type { AuthUser } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'

type SelfResponse = {
  success?: boolean
  data?: AuthUser
}

type HomeRedirectResult = {
  to: '/dashboard' | '/workspace' | '/sign-in'
  user: AuthUser | null
}

export async function resolveHomeRedirect(
  getSelf: () => Promise<SelfResponse>
): Promise<HomeRedirectResult> {
  const response = await getSelf().catch(() => null)
  if (response?.success && response.data) {
    const to =
      Number(response.data.role ?? 0) >= ROLE.ADMIN
        ? '/dashboard'
        : '/workspace'
    return { to, user: response.data }
  }
  return { to: '/sign-in', user: null }
}
