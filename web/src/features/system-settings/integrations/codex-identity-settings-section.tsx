import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
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
import { useUpdateCodexIdentityOptions } from '../hooks/use-update-option'
import {
  buildCodexIdentityOptionUpdates,
  formatSyncTime,
  type CodexIdentityFormValues,
} from './codex-identity-settings'

export function CodexIdentitySettingsSection({
  defaultValues,
}: {
  defaultValues: CodexIdentityFormValues
}) {
  const { t } = useTranslation()
  const updateOptions = useUpdateCodexIdentityOptions()
  const form = useForm<CodexIdentityFormValues>({ defaultValues })

  useResetForm(form, defaultValues)

  async function onSubmit(values: CodexIdentityFormValues) {
    const updates = buildCodexIdentityOptionUpdates(defaultValues, values)
    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }
    await updateOptions.mutateAsync({ updates })
  }

  return (
    <SettingsSection title={t('Codex Identity')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOptions.isPending || form.formState.isSubmitting}
            saveLabel='Save Codex identity settings'
          />

          <FormField
            control={form.control}
            name='CodexClientUserAgent'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Codex User-Agent')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='codex-cli/0.144.0'
                    autoComplete='off'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Optional canonical Codex CLI User-Agent. It must start with codex-cli/<version>; the version is rebuilt from the effective client version.'
                  )}
                </FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='CodexClientVersion'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Manual Codex client version')}</FormLabel>
                <FormControl>
                  <Input placeholder='0.144.0' autoComplete='off' {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Manual version wins over synced releases. Versions below 0.144.0 or invalid values fall back to the built-in safe version.'
                  )}
                </FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='CodexSyncedClientVersion'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Synced Codex client version')}</FormLabel>
                <FormControl>
                  <Input readOnly {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Read-only latest stable official release captured by the automatic sync.'
                  )}
                </FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='CodexSyncedClientVersionAt'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Last Codex version sync')}</FormLabel>
                <FormControl>
                  <Input
                    readOnly
                    value={formatSyncTime(field.value) || field.value}
                    aria-label={t('Last Codex version sync')}
                  />
                </FormControl>
                <FormDescription>
                  {t('Automatic release checks run every six hours.')}
                </FormDescription>
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='CodexAutoSyncClientVersion'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Auto-sync Codex client version')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Keep the synced version current from official stable releases.'
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
            name='CodexEnforceClientIdentity'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enforce Codex client identity')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Emergency kill switch. Disable only to roll back canonical outbound identity enforcement.'
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

        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
