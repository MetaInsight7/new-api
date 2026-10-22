import { useQuery } from '@tanstack/react-query'
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
import { useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getApiKeys } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'
import { getPricing } from '@/features/pricing/api'
import { useStatus } from '@/hooks/use-status'
import { getUserModels } from '@/lib/api'

import { IntegrationCards } from './integration-cards'
import { IntegrationDialog } from './integration-dialog'
import {
  API_KEY_PLACEHOLDER,
  normalizeApiKey,
  resolveApiEndpoint,
  resolveSnippetModel,
  type IntegrationId,
  type SnippetContext,
  type SnippetKind,
} from './integration-snippets'
import { useResolvedApiKeys } from './use-resolved-api-keys'

const PREFERRED_EXAMPLE_MODELS = [
  'gpt-4o-mini',
  'gpt-4.1-mini',
  'gpt-5-mini',
  'gpt-4o',
]

const EXCLUDED_ENDPOINT_TYPES = new Set([
  'embeddings',
  'jina-rerank',
  'video-to-music',
])
const EXCLUDED_NAME_PATTERN = /(^|[-_./])(embedding|tts)/i
const IMAGE_NAME_PATTERN = /(^|[-_./])(image|banana)/i
const VIDEO_NAME_PATTERN = /(^|[-_./])(seedance|sora|veo|kling)/i
const GENERIC_VIDEO_NAME_PATTERN = /(^|[-_./])video/i
const VIDEO_ENDPOINT_TYPES = new Set(['openai-video'])
const CHAT_ENDPOINT_TYPES = new Set([
  'openai',
  'openai-response',
  'openai-response-compact',
  'anthropic',
  'gemini',
])
const RECENT_KEY_LIMIT = 3
const EMPTY_API_KEYS: ApiKey[] = []

function getCurrentOrigin(): string {
  if (typeof window === 'undefined') return ''
  return window.location.origin
}

function pickDefaultModel(models: string[]): string {
  return (
    PREFERRED_EXAMPLE_MODELS.find((model) => models.includes(model)) ??
    models[0] ??
    'gpt-4o-mini'
  )
}

function getPreferredKey(keys: ApiKey[]): ApiKey | null {
  return keys.find((item) => item.status === 1) ?? keys[0] ?? null
}

export function IntegrationPanel() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [openIntegration, setOpenIntegration] = useState<IntegrationId | null>(
    null
  )
  const [selectedKeyId, setSelectedKeyId] = useState<number | null>(null)
  const [selectedModel, setSelectedModel] = useState<string | null>(null)

  const apiKeysQuery = useQuery({
    queryKey: ['workspace', 'integration', 'api-keys'],
    queryFn: async () => {
      const result = await getApiKeys({ p: 1, size: RECENT_KEY_LIMIT })
      if (!result.success) {
        throw new Error(result.message || t('Failed to load API keys'))
      }
      return result.data?.items ?? []
    },
    enabled: openIntegration !== null,
    staleTime: 60 * 1000,
    retry: false,
    meta: { errorToast: false },
  })

  const apiKeys = apiKeysQuery.data ?? EMPTY_API_KEYS
  const selectedKey = useMemo(() => {
    if (selectedKeyId !== null) {
      return apiKeys.find((key) => key.id === selectedKeyId) ?? null
    }
    return getPreferredKey(apiKeys)
  }, [apiKeys, selectedKeyId])

  const { resolvedKeys, loadingKeys, resolveKey } = useResolvedApiKeys(
    selectedKey?.id ?? null,
    openIntegration !== null
  )

  const modelsQuery = useQuery({
    queryKey: ['workspace', 'integration', 'models'],
    queryFn: async () => {
      const result = await getUserModels()
      if (!result.success) {
        throw new Error(result.message || t('Failed to load models'))
      }
      return result.data ?? []
    },
    enabled: openIntegration !== null,
    staleTime: 5 * 60 * 1000,
    retry: false,
    meta: { errorToast: false },
  })

  const pricingQuery = useQuery({
    queryKey: ['pricing'],
    queryFn: getPricing,
    enabled: openIntegration !== null,
    staleTime: 5 * 60 * 1000,
  })

  const modelEndpointTags = useMemo(() => {
    const tags = new Map<string, string[]>()
    for (const model of pricingQuery.data?.data ?? []) {
      tags.set(model.model_name, model.supported_endpoint_types ?? [])
    }
    return tags
  }, [pricingQuery.data])

  const classifyModel = useCallback(
    (model: string): SnippetKind | null => {
      const endpointTypes = modelEndpointTags.get(model) ?? []
      if (EXCLUDED_NAME_PATTERN.test(model)) return null
      if (
        endpointTypes.some((type) => VIDEO_ENDPOINT_TYPES.has(type)) ||
        VIDEO_NAME_PATTERN.test(model)
      ) {
        return 'video'
      }
      if (GENERIC_VIDEO_NAME_PATTERN.test(model)) return null
      if (
        endpointTypes.includes('image-generation') ||
        IMAGE_NAME_PATTERN.test(model)
      ) {
        return 'image'
      }
      if (
        endpointTypes.length === 0 ||
        endpointTypes.some((type) => CHAT_ENDPOINT_TYPES.has(type))
      ) {
        return 'chat'
      }
      if (endpointTypes.some((type) => EXCLUDED_ENDPOINT_TYPES.has(type))) {
        return null
      }
      return 'chat'
    },
    [modelEndpointTags]
  )

  const availableModels = useMemo(
    () =>
      (modelsQuery.data ?? []).filter((model) => classifyModel(model) !== null),
    [classifyModel, modelsQuery.data]
  )

  const exampleModel = useMemo(() => {
    const fallbackModel = pickDefaultModel(availableModels)
    return resolveSnippetModel(selectedModel, availableModels, fallbackModel)
  }, [availableModels, selectedModel])

  const apiEndpoint = useMemo(
    () =>
      resolveApiEndpoint(
        typeof status?.server_address === 'string'
          ? status.server_address
          : undefined,
        getCurrentOrigin()
      ),
    [status]
  )

  const snippetContext = useMemo<SnippetContext>(
    () => ({
      endpoint: apiEndpoint,
      model: exampleModel,
      kind: classifyModel(exampleModel) ?? 'chat',
      apiKey: selectedKey
        ? normalizeApiKey(selectedKey.key)
        : API_KEY_PLACEHOLDER,
    }),
    [apiEndpoint, classifyModel, exampleModel, selectedKey]
  )

  let keysError = ''
  if (apiKeysQuery.isError) {
    keysError =
      apiKeysQuery.error instanceof Error
        ? apiKeysQuery.error.message
        : t('Failed to load API keys')
  }

  let modelsError = ''
  if (modelsQuery.isError) {
    modelsError =
      modelsQuery.error instanceof Error
        ? modelsQuery.error.message
        : t('Failed to load models')
  }

  return (
    <>
      <IntegrationCards onSelect={setOpenIntegration} />
      <IntegrationDialog
        integrationId={openIntegration}
        onOpenChange={(open) => {
          if (!open) setOpenIntegration(null)
        }}
        keys={apiKeys}
        keysLoading={apiKeysQuery.isPending}
        keysError={keysError}
        onRetryKeys={() => void apiKeysQuery.refetch()}
        selectedKeyId={selectedKey?.id ?? null}
        resolvedKeys={resolvedKeys}
        loadingKeys={loadingKeys}
        resolveKey={resolveKey}
        onSelectKey={setSelectedKeyId}
        models={availableModels}
        modelsError={modelsError}
        onRetryModels={() => void modelsQuery.refetch()}
        selectedModel={exampleModel}
        onSelectModel={setSelectedModel}
        snippetContext={snippetContext}
      />
    </>
  )
}
