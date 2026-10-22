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
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

export function AdsSyncAlert(props: { error?: string }) {
  const { t } = useTranslation()
  if (!props.error) return null

  let title = t('Google Ads synchronization failed')
  if (props.error === 'ads spend synchronization failed') {
    title = t('Google Ads spend synchronization failed')
  } else if (props.error === 'ads daily synchronization failed') {
    title = t('Google Ads daily synchronization failed')
  } else if (props.error === 'ads spend and daily synchronization failed') {
    title = t('Google Ads spend and daily synchronization failed')
  }

  return (
    <Alert variant='destructive'>
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        {t('Showing the last successfully synced data.')}
      </AlertDescription>
    </Alert>
  )
}
