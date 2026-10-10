import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import type { ChannelConcurrencySettingsValues } from '../types'
import { safeNumberFieldProps } from '../utils/numeric-field'

export type { ChannelConcurrencySettingsValues } from '../types'

type ChannelConcurrencyFormValues = {
  channel_concurrency_setting: {
    wait_enabled: boolean
    wait_timeout_ms: number
    max_waiting_per_channel: number
    cooldown_enabled: boolean
    cooldown_seconds: number
    cooldown_on_status_429: boolean
    cooldown_on_message_match: boolean
    load_cache_enabled: boolean
    slot_ttl_minutes: number
    wait_interval_ms: number
    load_cache_ttl_ms: number
    max_acquire_attempts: number
  }
}

type ChannelConcurrencySettingsSectionProps = {
  defaultValues: ChannelConcurrencySettingsValues
}

const optionSaveOrder = [
  'channel_concurrency_setting.wait_enabled',
  'channel_concurrency_setting.wait_timeout_ms',
  'channel_concurrency_setting.max_waiting_per_channel',
  'channel_concurrency_setting.cooldown_enabled',
  'channel_concurrency_setting.cooldown_seconds',
  'channel_concurrency_setting.cooldown_on_status_429',
  'channel_concurrency_setting.cooldown_on_message_match',
  'channel_concurrency_setting.load_cache_enabled',
  'channel_concurrency_setting.slot_ttl_minutes',
  'channel_concurrency_setting.wait_interval_ms',
  'channel_concurrency_setting.load_cache_ttl_ms',
  'channel_concurrency_setting.max_acquire_attempts',
] as const satisfies readonly (keyof ChannelConcurrencySettingsValues)[]

function buildFormValues(
  values: ChannelConcurrencySettingsValues
): ChannelConcurrencyFormValues {
  return {
    channel_concurrency_setting: {
      wait_enabled: values['channel_concurrency_setting.wait_enabled'],
      wait_timeout_ms: values['channel_concurrency_setting.wait_timeout_ms'],
      max_waiting_per_channel:
        values['channel_concurrency_setting.max_waiting_per_channel'],
      cooldown_enabled: values['channel_concurrency_setting.cooldown_enabled'],
      cooldown_seconds: values['channel_concurrency_setting.cooldown_seconds'],
      cooldown_on_status_429:
        values['channel_concurrency_setting.cooldown_on_status_429'],
      cooldown_on_message_match:
        values['channel_concurrency_setting.cooldown_on_message_match'],
      load_cache_enabled:
        values['channel_concurrency_setting.load_cache_enabled'],
      slot_ttl_minutes: values['channel_concurrency_setting.slot_ttl_minutes'],
      wait_interval_ms: values['channel_concurrency_setting.wait_interval_ms'],
      load_cache_ttl_ms:
        values['channel_concurrency_setting.load_cache_ttl_ms'],
      max_acquire_attempts:
        values['channel_concurrency_setting.max_acquire_attempts'],
    },
  }
}

function normalizeFormValues(
  values: ChannelConcurrencyFormValues
): ChannelConcurrencySettingsValues {
  return {
    'channel_concurrency_setting.wait_enabled':
      values.channel_concurrency_setting.wait_enabled,
    'channel_concurrency_setting.wait_timeout_ms':
      values.channel_concurrency_setting.wait_timeout_ms,
    'channel_concurrency_setting.max_waiting_per_channel':
      values.channel_concurrency_setting.max_waiting_per_channel,
    'channel_concurrency_setting.cooldown_enabled':
      values.channel_concurrency_setting.cooldown_enabled,
    'channel_concurrency_setting.cooldown_seconds':
      values.channel_concurrency_setting.cooldown_seconds,
    'channel_concurrency_setting.cooldown_on_status_429':
      values.channel_concurrency_setting.cooldown_on_status_429,
    'channel_concurrency_setting.cooldown_on_message_match':
      values.channel_concurrency_setting.cooldown_on_message_match,
    'channel_concurrency_setting.load_cache_enabled':
      values.channel_concurrency_setting.load_cache_enabled,
    'channel_concurrency_setting.slot_ttl_minutes':
      values.channel_concurrency_setting.slot_ttl_minutes,
    'channel_concurrency_setting.wait_interval_ms':
      values.channel_concurrency_setting.wait_interval_ms,
    'channel_concurrency_setting.load_cache_ttl_ms':
      values.channel_concurrency_setting.load_cache_ttl_ms,
    'channel_concurrency_setting.max_acquire_attempts':
      values.channel_concurrency_setting.max_acquire_attempts,
  }
}

export function ChannelConcurrencySettingsSection({
  defaultValues,
}: ChannelConcurrencySettingsSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption({
    invalidateSystemOptions: false,
    showFeedback: false,
  })
  const formSchema = useMemo(() => {
    const integerBetween = (minimum: number, maximum: number) => {
      const message = t(
        'Enter a whole number from {{minimum}} to {{maximum}}.',
        { minimum, maximum }
      )
      return z
        .number({ error: message })
        .int(message)
        .min(minimum, message)
        .max(maximum, message)
    }

    return z.object({
      channel_concurrency_setting: z.object({
        wait_enabled: z.boolean(),
        wait_timeout_ms: integerBetween(1, 60000),
        max_waiting_per_channel: integerBetween(0, 10000),
        cooldown_enabled: z.boolean(),
        cooldown_seconds: integerBetween(1, 3600),
        cooldown_on_status_429: z.boolean(),
        cooldown_on_message_match: z.boolean(),
        load_cache_enabled: z.boolean(),
        slot_ttl_minutes: integerBetween(1, 1440),
        wait_interval_ms: integerBetween(1, 5000),
        load_cache_ttl_ms: integerBetween(1, 5000),
        max_acquire_attempts: integerBetween(1, 100),
      }),
    })
  }, [t])
  const formDefaults = useMemo(
    () => buildFormValues(defaultValues),
    [defaultValues]
  )
  const form = useForm<ChannelConcurrencyFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: formDefaults,
  })
  const baselineRef = useRef(defaultValues)
  const serializedServerDefaultsRef = useRef(JSON.stringify(defaultValues))

  useResetForm(form, formDefaults)

  useEffect(() => {
    const serialized = JSON.stringify(defaultValues)
    if (serialized === serializedServerDefaultsRef.current) return

    baselineRef.current = defaultValues
    serializedServerDefaultsRef.current = serialized
  }, [defaultValues])
  const waitEnabled = form.watch('channel_concurrency_setting.wait_enabled')
  const cooldownEnabled = form.watch(
    'channel_concurrency_setting.cooldown_enabled'
  )
  const loadCacheEnabled = form.watch(
    'channel_concurrency_setting.load_cache_enabled'
  )

  const handleSubmit = form.handleSubmit(async (values) => {
    const normalized = normalizeFormValues(values)
    const changedKeys = optionSaveOrder.filter(
      (key) => baselineRef.current[key] !== normalized[key]
    )
    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    try {
      for (const key of changedKeys) {
        await updateOption.mutateAsync({ key, value: normalized[key] })
      }
    } catch {
      await queryClient.invalidateQueries({ queryKey: ['system-options'] })
      toast.error(
        t(
          'Some channel concurrency settings may already be active. Later changes were not saved; server values are being reloaded.'
        )
      )
      return
    }

    baselineRef.current = normalized
    form.reset(buildFormValues(normalized))
    await queryClient.invalidateQueries({ queryKey: ['system-options'] })
    toast.success(t('Channel concurrency settings saved'))
  })

  return (
    <SettingsSection title={t('Channel Concurrency Limits')}>
      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={form.formState.isSubmitting}
          />

          <p
            data-settings-form-span='full'
            className='text-muted-foreground text-sm'
          >
            {t(
              'Configure waiting, cooldown, and load observation for channels with a concurrency limit.'
            )}
          </p>

          <div data-settings-form-span='full'>
            <h4 className='font-medium'>{t('Common policies')}</h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Frequently adjusted waiting, cooldown, and cache controls.')}
            </p>
          </div>

          <FormField
            control={form.control}
            name='channel_concurrency_setting.wait_enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Wait when channel concurrency is full')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'When waiting is off, its dependent values remain visible and keep their saved values.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.wait_timeout_ms'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Wait timeout (milliseconds)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={60000}
                    step={1}
                    disabled={!waitEnabled}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Maximum time a request waits for an available channel slot.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.max_waiting_per_channel'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Maximum waiting requests per channel')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    max={10000}
                    step={1}
                    disabled={!waitEnabled}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  <span className='block'>
                    {t('Maximum queued requests per limited channel.')}
                  </span>
                  <span className='block'>
                    {t(
                      '0 automatically follows the channel concurrency limit.'
                    )}
                  </span>
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.cooldown_enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable concurrency cooldown')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When cooldown is off, its duration and triggers remain visible and keep their saved values.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.cooldown_seconds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Cooldown duration (seconds)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={3600}
                    step={1}
                    disabled={!cooldownEnabled}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'How long a channel stays out of selection after a configured cooldown trigger.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.cooldown_on_status_429'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Cooldown on upstream HTTP 429')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Triggers cooldown for explicit upstream HTTP 429 responses.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!cooldownEnabled}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.cooldown_on_message_match'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>
                    {t('Cooldown on fuzzy error message match')}
                  </FormLabel>
                  <FormDescription>
                    {t(
                      'Also matches rate-limit-like error text and may cause false positives; off by default.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!cooldownEnabled}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.load_cache_enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable load snapshot cache')}</FormLabel>
                  <FormDescription>
                    {t(
                      'The cache only assists load observation; authoritative slot acquisition still enforces the limit. Its TTL remains visible and keeps its saved value while caching is off.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <div data-settings-form-span='full'>
            <h4 className='font-medium'>{t('Advanced runtime parameters')}</h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'Low-level lease, polling, cache, and acquisition safeguards.'
              )}
            </p>
          </div>

          <FormField
            control={form.control}
            name='channel_concurrency_setting.slot_ttl_minutes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Slot TTL (minutes)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={1440}
                    step={1}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'How long an acquired slot lease remains valid without renewal.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.wait_interval_ms'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Wait polling interval (milliseconds)')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={5000}
                    step={1}
                    disabled={!waitEnabled}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'How often waiting requests retry authoritative slot acquisition.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.load_cache_ttl_ms'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Load snapshot cache TTL (milliseconds)')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={5000}
                    step={1}
                    disabled={!loadCacheEnabled}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'How briefly load observations are reused for routing and status reads.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='channel_concurrency_setting.max_acquire_attempts'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('Maximum acquisition attempts per round')}
                </FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={100}
                    step={1}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Limits how many candidates one routing round may try to acquire.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
