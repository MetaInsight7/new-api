import { ListPlus, Loader2, RefreshCw, Search, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'

import {
  applyGroupModelRatioBatch,
  formatBillingModelName,
  type GroupModelRatioBatchResult,
  type GroupModelRatioMap,
} from './lib/group-model-ratio'

type BatchMode = 'set' | 'remove'
type ConfigurationFilter = 'all' | 'configured' | 'unconfigured'

type GroupModelRatioBatchSheetProps = {
  open: boolean
  group: string
  ratios: GroupModelRatioMap
  onOpenChange: (open: boolean) => void
  onApply: (ratios: GroupModelRatioMap) => void
}

export function GroupModelRatioBatchSheet(
  props: GroupModelRatioBatchSheetProps
) {
  const { t } = useTranslation()
  const pricing = usePricingData(props.open)
  const [search, setSearch] = useState('')
  const [vendor, setVendor] = useState('all')
  const [configuration, setConfiguration] =
    useState<ConfigurationFilter>('unconfigured')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [mode, setMode] = useState<BatchMode>('set')
  const [ratioDraft, setRatioDraft] = useState('1')
  const [overwrite, setOverwrite] = useState(false)
  const [removeConfirmationOpen, setRemoveConfirmationOpen] = useState(false)
  const currentGroup = Object.hasOwn(props.ratios, props.group)
    ? props.ratios[props.group]
    : {}

  const models = useMemo(() => {
    const unique = new Map<string, (typeof pricing.models)[number]>()
    for (const model of pricing.models) {
      if (
        !model.enable_groups.includes('all') &&
        !model.enable_groups.includes(props.group)
      ) {
        continue
      }
      if (!unique.has(model.model_name)) unique.set(model.model_name, model)
    }
    return [...unique.values()].sort((left, right) =>
      left.model_name.localeCompare(right.model_name)
    )
  }, [pricing.models, props.group])

  const vendorOptions = useMemo(() => {
    const options = new Map<string, string>()
    for (const model of models) {
      if (model.vendor_id == null) continue
      options.set(
        String(model.vendor_id),
        model.vendor_name ?? String(model.vendor_id)
      )
    }
    return [...options.entries()].sort((left, right) =>
      left[1].localeCompare(right[1])
    )
  }, [models])

  const isConfigured = (modelName: string) =>
    Object.hasOwn(currentGroup, formatBillingModelName(modelName))

  const configuredCount = models.filter((model) =>
    isConfigured(model.model_name)
  ).length
  const filteredModels = models.filter((model) => {
    const term = search.trim().toLowerCase()
    if (
      term &&
      !model.model_name.toLowerCase().includes(term) &&
      !model.vendor_name?.toLowerCase().includes(term)
    ) {
      return false
    }
    if (vendor !== 'all' && String(model.vendor_id) !== vendor) return false
    const configured = isConfigured(model.model_name)
    if (configuration === 'configured' && !configured) return false
    if (configuration === 'unconfigured' && configured) return false
    return true
  })
  const filteredNames = filteredModels.map((model) => model.model_name)
  const hiddenSelectedCount = [...selected].filter(
    (modelName) => !filteredNames.includes(modelName)
  ).length
  const ratio = Number(ratioDraft)
  const ratioValid =
    ratioDraft.trim() !== '' && Number.isFinite(ratio) && ratio >= 0
  const preview = useMemo<GroupModelRatioBatchResult | null>(() => {
    if (mode === 'set' && !ratioValid) return null
    return applyGroupModelRatioBatch({
      ratios: props.ratios,
      group: props.group,
      modelNames: [...selected],
      action:
        mode === 'set' ? { type: 'set', ratio, overwrite } : { type: 'remove' },
    })
  }, [mode, overwrite, props.group, props.ratios, ratio, ratioValid, selected])
  const changedCount = preview
    ? preview.added + preview.updated + preview.removed
    : 0

  const setModeAndDefaults = (nextMode: BatchMode) => {
    setMode(nextMode)
    setConfiguration(nextMode === 'set' ? 'unconfigured' : 'configured')
    setSelected(new Set())
  }
  const selectAllFiltered = () => {
    setSelected((current) => new Set([...current, ...filteredNames]))
  }
  const applyPreview = () => {
    if (!preview || changedCount === 0) return
    props.onApply(preview.ratios)
    props.onOpenChange(false)
  }

  return (
    <>
      <Sheet open={props.open} onOpenChange={props.onOpenChange}>
        <SheetContent className={sideDrawerContentClassName('sm:max-w-3xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>
              {t('Batch model ratios for {{group}}', { group: props.group })}
            </SheetTitle>
            <SheetDescription>
              {t(
                'Filters select current models only. The saved configuration contains exact billing keys, not wildcard rules.'
              )}
            </SheetDescription>
          </SheetHeader>

          <div className={sideDrawerFormClassName('gap-4')}>
            <div className='grid gap-3 sm:grid-cols-[minmax(0,1fr)_180px_180px]'>
              <div className='relative'>
                <Search
                  aria-hidden='true'
                  className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2'
                />
                <Input
                  value={search}
                  className='pl-9'
                  aria-label={t('Search models')}
                  placeholder={t('Search models...')}
                  onChange={(event) => setSearch(event.target.value)}
                />
              </div>
              <Select
                value={vendor}
                onValueChange={(value) => value && setVendor(value)}
              >
                <SelectTrigger aria-label={t('Vendor')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All vendors')}</SelectItem>
                    {vendorOptions.map(([value, label]) => (
                      <SelectItem key={value} value={value}>
                        {label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Select
                value={configuration}
                onValueChange={(value) => {
                  if (
                    value === 'all' ||
                    value === 'configured' ||
                    value === 'unconfigured'
                  ) {
                    setConfiguration(value)
                  }
                }}
              >
                <SelectTrigger aria-label={t('Configuration status')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All models')}</SelectItem>
                    <SelectItem value='configured'>
                      {t('Configured')}
                    </SelectItem>
                    <SelectItem value='unconfigured'>
                      {t('Unconfigured')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>

            <div className='flex flex-wrap items-center gap-2'>
              <Badge variant='outline'>
                {t('{{count}} available', { count: models.length })}
              </Badge>
              <Badge variant='secondary'>
                {t('{{count}} configured', { count: configuredCount })}
              </Badge>
              <Badge variant='outline'>
                {t('{{count}} unconfigured', {
                  count: models.length - configuredCount,
                })}
              </Badge>
              <div className='ml-auto flex flex-wrap gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={filteredNames.length === 0}
                  onClick={selectAllFiltered}
                >
                  {t('Select all (filtered)')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={selected.size === 0}
                  onClick={() => setSelected(new Set())}
                >
                  {t('Clear selection')}
                </Button>
              </div>
            </div>

            <div className='overflow-hidden rounded-lg border'>
              <div className='bg-muted/40 flex items-center justify-between border-b px-3 py-2 text-xs'>
                <span>
                  {t('{{count}} selected models', { count: selected.size })}
                </span>
                {hiddenSelectedCount > 0 && (
                  <span className='text-muted-foreground'>
                    {t('{{count}} selected outside the current filters', {
                      count: hiddenSelectedCount,
                    })}
                  </span>
                )}
              </div>
              <div className='max-h-[min(44dvh,460px)] overflow-y-auto'>
                {pricing.isLoading && (
                  <div className='text-muted-foreground flex items-center justify-center gap-2 p-8'>
                    <Loader2
                      aria-hidden='true'
                      className='size-4 animate-spin'
                    />
                    {t('Loading...')}
                  </div>
                )}
                {!pricing.isLoading && pricing.error && (
                  <div className='flex flex-col items-center gap-3 p-8 text-center'>
                    <p className='text-destructive text-sm'>
                      {t('Unable to load available models')}
                    </p>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => void pricing.refetch()}
                    >
                      <RefreshCw aria-hidden='true' />
                      {t('Retry')}
                    </Button>
                  </div>
                )}
                {!pricing.isLoading &&
                  !pricing.error &&
                  filteredModels.length === 0 && (
                    <p className='text-muted-foreground p-8 text-center text-sm'>
                      {t('No models match the selected filters')}
                    </p>
                  )}
                {!pricing.isLoading &&
                  !pricing.error &&
                  filteredModels.length > 0 && (
                    <div className='divide-y'>
                      {filteredModels.map((model) => {
                        const checked = selected.has(model.model_name)
                        const billingKey = formatBillingModelName(
                          model.model_name
                        )
                        const configured = Object.hasOwn(
                          currentGroup,
                          billingKey
                        )
                        return (
                          <label
                            key={model.model_name}
                            className='hover:bg-muted/40 flex cursor-pointer items-start gap-3 px-3 py-2.5'
                          >
                            <Checkbox
                              checked={checked}
                              aria-label={t('Select {{model}}', {
                                model: model.model_name,
                              })}
                              onCheckedChange={(value) => {
                                setSelected((current) => {
                                  const next = new Set(current)
                                  if (value) next.add(model.model_name)
                                  else next.delete(model.model_name)
                                  return next
                                })
                              }}
                            />
                            <span className='min-w-0 flex-1'>
                              <span className='block truncate font-mono text-sm'>
                                {model.model_name}
                              </span>
                              <span className='text-muted-foreground flex flex-wrap gap-x-2 text-xs'>
                                {model.vendor_name && (
                                  <span>{model.vendor_name}</span>
                                )}
                                {billingKey !== model.model_name && (
                                  <span>
                                    {t('Billing key: {{key}}', {
                                      key: billingKey,
                                    })}
                                  </span>
                                )}
                              </span>
                            </span>
                            {configured && (
                              <Badge variant='secondary'>
                                {t('Ratio {{ratio}}', {
                                  ratio: currentGroup[billingKey],
                                })}
                              </Badge>
                            )}
                          </label>
                        )
                      })}
                    </div>
                  )}
              </div>
            </div>

            <div className='grid gap-3 rounded-lg border p-3 sm:grid-cols-[180px_160px_minmax(0,1fr)]'>
              <Select
                value={mode}
                onValueChange={(value) => {
                  if (value === 'set' || value === 'remove') {
                    setModeAndDefaults(value)
                  }
                }}
              >
                <SelectTrigger aria-label={t('Batch action')}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='set'>{t('Set ratio')}</SelectItem>
                    <SelectItem value='remove'>{t('Remove ratios')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <div>
                <Input
                  type='number'
                  min={0}
                  step='any'
                  value={ratioDraft}
                  disabled={mode === 'remove'}
                  aria-label={t('Ratio')}
                  aria-invalid={mode === 'set' && !ratioValid}
                  onChange={(event) => setRatioDraft(event.target.value)}
                />
                {mode === 'set' && !ratioValid && (
                  <p className='text-destructive mt-1 text-xs'>
                    {t(
                      'Enter a non-negative finite ratio. Zero means free; empty is not zero.'
                    )}
                  </p>
                )}
              </div>
              <label className='flex items-center gap-2 self-start pt-2 text-sm'>
                <Checkbox
                  checked={overwrite}
                  disabled={mode === 'remove'}
                  onCheckedChange={(value) => setOverwrite(Boolean(value))}
                />
                <span>{t('Overwrite existing ratios')}</span>
              </label>
            </div>

            <div className='bg-muted/35 grid gap-2 rounded-lg border p-3 text-sm sm:grid-cols-4'>
              <span>
                {t('Added: {{count}}', { count: preview?.added ?? 0 })}
              </span>
              <span>
                {t('Updated: {{count}}', { count: preview?.updated ?? 0 })}
              </span>
              <span>
                {t('Removed: {{count}}', { count: preview?.removed ?? 0 })}
              </span>
              <span>
                {t('Unchanged: {{count}}', { count: preview?.unchanged ?? 0 })}
              </span>
              {!!preview?.collapsed && (
                <p className='text-muted-foreground sm:col-span-4'>
                  {t(
                    '{{count}} duplicate model selections were merged into existing normalized billing keys.',
                    { count: preview.collapsed }
                  )}
                </p>
              )}
            </div>
          </div>

          <SheetFooter className={sideDrawerFooterClassName()}>
            <SheetClose render={<Button type='button' variant='outline' />}>
              {t('Cancel')}
            </SheetClose>
            <Button
              type='button'
              variant={mode === 'remove' ? 'destructive' : 'default'}
              disabled={!preview || selected.size === 0 || changedCount === 0}
              onClick={() => {
                if (mode === 'remove') setRemoveConfirmationOpen(true)
                else applyPreview()
              }}
            >
              {mode === 'remove' ? (
                <Trash2 aria-hidden='true' />
              ) : (
                <ListPlus aria-hidden='true' />
              )}
              {t('Apply to draft')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <AlertDialog
        open={removeConfirmationOpen}
        onOpenChange={setRemoveConfirmationOpen}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Remove selected model ratios?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                '{{count}} exact model ratio overrides will be removed from the draft. Other models and groups stay unchanged.',
                { count: preview?.removed ?? 0 }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction variant='destructive' onClick={applyPreview}>
              {t('Remove ratios')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
