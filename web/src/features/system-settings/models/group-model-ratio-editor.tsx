import { ChevronDown, Plus } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { tryJsonParse } from '../utils/json-parser'
import { GroupModelRatioRow } from './group-model-ratio-row'
import {
  parseGroupModelRatios,
  type GroupModelRatioMap,
} from './lib/group-model-ratio'

type GroupModelRatioEditorProps = {
  value: string
  groupRatio: string
  onChange: (value: string) => void
  onValidityChange: (valid: boolean) => void
}

export function GroupModelRatioEditor(props: GroupModelRatioEditorProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({})
  const [newGroupName, setNewGroupName] = useState('')
  const onValidityChange = props.onValidityChange
  const [invalidRows, setInvalidRows] = useState<Set<string>>(new Set())
  const ratios = useMemo(
    () => parseGroupModelRatios(props.value),
    [props.value]
  )
  const groupNames = useMemo(() => {
    const parsed = tryJsonParse<unknown>(props.groupRatio)
    const pricingGroups =
      parsed.success &&
      parsed.data &&
      typeof parsed.data === 'object' &&
      !Array.isArray(parsed.data)
        ? Object.keys(parsed.data)
        : []
    return [...new Set([...pricingGroups, ...Object.keys(ratios ?? {})])]
  }, [props.groupRatio, ratios])
  const setRowValidity = useCallback((key: string, valid: boolean) => {
    setInvalidRows((current) => {
      if (current.has(key) === !valid) return current
      const next = new Set(current)
      if (valid) next.delete(key)
      else next.add(key)
      return next
    })
  }, [])

  useEffect(() => {
    onValidityChange(ratios !== null && invalidRows.size === 0)
  }, [onValidityChange, ratios, invalidRows])

  const emit = (next: GroupModelRatioMap) =>
    props.onChange(JSON.stringify(next, null, 2))

  const addModel = (group: string) => {
    if (ratios === null) return
    const modelRatios = Object.hasOwn(ratios, group) ? ratios[group] : {}
    let index = Object.keys(modelRatios).length + 1
    while (Object.hasOwn(modelRatios, `model-${index}`)) index += 1
    emit({ ...ratios, [group]: { ...modelRatios, [`model-${index}`]: 1 } })
    setOpenGroups((current) => ({ ...current, [group]: true }))
  }

  return (
    <section aria-label={t('Model-specific group ratios')}>
      <Card>
        <CardHeader className='border-b'>
          <div className='flex flex-wrap items-start justify-between gap-3'>
            <div className='min-w-0 flex-1 space-y-1'>
              <CardTitle>{t('Model-specific group ratios')}</CardTitle>
              <CardDescription>
                {t(
                  'Nested JSON map of group → model → ratio. This overrides inter-group and group ratios for matching models.'
                )}
                <br />
                {t(
                  'Use billing-normalized model keys, same as ModelRatio, for example gpt-4o-gizmo-*.'
                )}
                <br />
                {t(
                  'These overrides do not create groups or grant model access.'
                )}
              </CardDescription>
            </div>
            <div className='flex w-full gap-2 sm:w-64'>
              <Input
                value={newGroupName}
                aria-label={t('Group for model-specific ratios')}
                placeholder={t('Group name')}
                disabled={ratios === null}
                onChange={(event) => setNewGroupName(event.target.value)}
              />
              <Button
                type='button'
                variant='outline'
                aria-label={t('Add model ratio group')}
                disabled={ratios === null || !newGroupName.trim()}
                onClick={() => {
                  addModel(newGroupName.trim())
                  setNewGroupName('')
                }}
              >
                <Plus aria-hidden='true' />
                {t('Add')}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className='space-y-3'>
          {ratios === null && (
            <ErrorState
              className='min-h-0'
              title={t('Invalid model-specific ratios')}
              description={t(
                'Switch to JSON to repair this configuration. Visual editing will not overwrite invalid data.'
              )}
            />
          )}
          {ratios !== null && groupNames.length === 0 && (
            <EmptyState
              className='min-h-0'
              title={t('No model-specific ratios')}
            />
          )}
          {ratios !== null &&
            groupNames.map((group) => {
              const modelRatios = Object.hasOwn(ratios, group)
                ? ratios[group]
                : {}
              const entries = Object.entries(modelRatios)
              const open = Object.hasOwn(openGroups, group)
                ? openGroups[group]
                : entries.length > 0
              return (
                <Collapsible
                  key={group}
                  open={open}
                  className='rounded-lg border'
                  onOpenChange={(nextOpen) =>
                    setOpenGroups((current) => ({
                      ...current,
                      [group]: nextOpen,
                    }))
                  }
                >
                  <div className='flex items-center justify-between gap-2 p-3'>
                    <CollapsibleTrigger
                      render={
                        <Button
                          type='button'
                          variant='ghost'
                          className='min-w-0 justify-start'
                        />
                      }
                      aria-label={t('Model ratios for {{group}}', { group })}
                    >
                      <ChevronDown
                        aria-hidden='true'
                        className={open ? 'rotate-180' : ''}
                      />
                      <span className='truncate font-semibold'>{group}</span>
                      <span className='text-muted-foreground text-xs'>
                        {t('{{formattedCount}} model overrides', {
                          formattedCount: formatNumber(entries.length, locale),
                        })}
                      </span>
                    </CollapsibleTrigger>
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      aria-label={t('Add model ratio for {{group}}', { group })}
                      onClick={() => addModel(group)}
                    >
                      <Plus aria-hidden='true' />
                      {t('Add')}
                    </Button>
                  </div>
                  <CollapsibleContent keepMounted>
                    <div className='space-y-3 border-t p-3'>
                      {entries.length === 0 ? (
                        <p className='text-muted-foreground text-sm'>
                          {t('No model-specific ratios')}
                        </p>
                      ) : (
                        entries.map(([model, ratio]) => (
                          <GroupModelRatioRow
                            key={model}
                            group={group}
                            model={model}
                            ratio={ratio}
                            modelNames={Object.keys(modelRatios)}
                            onValidityChange={setRowValidity}
                            onRatioChange={(groupName, modelName, value) =>
                              emit({
                                ...ratios,
                                [groupName]: {
                                  ...ratios[groupName],
                                  [modelName]: value,
                                },
                              })
                            }
                            onRename={(groupName, oldName, newName) => {
                              const updated = { ...ratios[groupName] }
                              const value = updated[oldName]
                              delete updated[oldName]
                              emit({
                                ...ratios,
                                [groupName]: { ...updated, [newName]: value },
                              })
                            }}
                            onRemove={(groupName, modelName) => {
                              const updated = { ...ratios[groupName] }
                              delete updated[modelName]
                              const next = { ...ratios }
                              if (Object.keys(updated).length) {
                                next[groupName] = updated
                              } else {
                                delete next[groupName]
                              }
                              emit(next)
                            }}
                          />
                        ))
                      )}
                    </div>
                  </CollapsibleContent>
                </Collapsible>
              )
            })}
        </CardContent>
      </Card>
    </section>
  )
}
