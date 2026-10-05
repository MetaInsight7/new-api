/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import {
  Check,
  Copy,
  Eye,
  EyeOff,
  Globe2,
  KeyRound,
  BookOpen,
  Server,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { useStatus } from '@/hooks/use-status'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { CopyButton } from '@/components/copy-button'
import { getApiKeys, fetchTokenKey } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'

type ApiEndpoint = {
  id: string
  label: string
  url: string
}

function normalizeBaseUrl(url: string): string {
  return url.trim().replace(/\/$/, '')
}

function normalizeKey(key: string): string {
  if (!key) return ''
  return key.startsWith('sk-') ? key : `sk-${key}`
}

export function WorkspaceApiAccess() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { status } = useStatus()
  const user = useAuthStore((state) => state.auth.user)
  const [selectedEndpointId, setSelectedEndpointId] = useState('')
  const [selectedKeyId, setSelectedKeyId] = useState('')
  const [resolvedKeys, setResolvedKeys] = useState<Record<string, string>>({})
  const [revealedKeyIds, setRevealedKeyIds] = useState<Set<string>>(
    () => new Set()
  )
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })

  const endpoints = useMemo<ApiEndpoint[]>(() => {
    const configured = Array.isArray(status?.api_info)
      ? status.api_info
          .filter((item) => item && typeof item.url === 'string' && item.url)
          .map((item, index) => ({
            id: String(item.id ?? `api-${index}`),
            label:
              item.route || item.description || `${t('API')} #${index + 1}`,
            url: normalizeBaseUrl(item.url),
          }))
      : []
    if (configured.length) return configured
    const serverAddress =
      typeof status?.server_address === 'string' && status.server_address
        ? status.server_address
        : window.location.origin
    return [
      {
        id: 'default',
        label: t('Default route'),
        url: normalizeBaseUrl(serverAddress),
      },
    ]
  }, [status, t])

  const keysQuery = useQuery({
    queryKey: ['workspace', 'api-keys', user?.id],
    queryFn: () => getApiKeys({ p: 1, size: 100 }),
    enabled: Boolean(user),
    staleTime: 60 * 1000,
  })
  const keys = useMemo(
    () => (keysQuery.data?.data?.items ?? []).filter((key) => key.status === 1),
    [keysQuery.data]
  )

  const activeEndpointId = endpoints.some(
    (endpoint) => endpoint.id === selectedEndpointId
  )
    ? selectedEndpointId
    : endpoints[0]?.id ?? ''
  let activeKeyId = ''
  if (keys.some((key) => String(key.id) === selectedKeyId)) {
    activeKeyId = selectedKeyId
  } else if (keys[0]) {
    activeKeyId = String(keys[0].id)
  }
  const endpoint = endpoints.find((item) => item.id === activeEndpointId)
  const selectedKey = keys.find((key) => String(key.id) === activeKeyId)
  const selectedKeyLabel = selectedKey
    ? selectedKey.name || `${t('API Key')} #${selectedKey.id}`
    : undefined
  const maskedKey = selectedKey ? normalizeKey(selectedKey.key) : ''
  const fullKey = selectedKey ? resolvedKeys[String(selectedKey.id)] : undefined
  const isConfigured = Boolean(selectedKey)
  const keysLoading = keysQuery.isLoading
  const isKeyRevealed = selectedKey
    ? revealedKeyIds.has(String(selectedKey.id))
    : false
  let statusClass =
    'bg-destructive/10 text-destructive inline-flex h-5 shrink-0 translate-y-1 items-center gap-1.5 rounded-full px-2 text-xs font-medium'
  let statusDotClass = 'bg-destructive size-1.5 rounded-full'
  let statusLabel = t('Not configured')
  if (isConfigured) {
    statusClass =
      'bg-success/10 text-success inline-flex h-5 shrink-0 translate-y-1 items-center gap-1.5 rounded-full px-2 text-xs font-medium'
    statusDotClass = 'bg-success size-1.5 rounded-full'
    statusLabel = t('Ready')
  } else if (keysLoading) {
    statusClass =
      'bg-muted text-muted-foreground inline-flex h-5 shrink-0 translate-y-1 items-center gap-1.5 rounded-full px-2 text-xs font-medium'
    statusDotClass = 'bg-muted-foreground size-1.5 rounded-full'
    statusLabel = t('Loading')
  }

  const revealKey = async () => {
    if (!selectedKey) return
    const keyId = String(selectedKey.id)
    if (isKeyRevealed) {
      setRevealedKeyIds((current) => {
        const next = new Set(current)
        next.delete(keyId)
        return next
      })
      return
    }
    if (fullKey) {
      setRevealedKeyIds((current) => new Set(current).add(keyId))
      return
    }
    try {
      const response = await fetchTokenKey(selectedKey.id)
      const fetchedKey = response.data?.key
      if (response.success && fetchedKey) {
        setResolvedKeys((current) => ({
          ...current,
          [keyId]: normalizeKey(fetchedKey),
        }))
        setRevealedKeyIds((current) => new Set(current).add(keyId))
      } else {
        toast.error(response.message || t('Failed to load API keys'))
      }
    } catch {
      toast.error(t('Failed to load API keys'))
    }
  }

  const copyKey = async () => {
    if (!selectedKey) {
      toast.info(t('Create an API key first'))
      return
    }

    let keyToCopy = fullKey
    if (!keyToCopy) {
      try {
        const response = await fetchTokenKey(selectedKey.id)
        if (!response.success || !response.data?.key) {
          toast.error(response.message || t('Failed to load API keys'))
          return
        }
        const fetchedKey = normalizeKey(response.data.key)
        keyToCopy = fetchedKey
        setResolvedKeys((current) => ({
          ...current,
          [String(selectedKey.id)]: fetchedKey,
        }))
      } catch {
        toast.error(t('Failed to load API keys'))
        return
      }
    }

    if (!keyToCopy) return
    const copied = await copyToClipboard(keyToCopy)
    if (copied) {
      toast.success(t('Copied!'))
    } else {
      toast.error(t('Failed to copy'))
    }
  }

  const openKeys = () => {
    navigate({ to: '/keys' })
  }

  const openDocs = () => {
    const docsUrl = typeof status?.docs_link === 'string'
      ? status.docs_link
      : ''
    if (docsUrl) window.open(docsUrl, '_blank', 'noopener,noreferrer')
    else toast.info(t('Documentation link is not configured'))
  }

  return (
    <section className='bg-card h-full overflow-hidden rounded-2xl border shadow-xs'>
      <div className='h-[65px] border-b px-4 py-3 sm:px-5'>
        <div className='flex items-center justify-between gap-3'>
          <div className='flex items-center gap-2 text-sm font-semibold'>
            <Server className='text-primary size-4' aria-hidden='true' />
            {t('API Access')}
          </div>
          <span
            className={statusClass}
          >
            <span className={statusDotClass} aria-hidden='true' />
            {statusLabel}
          </span>
        </div>
        <div className='text-muted-foreground mt-1 text-xs leading-4'>
          {endpoints.length} {t('API routes')} · {keys.length} {t('API Keys')}
        </div>
      </div>
      <div className='grid gap-3 p-4 sm:p-5'>
        <div className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
          <div className='bg-muted/20 grid min-w-0 gap-1.5 rounded-xl border p-3.5 sm:p-4'>
            <div className='text-muted-foreground flex items-center gap-1.5 text-xs font-medium'>
              <Globe2 className='text-primary size-3.5' aria-hidden='true' />
              {t('Current Route')}
            </div>
            <Select
              value={activeEndpointId}
              onValueChange={(value) => setSelectedEndpointId(value ?? '')}
            >
              <SelectTrigger className='!h-auto w-full !border-0 !bg-transparent !px-0 !py-0 !shadow-none !ring-0 focus-visible:!ring-0 [&_[data-slot=select-value]]:font-semibold'>
                <SelectValue placeholder={t('Select a route')}>
                  {endpoint?.label}
                </SelectValue>
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false} className='!w-[calc(var(--anchor-width)+2rem)] !rounded-lg !p-1 !shadow-lg'>
                {endpoints.map((item) => (
                  <SelectItem key={item.id} value={item.id} className='!rounded-md !py-2.5 !px-3'>
                    <span className='truncate font-medium'>{item.label}</span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className='bg-muted/20 grid min-w-0 gap-1.5 rounded-xl border p-3.5 sm:p-4'>
            <div className='text-muted-foreground flex items-center gap-1.5 text-xs font-medium'>
              <KeyRound className='text-primary size-3.5' aria-hidden='true' />
              {t('Current API Key')}
            </div>
            {keys.length ? (
              <Select
                value={activeKeyId}
                onValueChange={(value) => setSelectedKeyId(value ?? '')}
              >
                <SelectTrigger className='!h-auto w-full !border-0 !bg-transparent !px-0 !py-0 !shadow-none !ring-0 focus-visible:!ring-0 [&_[data-slot=select-value]]:font-semibold'>
                  <SelectValue placeholder={t('No available API keys')}>
                    {selectedKeyLabel}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false} className='!w-[calc(var(--anchor-width)+2rem)] !rounded-lg !p-1 !shadow-lg'>
                  {keys.map((key: ApiKey) => (
                    <SelectItem key={key.id} value={String(key.id)} className='!rounded-md !py-2.5 !px-3'>
                      <div className='flex flex-col'>
                        <span className='truncate font-medium'>
                          {key.name || `${t('API Key')} #${key.id}`}
                        </span>
                        {key.group && (
                          <span className='text-muted-foreground text-[11px]'>{key.group}</span>
                        )}
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <span className='text-muted-foreground text-sm'>
                {t('No available API keys')}
              </span>
            )}
          </div>
        </div>

        {selectedKey ? (
          <div className='bg-card grid gap-2 rounded-xl border p-3.5'>
            <div className='grid min-w-0 grid-cols-[3.5rem_minmax(0,1fr)] items-center gap-2'>
              <span className='text-xs font-medium'>{t('Base URL')}</span>
              <div className='bg-muted/50 flex h-9 min-w-0 items-center gap-1 rounded-lg border px-2'>
                <code className='min-w-0 flex-1 overflow-x-auto text-xs scrollbar-none'>{endpoint?.url ?? '-'}</code>
                <CopyButton
                  value={endpoint?.url ?? ''}
                  tooltip={t('Copy Base URL')}
                  size='icon'
                  className='size-6 rounded-md'
                  iconClassName='size-3'
                />
              </div>
            </div>
            <div className='grid min-w-0 grid-cols-[3.5rem_minmax(0,1fr)] items-center gap-2 border-t pt-2'>
              <span className='text-xs font-medium'>{t('API Key')}</span>
              <div className='bg-muted/50 flex h-9 min-w-0 items-center gap-1 rounded-lg border px-2'>
                <code className='min-w-0 flex-1 truncate text-xs'>
                  {(isKeyRevealed ? fullKey : maskedKey) || '-'}
                </code>
                <Button type='button' variant='ghost' size='icon-xs' onClick={() => void revealKey()} aria-label={t('Reveal API Key')}>
                  {isKeyRevealed ? <EyeOff className='size-3' /> : <Eye className='size-3' />}
                </Button>
                <Tooltip>
                  <TooltipTrigger render={<Button type='button' variant='ghost' size='icon-xs' onClick={() => void copyKey()} aria-label={t('Copy full API Key')} />}>
                    {copiedText === fullKey ? <Check className='text-success size-3' /> : <Copy className='size-3' />}
                  </TooltipTrigger>
                  <TooltipContent><p>{t('Copy full API Key')}</p></TooltipContent>
                </Tooltip>
              </div>
            </div>
          </div>
        ) : (
          <div className='bg-background text-muted-foreground flex min-h-28 items-center justify-center rounded-xl border p-3 text-sm'>
            {t('Configure an API key first')}
          </div>
        )}
        <div className='grid grid-cols-2 gap-3'>
          <Button type='button' onClick={openKeys} className='w-full'>
            <KeyRound data-icon='inline-start' />
            {t('API Keys')}
          </Button>
          <Button type='button' variant='outline' onClick={openDocs} className='w-full'>
            <BookOpen data-icon='inline-start' />
            {t('Usage Documentation')}
          </Button>
        </div>
      </div>
    </section>
  )
}
