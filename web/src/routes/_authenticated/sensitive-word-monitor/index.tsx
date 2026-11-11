/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { createFileRoute, redirect } from '@tanstack/react-router'

import { SensitiveWordMonitor } from '@/features/sensitive-word-monitor'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import { ROLE } from '@/lib/roles'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

export function canAccessSensitiveWordMonitor(
  user: AuthUser | null | undefined
): boolean {
  return Boolean(
    user &&
    user.role >= ROLE.ADMIN &&
    hasPermission(
      user,
      ADMIN_PERMISSION_RESOURCES.SENSITIVE_WORD,
      ADMIN_PERMISSION_ACTIONS.MANAGE
    )
  )
}

export const Route = createFileRoute('/_authenticated/sensitive-word-monitor/')(
  {
    beforeLoad: () => {
      const user = useAuthStore.getState().auth.user
      if (!canAccessSensitiveWordMonitor(user)) {
        throw redirect({ to: '/403' })
      }
    },
    component: SensitiveWordMonitor,
  }
)
