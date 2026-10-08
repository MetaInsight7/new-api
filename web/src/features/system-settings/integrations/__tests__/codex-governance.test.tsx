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

let client: QueryClient

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(() => client.clear())

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
  expect(parsed.ai_analysis_api_key).toBe('')
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

it('explicitly clears saved AI keys without emitting a fake configuration field', async () => {
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  await saveCodexGovernanceSettings(
    { ...codexGovernanceSettingsDefaults, clear_ai_analysis_api_key: true },
    codexGovernanceSettingsDefaults
  )
  expect(update).toHaveBeenCalledExactlyOnceWith('/api/option/', {
    key: 'codex_model_governance_setting.ai_analysis_api_key',
    value: '',
  })
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
