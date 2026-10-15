import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  testCodexModelGovernanceRule,
  triggerCodexModelGovernanceTask,
} from '@/features/codex-model-governance/api'
import { handleServerError } from '@/lib/handle-server-error'

import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchItem,
  SettingsSwitchContent,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useSystemOptions } from '../hooks/use-system-options'
import {
  codexGovernanceLines,
  codexGovernanceSettingsFromOptions,
  codexGovernanceSettingsSchema,
  saveCodexGovernanceSettings,
  type CodexGovernanceSettingsValues,
} from './codex-governance-settings'

export function CodexGovernanceSettingsSection(props: {
  defaultValues?: CodexGovernanceSettingsValues
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const options = useSystemOptions()
  const savedValues = useMemo(
    () =>
      codexGovernanceSettingsFromOptions(
        Object.fromEntries(
          (options.data?.data ?? []).map((option) => [option.key, option.value])
        )
      ),
    [options.data]
  )
  const defaults = props.defaultValues ?? savedValues
  const form = useForm<CodexGovernanceSettingsValues>({
    resolver: zodResolver(codexGovernanceSettingsSchema),
    defaultValues: defaults,
  })
  useResetForm(form, defaults)
  const [sample, setSample] = useState('')
  const save = useMutation({
    mutationFn: (values: CodexGovernanceSettingsValues) =>
      saveCodexGovernanceSettings(values, defaults),
    onSuccess: (_, values) => {
      form.reset(values)
      toast.success(t('Setting updated successfully'))
      void queryClient.invalidateQueries({ queryKey: ['system-options'] })
    },
    onError: (error) => handleServerError(error, t('Failed to update setting')),
  })
  const rule = useMutation({
    mutationFn: () =>
      testCodexModelGovernanceRule(
        sample,
        codexGovernanceLines(form.getValues('unsupported_message_patterns'))
      ),
    onError: (error) =>
      handleServerError(error, t('Failed to test Codex rule')),
  })
  const trigger = useMutation({
    mutationFn: triggerCodexModelGovernanceTask,
    onSuccess: () => toast.success(t('Task queued')),
    onError: (error) => handleServerError(error, t('Failed to start task')),
  })
  const switches = [
    { field: 'enabled', label: t('Enable Codex model governance') },
    { field: 'probe_enabled', label: t('Probe Codex models') },
  ] as const
  const textFields = [
    {
      field: 'unsupported_message_patterns',
      label: t('Codex unsupported message patterns'),
      multiline: true,
    },
    {
      field: 'official_source_urls',
      label: t('Official Codex source URLs'),
      multiline: true,
    },
    {
      field: 'official_lifecycle_terms',
      label: t('Official lifecycle terms'),
      multiline: true,
    },
  ] as const

  if (!props.defaultValues && options.isError) {
    return (
      <ErrorState
        title={t('Failed to load settings')}
        onRetry={() => void options.refetch()}
      />
    )
  }
  if (!props.defaultValues && options.isPending) return <LoadingState />

  return (
    <SettingsSection title={t('Codex model governance')}>
      <Form {...form}>
        <SettingsForm
          autoComplete='off'
          onSubmit={form.handleSubmit((values) => save.mutate(values))}
        >
          <SettingsPageFormActions
            onSave={form.handleSubmit((values) => save.mutate(values))}
            isSaving={save.isPending}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Only probe-confirmed unsupported models disable direct channel abilities. Official notices and AI findings require manual review.'
            )}
          </p>
          <SettingsFormGrid>
            {switches.map((entry) => (
              <FormField
                key={entry.field}
                control={form.control}
                name={entry.field}
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{entry.label}</FormLabel>
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
            ))}
            {(
              ['probe_interval_minutes', 'alert_cooldown_minutes'] as const
            ).map((name) => (
              <FormField
                key={name}
                control={form.control}
                name={name}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {name === 'probe_interval_minutes'
                        ? t('Codex probe interval (minutes)')
                        : t('Codex governance alert cooldown (minutes)')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={name === 'probe_interval_minutes' ? 60 : 1}
                        max={525600}
                        value={field.value}
                        onChange={(event) =>
                          field.onChange(Number(event.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ))}
            {textFields.map((entry) => (
              <FormField
                key={entry.field}
                control={form.control}
                name={entry.field}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{entry.label}</FormLabel>
                    <FormControl>
                      <Textarea rows={4} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            ))}
          </SettingsFormGrid>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Configure the shared DingTalk robot and AI service in Monitoring & Alerts. Governance rules and its alert cooldown remain independent.'
            )}
          </p>
          <div className='space-y-2'>
            <Label htmlFor='codex-governance-rule-sample'>
              {t('Test Codex rule')}
            </Label>
            <Textarea
              id='codex-governance-rule-sample'
              value={sample}
              onChange={(event) => {
                setSample(event.target.value)
                rule.reset()
              }}
            />
            <Button
              type='button'
              variant='outline'
              disabled={rule.isPending || !sample.trim()}
              onClick={() => rule.mutate()}
            >
              {t('Test Codex rule')}
            </Button>
            {rule.data && (
              <p role='status'>
                {rule.data.matched
                  ? t('Rule matched')
                  : t('Rule did not match')}{' '}
                {rule.data.model_name} {rule.data.matched_rule}
              </p>
            )}
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              variant='outline'
              disabled={
                trigger.isPending ||
                !defaults.enabled ||
                !defaults.probe_enabled
              }
              onClick={() => trigger.mutate('probe')}
            >
              {t('Run Codex probes')}
            </Button>
            <Button
              type='button'
              variant='outline'
              disabled={
                trigger.isPending ||
                !defaults.enabled ||
                !codexGovernanceLines(defaults.official_source_urls).length
              }
              onClick={() => trigger.mutate('notice')}
            >
              {t('Check official notices')}
            </Button>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
