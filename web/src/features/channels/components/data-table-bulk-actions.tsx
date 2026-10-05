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
import { useQueryClient } from '@tanstack/react-query'
import type { Table } from '@tanstack/react-table'
import { KeyRound, Power, PowerOff, Tag, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import {
  handleBatchDelete,
  handleBatchDisable,
  handleBatchEnable,
  handleBatchSetTag,
  handleBatchSetCodexFingerprintMode,
} from '../lib'
import type { Channel } from '../types'

interface DataTableBulkActionsProps<TData> {
  table: Table<TData>
}

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [showTagDialog, setShowTagDialog] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [tagValue, setTagValue] = useState('')
  const [showCodexFingerprintDialog, setShowCodexFingerprintDialog] =
    useState(false)
  const [codexFingerprintMode, setCodexFingerprintMode] = useState<
    'off' | 'device' | 'session' | 'full'
  >('off')
  const currentUser = useAuthStore((s) => s.auth.user)
  const canEditSensitive = hasPermission(
    currentUser,
    ADMIN_PERMISSION_RESOURCES.CHANNEL,
    ADMIN_PERMISSION_ACTIONS.SENSITIVE_WRITE
  )

  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedIds = selectedRows.reduce<number[]>((ids, row) => {
    const id = (row.original as Channel).id

    if (typeof id === 'number') {
      ids.push(id)
    }

    return ids
  }, [])

  const handleClearSelection = () => {
    table.resetRowSelection()
  }

  const handleEnableAll = () => {
    handleBatchEnable(selectedIds, queryClient, handleClearSelection)
  }

  const handleDisableAll = () => {
    handleBatchDisable(selectedIds, queryClient, handleClearSelection)
  }

  const handleDeleteAll = () => {
    if (!canEditSensitive) return
    handleBatchDelete(selectedIds, queryClient, () => {
      setShowDeleteConfirm(false)
      handleClearSelection()
    })
  }

  const handleSetTag = () => {
    handleBatchSetTag(selectedIds, tagValue || null, queryClient, () => {
      setShowTagDialog(false)
      setTagValue('')
      handleClearSelection()
    })
  }

  const handleSetCodexFingerprintMode = () => {
    if (!canEditSensitive) return
    handleBatchSetCodexFingerprintMode(
      selectedIds,
      codexFingerprintMode,
      queryClient,
      () => {
        setShowCodexFingerprintDialog(false)
        handleClearSelection()
      }
    )
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName='channel'>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleEnableAll}
                className='size-8'
                aria-label={t('Enable selected channels')}
                title={t('Enable selected channels')}
              />
            }
          >
            <Power />
            <span className='sr-only'>{t('Enable selected channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Enable selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowCodexFingerprintDialog(true)}
                disabled={!canEditSensitive}
                className='size-8'
                aria-label={t(
                  'Set Codex fingerprint mode for selected channels'
                )}
                title={
                  canEditSensitive
                    ? t('Set Codex fingerprint mode for selected channels')
                    : t('No permission to perform this action')
                }
              />
            }
          >
            <KeyRound />
            <span className='sr-only'>
              {t('Set Codex fingerprint mode for selected channels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>
              {canEditSensitive
                ? t('Set Codex fingerprint mode')
                : t('No permission to perform this action')}
            </p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleDisableAll}
                className='size-8'
                aria-label={t('Disable selected channels')}
                title={t('Disable selected channels')}
              />
            }
          >
            <PowerOff />
            <span className='sr-only'>{t('Disable selected channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Disable selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowTagDialog(true)}
                className='size-8'
                aria-label={t('Set tag for selected channels')}
                title={t('Set tag for selected channels')}
              />
            }
          >
            <Tag />
            <span className='sr-only'>
              {t('Set tag for selected channels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Set tag for selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                onClick={() => {
                  if (!canEditSensitive) return
                  setShowDeleteConfirm(true)
                }}
                aria-disabled={!canEditSensitive}
                className={cn(
                  'size-8',
                  !canEditSensitive && 'cursor-not-allowed opacity-50'
                )}
                aria-label={t('Delete selected channels')}
                title={
                  canEditSensitive
                    ? t('Delete selected channels')
                    : t('No permission to perform this action')
                }
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>{t('Delete selected channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>
              {canEditSensitive
                ? t('Delete selected channels')
                : t('No permission to perform this action')}
            </p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      {/* Set Tag Dialog */}
      <Dialog
        open={showTagDialog}
        onOpenChange={setShowTagDialog}
        title={t('Set Tag')}
        description={
          <>
            {t('Set a tag for')}
            {selectedIds.length}{' '}
            {t('selected channel(s). Leave empty to remove tag.')}
          </>
        }
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => {
                setShowTagDialog(false)
                setTagValue('')
              }}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleSetTag}>{t('Set Tag')}</Button>
          </>
        }
      >
        <div className='grid gap-4 py-4'>
          <div className='grid gap-2'>
            <Label htmlFor='tag'>{t('Tag')}</Label>
            <Input
              id='tag'
              placeholder={t('Enter tag name (optional)')}
              value={tagValue}
              onChange={(e) => setTagValue(e.target.value)}
            />
          </div>
        </div>
      </Dialog>

      <Dialog
        open={showCodexFingerprintDialog}
        onOpenChange={setShowCodexFingerprintDialog}
        title={t('Set Codex fingerprint mode')}
        description={t(
          'Only selected Codex channels are changed. Other channel types are ignored.'
        )}
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setShowCodexFingerprintDialog(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              onClick={handleSetCodexFingerprintMode}
              disabled={!canEditSensitive}
            >
              {t('Save')}
            </Button>
          </>
        }
      >
        <div className='grid gap-2 py-4'>
          <Label htmlFor='codex-fingerprint-mode'>{t('Mode')}</Label>
          <Select
            value={codexFingerprintMode}
            onValueChange={(value) => {
              if (
                value === 'off' ||
                value === 'device' ||
                value === 'session' ||
                value === 'full'
              ) {
                setCodexFingerprintMode(value)
              }
            }}
          >
            <SelectTrigger id='codex-fingerprint-mode'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='off'>{t('Off')}</SelectItem>
              <SelectItem value='device'>{t('Device')}</SelectItem>
              <SelectItem value='session'>{t('Session')}</SelectItem>
              <SelectItem value='full'>{t('Full')}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={showDeleteConfirm}
        onOpenChange={setShowDeleteConfirm}
        title={t('Delete Channels?')}
        description={
          <>
            {t('Are you sure you want to delete')}
            {selectedIds.length}{' '}
            {t('channel(s)? This action cannot be undone.')}
          </>
        }
        contentHeight='auto'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setShowDeleteConfirm(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={handleDeleteAll}
              disabled={!canEditSensitive}
            >
              {t('Delete')}
            </Button>
          </>
        }
      >
        {' '}
      </Dialog>
    </>
  )
}
