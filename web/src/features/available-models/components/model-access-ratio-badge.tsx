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
import type { ComponentProps } from 'react'

import { GroupMultiplierBadge } from '@/components/group-badge'
import { cn } from '@/lib/utils'

type ModelAccessRatioBadgeProps = ComponentProps<
  typeof GroupMultiplierBadge
> & {
  tone?: 'exclusive' | 'group'
}

export function ModelAccessRatioBadge({
  className,
  tone = 'group',
  ...props
}: ModelAccessRatioBadgeProps) {
  return (
    <GroupMultiplierBadge
      {...props}
      className={cn(
        'h-5 min-w-0 px-2 py-0 text-[11px] leading-none font-semibold tabular-nums',
        tone === 'exclusive'
          ? 'border-violet-400/60 bg-violet-500/15 text-violet-600 dark:border-violet-300/60 dark:bg-violet-400/20 dark:text-violet-200'
          : 'border-primary/25 bg-primary/10 text-primary',
        className
      )}
    />
  )
}
