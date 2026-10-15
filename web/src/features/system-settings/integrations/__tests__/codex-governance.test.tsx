import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  codexGovernanceSettingsDefaults,
  codexGovernanceSettingsFromOptions,
  saveCodexGovernanceSettings,
} from '../codex-governance-settings'
import { CodexGovernanceSettingsSection } from '../codex-governance-settings-section'
import {
  monitorAlertSettingsDefaults,
  monitorAlertSettingsFromOptions,
  saveMonitorAlertSettings,
} from '../monitor-alert-settings'
import { MonitoringSettingsSection } from '../monitoring-settings-section'

let client: QueryClient

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(() => client.clear())

it('offers the shared robot and analysis controls only in Monitoring & Alerts', () => {
  render(
    <QueryClientProvider client={client}>
      <MonitoringSettingsSection
        defaultValues={{
          QuotaRemindThreshold: '',
          'perf_metrics_setting.enabled': false,
          'perf_metrics_setting.flush_interval': 5,
          'perf_metrics_setting.bucket_time': 'hour',
          'perf_metrics_setting.retention_days': 0,
        }}
      />
    </QueryClientProvider>
  )
  expect(
    screen.getByRole('switch', { name: 'Enable DingTalk alerts' })
  ).not.toBeChecked()
  expect(
    screen.getByRole('spinbutton', { name: 'Channel alert cooldown (minutes)' })
  ).toHaveValue(60)
  expect(
    screen.getByLabelText('Monitoring AI analysis API key')
  ).toHaveAttribute('type', 'password')
})

it('loads persisted governance options from the shared system-options query without defaultValues', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [
        { key: 'codex_model_governance_setting.enabled', value: 'true' },
        { key: 'codex_model_governance_setting.probe_enabled', value: 'true' },
        {
          key: 'codex_model_governance_setting.probe_interval_minutes',
          value: '180',
        },
        {
          key: 'codex_model_governance_setting.official_source_urls',
          value: '["https://official.example/notice"]',
        },
      ],
    },
  })
  render(
    <QueryClientProvider client={client}>
      <CodexGovernanceSettingsSection />
    </QueryClientProvider>
  )
  expect(
    await screen.findByRole('spinbutton', {
      name: 'Codex probe interval (minutes)',
    })
  ).toHaveValue(180)
  expect(
    screen.getByRole('switch', { name: 'Enable Codex model governance' })
  ).toBeChecked()
  expect(
    screen.getByRole('textbox', { name: 'Official Codex source URLs' })
  ).toHaveValue('https://official.example/notice')
  expect(screen.getByRole('button', { name: 'Run Codex probes' })).toBeEnabled()
  expect(
    screen.queryByLabelText('DingTalk webhook URL')
  ).not.toBeInTheDocument()
})

it('parses arrays and string options but never re-displays saved secrets', () => {
  const parsed = codexGovernanceSettingsFromOptions({
    'codex_model_governance_setting.official_lifecycle_terms':
      'retired\nsunset',
    'codex_model_governance_setting.probe_interval_minutes': '240',
    'codex_model_governance_setting.ai_analysis_api_key': 'not-for-display',
  })
  expect(parsed.official_lifecycle_terms).toBe('retired\nsunset')
  expect(parsed.probe_interval_minutes).toBe(240)
  expect(parsed).not.toHaveProperty('ai_analysis_api_key')
})

it('keeps blank saved secrets, serializes lists, and enables governance last', async () => {
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  await saveCodexGovernanceSettings(
    {
      ...codexGovernanceSettingsDefaults,
      enabled: true,
      probe_enabled: true,
      official_source_urls: 'https://official.example/notice\n',
    },
    codexGovernanceSettingsDefaults
  )
  expect(update).toHaveBeenLastCalledWith('/api/option/', {
    key: 'codex_model_governance_setting.enabled',
    value: true,
  })
  expect(update).toHaveBeenCalledWith('/api/option/', {
    key: 'codex_model_governance_setting.official_source_urls',
    value: '["https://official.example/notice"]',
  })
  expect(update).not.toHaveBeenCalledWith(
    '/api/option/',
    expect.objectContaining({
      key: 'codex_model_governance_setting.ai_analysis_api_key',
    })
  )
  expect(update).not.toHaveBeenCalledWith(
    '/api/option/',
    expect.objectContaining({
      key: 'codex_model_governance_setting.dingtalk_secret',
    })
  )
})

it('does not write obsolete connection fields even when old options exist', async () => {
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  await saveCodexGovernanceSettings(
    codexGovernanceSettingsFromOptions({
      'codex_model_governance_setting.dingtalk_enabled': 'true',
      'codex_model_governance_setting.ai_analysis_api_key': 'obsolete-key',
    }),
    codexGovernanceSettingsDefaults
  )
  expect(update).not.toHaveBeenCalled()
})

it('rejects a failed option update before enabling governance', async () => {
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: false, message: 'Invalid rule' } })
  await expect(
    saveCodexGovernanceSettings(
      {
        ...codexGovernanceSettingsDefaults,
        enabled: true,
        unsupported_message_patterns: '[',
      },
      codexGovernanceSettingsDefaults
    )
  ).rejects.toThrow('Invalid rule')
  expect(update).not.toHaveBeenCalledWith('/api/option/', {
    key: 'codex_model_governance_setting.enabled',
    value: true,
  })
})

it('preserves blank shared secrets, clears explicitly, and pins the displayed analysis endpoint and model', async () => {
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const saved = monitorAlertSettingsFromOptions({
    'monitor_setting.ai_analysis_api_key': 'not-for-display',
    'monitor_setting.dingtalk_alert_secret': 'not-for-display',
  })
  expect(saved.ai_analysis_api_key).toBe('')
  expect(saved.dingtalk_alert_secret).toBe('')
  await saveMonitorAlertSettings(saved, monitorAlertSettingsDefaults)
  expect(update).toHaveBeenCalledTimes(2)
  expect(update).toHaveBeenCalledWith('/api/option/', {
    key: 'monitor_setting.ai_analysis_base_url',
    value: 'https://api.openai.com/v1',
  })
  expect(update).toHaveBeenCalledWith('/api/option/', {
    key: 'monitor_setting.ai_analysis_model',
    value: 'gpt-5.4-mini',
  })
  update.mockClear()
  await saveMonitorAlertSettings(
    {
      ...saved,
      clear_ai_analysis_api_key: true,
      dingtalk_alert_secret: 'replacement',
    },
    saved
  )
  expect(update).toHaveBeenCalledWith('/api/option/', {
    key: 'monitor_setting.ai_analysis_api_key',
    value: '',
  })
  expect(update).toHaveBeenCalledWith('/api/option/', {
    key: 'monitor_setting.dingtalk_alert_secret',
    value: 'replacement',
  })
  expect(
    update.mock.calls.some((call) => JSON.stringify(call).includes('clear_'))
  ).toBe(false)
})

it('does not enable shared alerts when the target or analysis configuration fails to save', async () => {
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: false, message: 'Save rejected' } })
  await expect(
    saveMonitorAlertSettings(
      {
        ...monitorAlertSettingsDefaults,
        dingtalk_alert_enabled: true,
        dingtalk_alert_webhook_url: 'https://robot.example/send',
      },
      monitorAlertSettingsDefaults
    )
  ).rejects.toThrow('Save rejected')
  expect(update).not.toHaveBeenCalledWith('/api/option/', {
    key: 'monitor_setting.dingtalk_alert_enabled',
    value: true,
  })
})
