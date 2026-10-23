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
import { SystemBehaviorSection } from '../general/system-behavior-section'
import { CodexGovernanceSettingsSection } from '../integrations/codex-governance-settings-section'
import { CodexIdentitySettingsSection } from '../integrations/codex-identity-settings-section'
import { EmailSettingsSection } from '../integrations/email-settings-section'
import { MonitoringSettingsSection } from '../integrations/monitoring-settings-section'
import { WorkerSettingsSection } from '../integrations/worker-settings-section'
import { LogSettingsSection } from '../maintenance/log-settings-section'
import { PerformanceSection } from '../maintenance/performance-section'
import { UpdateCheckerSection } from '../maintenance/update-checker-section'
import type { OperationsSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { ChannelConcurrencySettingsSection } from './channel-concurrency-settings-section'

const OPERATIONS_SECTIONS = [
  {
    id: 'behavior',
    titleKey: 'System Behavior',
    build: (settings: OperationsSettings) => (
      <SystemBehaviorSection
        defaultValues={{
          DefaultCollapseSidebar: settings.DefaultCollapseSidebar,
          DemoSiteEnabled: settings.DemoSiteEnabled,
          SelfUseModeEnabled: settings.SelfUseModeEnabled,
        }}
      />
    ),
  },
  {
    id: 'channel-concurrency',
    titleKey: 'Channel Concurrency Limits',
    build: (settings: OperationsSettings) => (
      <ChannelConcurrencySettingsSection
        defaultValues={{
          'channel_concurrency_setting.wait_enabled':
            settings['channel_concurrency_setting.wait_enabled'],
          'channel_concurrency_setting.wait_timeout_ms':
            settings['channel_concurrency_setting.wait_timeout_ms'],
          'channel_concurrency_setting.max_waiting_per_channel':
            settings['channel_concurrency_setting.max_waiting_per_channel'],
          'channel_concurrency_setting.cooldown_enabled':
            settings['channel_concurrency_setting.cooldown_enabled'],
          'channel_concurrency_setting.cooldown_seconds':
            settings['channel_concurrency_setting.cooldown_seconds'],
          'channel_concurrency_setting.cooldown_on_status_429':
            settings['channel_concurrency_setting.cooldown_on_status_429'],
          'channel_concurrency_setting.cooldown_on_message_match':
            settings['channel_concurrency_setting.cooldown_on_message_match'],
          'channel_concurrency_setting.load_cache_enabled':
            settings['channel_concurrency_setting.load_cache_enabled'],
          'channel_concurrency_setting.slot_ttl_minutes':
            settings['channel_concurrency_setting.slot_ttl_minutes'],
          'channel_concurrency_setting.wait_interval_ms':
            settings['channel_concurrency_setting.wait_interval_ms'],
          'channel_concurrency_setting.load_cache_ttl_ms':
            settings['channel_concurrency_setting.load_cache_ttl_ms'],
          'channel_concurrency_setting.max_acquire_attempts':
            settings['channel_concurrency_setting.max_acquire_attempts'],
        }}
      />
    ),
  },
  {
    id: 'codex-identity',
    titleKey: 'Codex Identity',
    build: (settings: OperationsSettings) => (
      <CodexIdentitySettingsSection
        defaultValues={{
          CodexClientUserAgent: settings.CodexClientUserAgent,
          CodexClientVersion: settings.CodexClientVersion,
          CodexSyncedClientVersion: settings.CodexSyncedClientVersion,
          CodexSyncedClientVersionAt: settings.CodexSyncedClientVersionAt,
          CodexAutoSyncClientVersion: settings.CodexAutoSyncClientVersion,
          CodexEnforceClientIdentity: settings.CodexEnforceClientIdentity,
        }}
      />
    ),
  },
  {
    id: 'alerts',
    titleKey: 'Monitoring & Alerts',
    build: (settings: OperationsSettings) => (
      <MonitoringSettingsSection
        monitorOptions={settings}
        defaultValues={{
          QuotaRemindThreshold: settings.QuotaRemindThreshold,
          'perf_metrics_setting.enabled':
            settings['perf_metrics_setting.enabled'] ?? true,
          'perf_metrics_setting.flush_interval':
            settings['perf_metrics_setting.flush_interval'] ?? 5,
          'perf_metrics_setting.bucket_time':
            settings['perf_metrics_setting.bucket_time'] ?? 'hour',
          'perf_metrics_setting.retention_days':
            settings['perf_metrics_setting.retention_days'] ?? 0,
        }}
      />
    ),
  },
  {
    id: 'codex-governance',
    titleKey: 'Codex Model Governance',
    build: () => <CodexGovernanceSettingsSection />,
  },
  {
    id: 'email',
    titleKey: 'Email & Notifications',
    build: (settings: OperationsSettings) => (
      <EmailSettingsSection
        defaultValues={{
          QuotaRemindEnabled: settings.QuotaRemindEnabled,
          QuotaRemindThreshold: settings.QuotaRemindThreshold,
          EmailProvider: settings.EmailProvider,
          ResendApiKey: settings.ResendApiKey,
          ResendKeyConfigured: settings.ResendKeyConfigured,
          ResendFrom: settings.ResendFrom,
          SMTPServer: settings.SMTPServer,
          SMTPPort: settings.SMTPPort,
          SMTPAccount: settings.SMTPAccount,
          SMTPFrom: settings.SMTPFrom,
          SMTPToken: settings.SMTPToken,
          SMTPSSLEnabled: settings.SMTPSSLEnabled,
          SMTPStartTLSEnabled: settings.SMTPStartTLSEnabled,
          SMTPInsecureSkipVerify: settings.SMTPInsecureSkipVerify,
          SMTPForceAuthLogin: settings.SMTPForceAuthLogin,
        }}
      />
    ),
  },
  {
    id: 'worker',
    titleKey: 'Worker Proxy',
    build: (settings: OperationsSettings) => (
      <WorkerSettingsSection
        defaultValues={{
          WorkerUrl: settings.WorkerUrl,
          WorkerValidKey: settings.WorkerValidKey,
          WorkerAllowHttpImageRequestEnabled:
            settings.WorkerAllowHttpImageRequestEnabled,
        }}
      />
    ),
  },
  {
    id: 'logs',
    titleKey: 'Log Maintenance',
    build: (settings: OperationsSettings) => (
      <LogSettingsSection
        defaultValues={{
          LogConsumeEnabled: Boolean(settings.LogConsumeEnabled),
          'log_setting.user_export_enabled':
            settings['log_setting.user_export_enabled'] ?? true,
          'log_setting.max_range_days':
            settings['log_setting.max_range_days'] ?? 365,
        }}
      />
    ),
  },
  {
    id: 'performance',
    titleKey: 'Performance',
    build: (settings: OperationsSettings) => (
      <PerformanceSection
        defaultValues={{
          'performance_setting.disk_cache_enabled':
            settings['performance_setting.disk_cache_enabled'] ?? false,
          'performance_setting.disk_cache_threshold_mb':
            settings['performance_setting.disk_cache_threshold_mb'] ?? 10,
          'performance_setting.disk_cache_max_size_mb':
            settings['performance_setting.disk_cache_max_size_mb'] ?? 1024,
          'performance_setting.disk_cache_path':
            settings['performance_setting.disk_cache_path'] ?? '',
          'performance_setting.monitor_enabled':
            settings['performance_setting.monitor_enabled'] ?? false,
          'performance_setting.monitor_cpu_threshold':
            settings['performance_setting.monitor_cpu_threshold'] ?? 90,
          'performance_setting.monitor_memory_threshold':
            settings['performance_setting.monitor_memory_threshold'] ?? 90,
          'performance_setting.monitor_disk_threshold':
            settings['performance_setting.monitor_disk_threshold'] ?? 95,
        }}
      />
    ),
  },
  {
    id: 'update-checker',
    titleKey: 'System maintenance',
    build: (
      _settings: OperationsSettings,
      currentVersion?: string | null,
      startTime?: number | null
    ) => (
      <UpdateCheckerSection
        currentVersion={currentVersion}
        startTime={startTime}
      />
    ),
  },
] as const

export type OperationsSectionId = (typeof OPERATIONS_SECTIONS)[number]['id']

const operationsRegistry = createSectionRegistry<
  OperationsSectionId,
  OperationsSettings,
  [string | null | undefined, number | null | undefined]
>({
  sections: OPERATIONS_SECTIONS,
  defaultSection: 'behavior',
  basePath: '/system-settings/operations',
  urlStyle: 'path',
})

export const OPERATIONS_SECTION_IDS = operationsRegistry.sectionIds
export const OPERATIONS_DEFAULT_SECTION = operationsRegistry.defaultSection
export const getOperationsSectionNavItems =
  operationsRegistry.getSectionNavItems
export const getOperationsSectionContent = operationsRegistry.getSectionContent
export const getOperationsSectionMeta = operationsRegistry.getSectionMeta
