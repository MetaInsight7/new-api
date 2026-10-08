import { Trash2 } from 'lucide-react'
import { useEffect, useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Field, FieldError, FieldGroup } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import { formatBillingModelName } from './lib/group-model-ratio'

type GroupModelRatioRowProps = {
  group: string
  model: string
  ratio: number
  modelNames: string[]
  onRename: (group: string, model: string, name: string) => void
  onRatioChange: (group: string, model: string, ratio: number) => void
  onRemove: (group: string, model: string) => void
  onValidityChange: (key: string, valid: boolean) => void
}

export function GroupModelRatioRow(props: GroupModelRatioRowProps) {
  const { t } = useTranslation()
  const errorId = useId()
  const [name, setName] = useState(props.model)
  const [ratio, setRatio] = useState(String(props.ratio))
  const onValidityChange = props.onValidityChange
  const normalizedName = formatBillingModelName(name)
  const nameChanged = name !== props.model
  const duplicate =
    nameChanged &&
    normalizedName !== props.model &&
    props.modelNames.includes(normalizedName)
  let nameError = ''
  if (!normalizedName) nameError = t('Enter a billing model key.')
  else if (duplicate) {
    nameError = t('This model already has a ratio in this group.')
  }
  const ratioValid =
    ratio.trim() !== '' && Number.isFinite(Number(ratio)) && Number(ratio) >= 0
  const rowKey = JSON.stringify([props.group, props.model])

  useEffect(() => {
    setRatio(String(props.ratio))
  }, [props.ratio])

  useEffect(() => {
    onValidityChange(rowKey, !nameError && ratioValid)
  }, [onValidityChange, rowKey, nameError, ratioValid])

  useEffect(
    () => () => onValidityChange(rowKey, true),
    [onValidityChange, rowKey]
  )

  return (
    <FieldGroup className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_140px_36px]'>
      <Field data-invalid={!!nameError}>
        <Input
          value={name}
          aria-label={t('Billing model key for {{group}} / {{model}}', props)}
          aria-invalid={!!nameError}
          aria-describedby={nameError ? `${errorId}-name` : undefined}
          placeholder={t('Model')}
          onChange={(event) => setName(event.target.value)}
          onBlur={() => {
            if (nameError || !nameChanged) return
            if (normalizedName === props.model) setName(props.model)
            else props.onRename(props.group, props.model, normalizedName)
          }}
        />
        {nameError && (
          <FieldError id={`${errorId}-name`}>{nameError}</FieldError>
        )}
      </Field>
      <Field data-invalid={!ratioValid}>
        <Input
          type='number'
          min={0}
          step='any'
          value={ratio}
          aria-label={t('Ratio for {{group}} / {{model}}', props)}
          aria-invalid={!ratioValid}
          aria-describedby={!ratioValid ? `${errorId}-ratio` : undefined}
          onChange={(event) => {
            const next = event.target.value
            setRatio(next)
            const value = Number(next)
            if (next.trim() !== '' && Number.isFinite(value) && value >= 0) {
              props.onRatioChange(props.group, props.model, value)
            }
          }}
        />
        {!ratioValid && (
          <FieldError id={`${errorId}-ratio`}>
            {t(
              'Enter a non-negative finite ratio. Zero means free; empty is not zero.'
            )}
          </FieldError>
        )}
      </Field>
      <Button
        type='button'
        variant='ghost'
        size='icon'
        className='text-destructive'
        aria-label={t('Remove model ratio for {{group}} / {{model}}', props)}
        onClick={() => props.onRemove(props.group, props.model)}
      >
        <Trash2 aria-hidden='true' />
      </Button>
    </FieldGroup>
  )
}
