import type { UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import {
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import type {
  MonitoringFormInput,
  MonitoringFormValues,
} from './monitoring-settings-section'

export function MonitorAlertFields(props: {
  form: UseFormReturn<MonitoringFormInput, unknown, MonitoringFormValues>
}) {
  const { t } = useTranslation()
  const switches = [
    { field: 'dingtalk_alert_enabled', label: t('Enable DingTalk alerts') },
    {
      field: 'clear_dingtalk_alert_secret',
      label: t('Clear saved DingTalk secret'),
    },
    {
      field: 'clear_ai_analysis_api_key',
      label: t('Clear saved AI key and disable AI analysis'),
    },
  ] as const
  const textFields = [
    {
      field: 'dingtalk_alert_webhook_url',
      label: t('DingTalk webhook URL'),
      secret: false,
    },
    {
      field: 'dingtalk_alert_secret',
      label: t('DingTalk secret'),
      secret: true,
    },
    {
      field: 'ai_analysis_api_key',
      label: t('Monitoring AI analysis API key'),
      secret: true,
    },
    {
      field: 'ai_analysis_base_url',
      label: t('Monitoring AI analysis base URL'),
      secret: false,
    },
    {
      field: 'ai_analysis_model',
      label: t('Monitoring AI analysis model'),
      secret: false,
    },
  ] as const
  return (
    <>
      <h4 className='font-medium'>{t('Shared DingTalk and AI analysis')}</h4>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Scheduled test failures and Codex governance share this connection. Their alert cooldowns remain independent.'
        )}
      </p>
      <p className='text-muted-foreground text-sm'>
        {t(
          'The displayed analysis endpoint and model are effective values. Saving explicitly pins them; environment defaults apply only before configuration.'
        )}
      </p>
      <SettingsFormGrid>
        {switches.map((entry) => (
          <FormField
            key={entry.field}
            control={props.form.control}
            name={`monitor_setting.${entry.field}`}
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
        <FormField
          control={props.form.control}
          name='monitor_setting.dingtalk_alert_cooldown_minutes'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Channel alert cooldown (minutes)')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  max={153722867}
                  step={1}
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
        {textFields.map((entry) => (
          <FormField
            key={entry.field}
            control={props.form.control}
            name={`monitor_setting.${entry.field}`}
            render={({ field }) => (
              <FormItem>
                <FormLabel>{entry.label}</FormLabel>
                <FormControl>
                  <Input
                    type={entry.secret ? 'password' : 'text'}
                    autoComplete={entry.secret ? 'new-password' : 'off'}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        ))}
      </SettingsFormGrid>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Leave secrets blank to keep them; use the clear switches to remove them. An analysis key enables optional Chinese summaries. Failures fall back to basic alerts.'
        )}
      </p>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Known credential, URL and IP patterns are masked, not fully anonymized. Channel IDs, names and failure facts may be sent to the configured analysis service.'
        )}
      </p>
    </>
  )
}
