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
import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { expect, test } from 'vitest'

import type { ChannelConcurrencyStatus } from '../../api'
import { ChannelConcurrencyStatusDisplay } from '../channel-concurrency-status'

const i18n = createInstance()
await i18n.init({
  lng: 'en',
  resources: { en: { translation: {} } },
  initAsync: false,
})

function status(
  active: number,
  overrides: Partial<ChannelConcurrencyStatus> = {}
): ChannelConcurrencyStatus {
  return {
    channel_id: 7,
    max_concurrency: 3,
    active,
    waiting: 0,
    cooling_down: false,
    ...overrides,
  }
}

function view(snapshot?: ChannelConcurrencyStatus) {
  return (
    <I18nextProvider i18n={i18n}>
      <ChannelConcurrencyStatusDisplay status={snapshot} />
    </I18nextProvider>
  )
}

test('uses neutral, warning, and danger states for channel load', () => {
  const rendered = render(view(status(0)))
  expect(
    screen.getByLabelText('Channel concurrency: 0 of 3 active')
  ).toHaveAttribute('data-concurrency-state', 'idle')
  expect(
    screen.getByText('0/3').closest('[data-slot="status-badge"]')
  ).toHaveClass('text-muted-foreground')

  rendered.rerender(view(status(2)))
  expect(
    screen.getByLabelText('Channel concurrency: 2 of 3 active')
  ).toHaveAttribute('data-concurrency-state', 'busy')
  expect(
    screen.getByText('2/3').closest('[data-slot="status-badge"]')
  ).toHaveClass('text-warning')

  rendered.rerender(view(status(3)))
  expect(
    screen.getByLabelText('Channel concurrency: 3 of 3 active')
  ).toHaveAttribute('data-concurrency-state', 'saturated')
  expect(
    screen.getByText('3/3').closest('[data-slot="status-badge"]')
  ).toHaveClass('text-destructive')
})

test('shows positive waiting and cooling state with accessible semantics', () => {
  render(view(status(3, { waiting: 2, cooling_down: true })))

  expect(
    screen.getByLabelText(
      'Channel concurrency: 3 of 3 active, 2 waiting, Cooling'
    )
  ).toBeInTheDocument()
  expect(screen.getByText('+2')).toBeInTheDocument()
  expect(
    screen.getByText('Cooling').closest('[data-slot="status-badge"]')
  ).toHaveClass('text-chart-1')
})

test('shows an unavailable placeholder when no snapshot exists', () => {
  render(view())

  expect(
    screen.getByLabelText('Channel concurrency unavailable')
  ).toHaveTextContent('-')
})
