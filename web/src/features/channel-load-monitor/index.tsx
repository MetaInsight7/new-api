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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type PaginationState,
} from '@tanstack/react-table'
import {
  Activity,
  Gauge,
  RefreshCw,
  Settings,
  Snowflake,
  UsersRound,
} from 'lucide-react'
import {
  useEffect,
  useMemo,
  useState,
  type ComponentType,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTableFacetedFilter,
  DataTablePagination,
  type DataTableFacetedFilterOption,
} from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { ChannelConcurrencyCell } from '@/features/channels/components/channel-concurrency-status'
import { ChannelTypeLogo } from '@/features/channels/components/channel-type-badge'
import {
  ChannelGroupsCell,
  ChannelPriorityCell,
  ChannelStatusCell,
  ChannelTypeCell,
} from '@/features/channels/components/channels-columns'
import { CHANNEL_STATUS_OPTIONS } from '@/features/channels/constants'
import {
  channelsQueryKeys,
  getChannelConcurrencyState,
  getChannelTypeLabel,
  type ChannelConcurrencyState,
} from '@/features/channels/lib'
import { useDebounce } from '@/hooks/use-debounce'
import { useMediaQuery } from '@/hooks/use-media-query'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatTimestampToDate } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { getChannelLoadMonitor } from './api'
import type {
  ChannelLoadMonitorItem,
  ChannelLoadMonitorParams,
  ChannelLoadMonitorSummary,
  ChannelLoadSortBy,
  ChannelLoadSortOrder,
  ChannelLoadState,
} from './types'

const route = getRouteApi('/_authenticated/channel-load-monitor/')
const REFRESH_INTERVAL_MS = 10_000

function MetricCard({
  title,
  value,
  description,
  icon: Icon,
}: {
  title: string
  value: string
  description: string
  icon: ComponentType<{ className?: string }>
}) {
  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle className='text-muted-foreground text-xs font-medium'>
          {title}
        </CardTitle>
        <CardAction>
          <Icon className='text-muted-foreground size-4' />
        </CardAction>
      </CardHeader>
      <CardContent>
        <div className='text-2xl font-semibold tracking-tight tabular-nums'>
          {value}
        </div>
        <CardDescription className='mt-1 text-xs'>
          {description}
        </CardDescription>
      </CardContent>
    </Card>
  )
}

function SummaryCards({
  summary,
  locale,
  t,
}: {
  summary: ChannelLoadMonitorSummary
  locale?: string
  t: ReturnType<typeof useTranslation>['t']
}) {
  const utilization =
    summary.capacity > 0
      ? Math.round((summary.active / summary.capacity) * 100)
      : 0
  return (
    <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
      <MetricCard
        title={t('Limited-channel RPM')}
        value={formatNumber(summary.limited_rpm ?? 0, locale)}
        description={t('Only channels with concurrency limits')}
        icon={UsersRound}
      />
      <MetricCard
        title={t('Active slots')}
        value={`${formatNumber(summary.active, locale)}/${formatNumber(
          summary.capacity,
          locale
        )}`}
        description={t('{{percent}}% overall utilization', {
          percent: formatNumber(utilization, locale),
        })}
        icon={Activity}
      />
      <MetricCard
        title={t('Waiting requests')}
        value={formatNumber(summary.waiting, locale)}
        description={
          summary.waiting > 0
            ? t('Review saturated channels')
            : t('No requests are waiting')
        }
        icon={Gauge}
      />
      <MetricCard
        title={t('Cooling channels')}
        value={formatNumber(summary.cooling_down, locale)}
        description={
          summary.cooling_down > 0
            ? t('Temporarily excluded from routing')
            : t('No channels are cooling')
        }
        icon={Snowflake}
      />
    </div>
  )
}

const stateVariants: Record<ChannelConcurrencyState, StatusVariant> = {
  unlimited: 'neutral',
  unavailable: 'neutral',
  idle: 'neutral',
  busy: 'warning',
  saturated: 'danger',
  over_limit: 'danger',
}

function ChannelStateBadges({ item }: { item: ChannelLoadMonitorItem }) {
  const { t } = useTranslation()
  const state = getChannelConcurrencyState(
    item.channel.max_concurrency,
    item.concurrency ?? undefined
  )
  const labels: Record<ChannelConcurrencyState, string> = {
    unlimited: t('Unlimited'),
    unavailable: t('Unavailable'),
    idle: t('Idle'),
    busy: t('Running'),
    saturated: t('At capacity'),
    over_limit: t('Over limit'),
  }
  return (
    <div className='flex flex-nowrap items-center gap-1'>
      <StatusBadge
        label={labels[state]}
        variant={stateVariants[state]}
        copyable={false}
      />
      {item.concurrency?.cooling_down && (
        <StatusBadge label={t('Cooling')} variant='blue' copyable={false} />
      )}
    </div>
  )
}

function SlotUsageCell({ item }: { item: ChannelLoadMonitorItem }) {
  const { t } = useTranslation()
  const maxConcurrency = item.channel.max_concurrency
  if (maxConcurrency <= 0) {
    return (
      <ChannelConcurrencyCell
        channel={item.channel}
        status={item.concurrency ?? undefined}
        showSecondaryBadges={false}
      />
    )
  }

  const active = item.concurrency?.active ?? 0
  const percentage = Math.min(100, (active / maxConcurrency) * 100)
  const state = getChannelConcurrencyState(
    maxConcurrency,
    item.concurrency ?? undefined
  )
  let progressTone = 'text-success'
  if (state === 'saturated' || state === 'over_limit') {
    progressTone = 'text-destructive'
  } else if (state === 'busy') {
    progressTone = 'text-warning'
  }

  return (
    <div className='flex min-w-40 items-center gap-2.5'>
      <Progress
        value={percentage}
        aria-label={`${item.channel.name} ${t('Slot usage')}`}
        className={cn(
          'w-24 shrink-0 gap-0 [&_[data-slot=progress-indicator]]:bg-current [&_[data-slot=progress-track]]:h-1.5',
          progressTone
        )}
      />
      <ChannelConcurrencyCell
        channel={item.channel}
        status={item.concurrency ?? undefined}
        showSecondaryBadges={false}
      />
    </div>
  )
}

function LoadMonitorCard({ item }: { item: ChannelLoadMonitorItem }) {
  const { t } = useTranslation()
  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle className='min-w-0'>
          <span className='block truncate'>{item.channel.name}</span>
          <span className='text-muted-foreground text-xs font-normal'>
            #{item.channel.id}
          </span>
        </CardTitle>
        <CardAction>
          <ChannelStatusCell channel={item.channel} />
        </CardAction>
      </CardHeader>
      <CardContent className='space-y-3'>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-xs'>{t('Slots')}</span>
          <SlotUsageCell item={item} />
        </div>
        <div className='flex items-center justify-between gap-3'>
          <span className='text-muted-foreground text-xs'>
            {t('Load state')}
          </span>
          <ChannelStateBadges item={item} />
        </div>
        <div className='grid grid-cols-2 gap-3 text-xs'>
          <div>
            <div className='text-muted-foreground'>{t('Type')}</div>
            <div className='mt-1'>
              <ChannelTypeCell channel={item.channel} />
            </div>
          </div>
          <div>
            <div className='text-muted-foreground'>
              {t('Priority / Weight')}
            </div>
            <div className='mt-1 flex items-center gap-1'>
              <ChannelPriorityCell channel={item.channel} readOnly />
              <span className='text-muted-foreground'>/</span>
              <span className='font-mono tabular-nums'>
                {item.channel.weight ?? 0}
              </span>
            </div>
          </div>
        </div>
        <ChannelGroupsCell group={item.channel.group} />
      </CardContent>
    </Card>
  )
}

function ViewStateCard({ children }: { children: ReactNode }) {
  return (
    <Card>
      <CardContent className='flex min-h-40 items-center justify-center text-center'>
        {children}
      </CardContent>
    </Card>
  )
}

export function ChannelLoadMonitor() {
  const { t, i18n } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const currentUser = useAuthStore((state) => state.auth.user)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const currentPage = search.page ?? 1
  const effectivePageSize = search.pageSize ?? (isMobile ? 10 : 20)
  const currentFilter = search.filter ?? ''
  const currentStatus = search.status ?? 'all'
  const currentType = search.type ?? 'all'
  const currentGroup = search.group ?? 'all'
  const currentLoadState = search.loadState ?? 'all'
  const currentSortBy = search.sortBy ?? 'priority'
  const currentSortOrder = search.sortOrder ?? 'desc'
  const [filterInput, setFilterInput] = useState(currentFilter)
  const debouncedFilter = useDebounce(filterInput, 300)

  useEffect(() => setFilterInput(currentFilter), [currentFilter])
  useEffect(() => {
    if (debouncedFilter === currentFilter) return
    void navigate({
      search: (previous) => ({
        ...previous,
        filter: debouncedFilter,
        page: 1,
      }),
      replace: true,
    })
  }, [currentFilter, debouncedFilter, navigate])

  const params = useMemo<ChannelLoadMonitorParams>(() => {
    const channelType = currentType === 'all' ? undefined : Number(currentType)
    return {
      p: currentPage,
      page_size: effectivePageSize,
      keyword: currentFilter || undefined,
      status: currentStatus === 'all' ? undefined : currentStatus,
      type: Number.isFinite(channelType) ? channelType : undefined,
      group: currentGroup === 'all' ? undefined : currentGroup,
      load_state: currentLoadState,
      sort_by: currentSortBy,
      sort_order: currentSortOrder,
    }
  }, [
    currentFilter,
    currentGroup,
    currentLoadState,
    currentPage,
    currentSortBy,
    currentSortOrder,
    currentStatus,
    currentType,
    effectivePageSize,
  ])

  const monitorQuery = useQuery({
    queryKey: channelsQueryKeys.concurrencyMonitor(params),
    queryFn: ({ signal }) => getChannelLoadMonitor(params, signal),
    staleTime: 0,
    refetchOnMount: 'always',
    refetchOnWindowFocus: 'always',
    refetchInterval: REFRESH_INTERVAL_MS,
    refetchIntervalInBackground: false,
    retry: false,
    meta: { errorToast: false },
  })
  const updateSearch = (patch: Record<string, unknown>) => {
    void navigate({
      search: (previous) => ({ ...previous, ...patch, page: 1 }),
      replace: true,
    })
  }

  const statusOptions = useMemo<DataTableFacetedFilterOption[]>(
    () => CHANNEL_STATUS_OPTIONS.map((option) => ({ ...option })),
    []
  )
  const typeOptions = useMemo<DataTableFacetedFilterOption[]>(() => {
    const counts = monitorQuery.data?.type_counts ?? {}
    const options = Object.entries(counts)
      .map(([type, count]) => ({ type: Number(type), count: Number(count) }))
      .filter((item) => item.type > 0 && item.count > 0)
      .sort((left, right) =>
        t(getChannelTypeLabel(left.type)).localeCompare(
          t(getChannelTypeLabel(right.type))
        )
      )
      .map((item) => ({
        label: getChannelTypeLabel(item.type),
        value: String(item.type),
        count: item.count,
        iconNode: <ChannelTypeLogo type={item.type} size={16} />,
      }))
    const selectedType = currentType === 'all' ? undefined : Number(currentType)
    if (
      selectedType &&
      !options.some((option) => option.value === String(selectedType))
    ) {
      options.push({
        label: getChannelTypeLabel(selectedType),
        value: String(selectedType),
        count: Number(counts[currentType]) || 0,
        iconNode: <ChannelTypeLogo type={selectedType} size={16} />,
      })
    }
    return [
      {
        label: 'All Types',
        value: 'all',
        count: Object.values(counts).reduce(
          (sum, count) => sum + (Number(count) || 0),
          0
        ),
      },
      ...options,
    ]
  }, [currentType, monitorQuery.data?.type_counts, t])
  const groupOptions = useMemo<DataTableFacetedFilterOption[]>(() => {
    const groups = [...(monitorQuery.data?.groups ?? [])]
    if (currentGroup !== 'all' && !groups.includes(currentGroup)) {
      groups.push(currentGroup)
    }
    groups.sort((left, right) => left.localeCompare(right))
    return [
      { value: 'all', label: 'All Groups' },
      ...groups.map((group) => ({ value: group, label: group })),
    ]
  }, [currentGroup, monitorQuery.data?.groups])
  const sortOptions = useMemo<DataTableFacetedFilterOption[]>(
    () => [
      { value: 'priority:desc', label: 'Priority: high to low' },
      { value: 'id:asc', label: 'ID: low to high' },
      { value: 'load:desc', label: 'Slot utilization: high to low' },
      { value: 'waiting:desc', label: 'Waiting: high to low' },
    ],
    []
  )

  const columns = useMemo<ColumnDef<ChannelLoadMonitorItem>[]>(
    () => [
      {
        id: 'channel',
        header: t('Channel'),
        cell: ({ row }) => (
          <div className='w-32 min-w-0'>
            <div
              className='truncate font-medium'
              title={row.original.channel.name}
            >
              {row.original.channel.name}
            </div>
            <div className='text-muted-foreground text-xs'>
              #{row.original.channel.id}
            </div>
          </div>
        ),
      },
      {
        id: 'type',
        header: t('Type'),
        cell: ({ row }) => <ChannelTypeCell channel={row.original.channel} />,
      },
      {
        id: 'group',
        header: t('Group'),
        cell: ({ row }) => (
          <ChannelGroupsCell group={row.original.channel.group} />
        ),
      },
      {
        id: 'priority',
        header: t('Priority / Weight'),
        cell: ({ row }) => (
          <div className='flex items-center gap-1'>
            <ChannelPriorityCell channel={row.original.channel} readOnly />
            <span className='text-muted-foreground'>/</span>
            <span className='font-mono tabular-nums'>
              {row.original.channel.weight ?? 0}
            </span>
          </div>
        ),
      },
      {
        id: 'slots',
        header: t('Slot usage'),
        cell: ({ row }) => <SlotUsageCell item={row.original} />,
      },
      {
        id: 'waiting',
        header: t('Waiting'),
        cell: ({ row }) => {
          const waiting = row.original.concurrency?.waiting ?? 0
          return waiting > 0 ? (
            <StatusBadge
              label={`+${waiting}`}
              variant='neutral'
              copyable={false}
            />
          ) : (
            <span className='text-muted-foreground'>0</span>
          )
        },
      },
      {
        id: 'state',
        header: t('Status'),
        cell: ({ row }) => (
          <div className='flex flex-wrap items-center gap-1'>
            <ChannelStatusCell channel={row.original.channel} />
            <ChannelStateBadges item={row.original} />
          </div>
        ),
      },
    ],
    [t]
  )

  const pagination: PaginationState = {
    pageIndex: currentPage - 1,
    pageSize: effectivePageSize,
  }
  const table = useReactTable({
    data: monitorQuery.data?.items ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    rowCount: monitorQuery.data?.total ?? 0,
    state: { pagination },
    onPaginationChange: (updater) => {
      const next = typeof updater === 'function' ? updater(pagination) : updater
      void navigate({
        search: (previous) => ({
          ...previous,
          page: next.pageIndex + 1,
          pageSize: next.pageSize,
        }),
        replace: true,
      })
    },
  })

  const loadFilters: Array<{ value: ChannelLoadState; label: string }> = [
    { value: 'all', label: t('All') },
    { value: 'full', label: t('At capacity') },
    { value: 'queued', label: t('Queued') },
    { value: 'cooling', label: t('Cooling') },
  ]
  const isStale = monitorQuery.isError && monitorQuery.data !== undefined

  return (
    <SectionPageLayout stackActionsOnMobile>
      <SectionPageLayout.Title>
        <span className='flex min-w-0 flex-col gap-0.5 sm:flex-row sm:items-baseline sm:gap-3'>
          <span>{t('Channel Load Monitor')}</span>
          <span className='text-muted-foreground truncate text-xs font-normal'>
            {t('Current snapshot · no history is stored')}
          </span>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        {currentUser?.role === ROLE.SUPER_ADMIN && (
          <Button
            variant='outline'
            size='sm'
            render={
              <Link
                to='/system-settings/operations/$section'
                params={{ section: 'channel-concurrency' }}
              />
            }
          >
            <Settings />
            {t('Concurrency policy')}
          </Button>
        )}
        <Button
          variant='outline'
          size='sm'
          disabled={monitorQuery.isFetching}
          onClick={() => void monitorQuery.refetch()}
        >
          <RefreshCw
            className={cn(monitorQuery.isFetching && 'animate-spin')}
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          {monitorQuery.data ? (
            <SummaryCards
              summary={monitorQuery.data.summary}
              locale={locale}
              t={t}
            />
          ) : (
            <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
              {Array.from({ length: 4 }, (_, index) => (
                <Skeleton key={index} className='h-28 rounded-xl' />
              ))}
            </div>
          )}

          <Card size='sm'>
            <CardContent>
              <div className='flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between'>
                <div className='min-w-0 flex-1 space-y-3'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <Input
                      value={filterInput}
                      onChange={(event) => setFilterInput(event.target.value)}
                      placeholder={t('Search by channel name or ID')}
                      aria-label={t('Search channels')}
                      className='w-full sm:w-64'
                    />
                    <DataTableFacetedFilter<ChannelLoadMonitorItem, string>
                      title={t('Status')}
                      options={statusOptions}
                      singleSelect
                      value={currentStatus === 'all' ? [] : [currentStatus]}
                      onValueChange={(values) =>
                        updateSearch({ status: values[0] ?? 'all' })
                      }
                    />
                    <DataTableFacetedFilter<ChannelLoadMonitorItem, string>
                      title={t('Type')}
                      options={typeOptions}
                      singleSelect
                      value={currentType === 'all' ? [] : [currentType]}
                      onValueChange={(values) =>
                        updateSearch({ type: values[0] ?? 'all' })
                      }
                    />
                    <DataTableFacetedFilter<ChannelLoadMonitorItem, string>
                      title={t('Group')}
                      options={groupOptions}
                      singleSelect
                      value={currentGroup === 'all' ? [] : [currentGroup]}
                      onValueChange={(values) =>
                        updateSearch({ group: values[0] ?? 'all' })
                      }
                    />
                    <DataTableFacetedFilter<ChannelLoadMonitorItem, string>
                      title={t('Priority')}
                      options={sortOptions}
                      singleSelect
                      value={[`${currentSortBy}:${currentSortOrder}`]}
                      onValueChange={(values) => {
                        const value = values[0] ?? 'priority:desc'
                        const [sortBy, sortOrder] = value.split(':') as [
                          ChannelLoadSortBy,
                          ChannelLoadSortOrder,
                        ]
                        updateSearch({ sortBy, sortOrder })
                      }}
                    />
                  </div>
                  <div className='overflow-x-auto pb-1'>
                    <ToggleGroup
                      value={[currentLoadState]}
                      variant='outline'
                      size='sm'
                      aria-label={t('Load state')}
                      onValueChange={(values) => {
                        const loadState = values[0] as
                          | ChannelLoadState
                          | undefined
                        if (loadState) updateSearch({ loadState })
                      }}
                    >
                      {loadFilters.map((filter) => (
                        <ToggleGroupItem
                          key={filter.value}
                          value={filter.value}
                        >
                          {filter.label}
                        </ToggleGroupItem>
                      ))}
                    </ToggleGroup>
                  </div>
                </div>
                {monitorQuery.data && (
                  <div className='text-muted-foreground flex shrink-0 flex-col gap-1 text-xs xl:items-end xl:text-right'>
                    <span>
                      {t('Auto-refreshing every {{seconds}}s', {
                        seconds: 10,
                      })}
                    </span>
                    <span>
                      {t('Last updated')}:{' '}
                      {formatTimestampToDate(
                        monitorQuery.data.snapshot_at,
                        'milliseconds'
                      )}
                    </span>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>

          {isStale && monitorQuery.data && (
            <div
              role='status'
              className='border-warning/40 bg-warning/10 text-warning rounded-lg border px-3 py-2 text-sm'
            >
              {t(
                'Refresh failed. Showing the last successful snapshot from {{time}}.',
                {
                  time: formatTimestampToDate(
                    monitorQuery.data.snapshot_at,
                    'milliseconds'
                  ),
                }
              )}
            </div>
          )}

          {monitorQuery.isPending && !monitorQuery.data && (
            <ViewStateCard>
              <div className='w-full space-y-3'>
                <Skeleton className='mx-auto h-5 w-48' />
                <Skeleton className='h-24 w-full' />
              </div>
            </ViewStateCard>
          )}
          {monitorQuery.isError && !monitorQuery.data && (
            <ViewStateCard>
              <div className='space-y-3'>
                <div>
                  <div className='font-medium'>
                    {t('Failed to load channel load snapshot')}
                  </div>
                  <div className='text-muted-foreground mt-1 text-sm'>
                    {t('Try refreshing the page or retry the request.')}
                  </div>
                </div>
                <Button
                  variant='outline'
                  onClick={() => void monitorQuery.refetch()}
                >
                  {t('Retry')}
                </Button>
              </div>
            </ViewStateCard>
          )}
          {monitorQuery.data && monitorQuery.data.items.length === 0 && (
            <ViewStateCard>
              <div>
                <div className='font-medium'>{t('No matching channels')}</div>
                <div className='text-muted-foreground mt-1 text-sm'>
                  {t('Adjust the filters to see more channels.')}
                </div>
              </div>
            </ViewStateCard>
          )}
          {monitorQuery.data && monitorQuery.data.items.length > 0 && (
            <>
              <div className='hidden overflow-hidden rounded-xl border sm:block'>
                <Table>
                  <TableHeader>
                    {table.getHeaderGroups().map((headerGroup) => (
                      <TableRow key={headerGroup.id}>
                        {headerGroup.headers.map((header) => (
                          <TableHead key={header.id}>
                            {header.isPlaceholder
                              ? null
                              : flexRender(
                                  header.column.columnDef.header,
                                  header.getContext()
                                )}
                          </TableHead>
                        ))}
                      </TableRow>
                    ))}
                  </TableHeader>
                  <TableBody>
                    {table.getRowModel().rows.map((row) => (
                      <TableRow key={row.original.channel.id}>
                        {row.getVisibleCells().map((cell) => (
                          <TableCell key={cell.id}>
                            {flexRender(
                              cell.column.columnDef.cell,
                              cell.getContext()
                            )}
                          </TableCell>
                        ))}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <div className='grid gap-3 sm:hidden'>
                {monitorQuery.data.items.map((item) => (
                  <LoadMonitorCard key={item.channel.id} item={item} />
                ))}
              </div>
              <DataTablePagination table={table} compact={isMobile} />
            </>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
