/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { act, render, screen, within } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, describe, expect, test, vi } from 'vitest'

import type { PricingModel } from '@/features/pricing/types'

import type { ModelAccessModel } from '../../types'
import { ModelAccessList } from '../model-access-list'

const model: ModelAccessModel = {
  id: 'example-model',
  allowlist_match_key: 'example-model',
  vendor: { id: 1, name: 'Example' },
  supported_endpoint_types: ['openai'],
  availability_status: 'available',
}

const pricing: PricingModel = {
  id: 1,
  model_name: model.id,
  quota_type: 0,
  model_ratio: 2,
  completion_ratio: 4,
  cache_ratio: 0.1,
  create_cache_ratio: 1.25,
  enable_groups: ['default'],
}

type RenderListOptions = {
  defaultRatio?: number | null
  modelRatios?: Readonly<Record<string, number>>
  pricing?: PricingModel
  pricingPresentation?: 'compact' | 'comparison'
}

function renderList(options: RenderListOptions = {}) {
  return render(
    <ModelAccessList
      defaultRatio={
        options.defaultRatio === undefined ? 0.5 : options.defaultRatio
      }
      modelRatios={options.modelRatios ?? { [model.id]: 0.25 }}
      pricingModels={new Map([[model.id, options.pricing ?? pricing]])}
      pricingPresentation={options.pricingPresentation}
      models={[model]}
      scopeIsEmpty={false}
      onClearFilters={vi.fn()}
    />
  )
}

afterEach(async () => {
  await act(() => i18next.changeLanguage('en'))
})

describe('Available Models pricing presentation', () => {
  test('shows the exclusive effective price separately from the official price', () => {
    const { container } = renderList({ pricingPresentation: 'comparison' })
    const comparison = container.querySelector<HTMLElement>(
      '[data-pricing-presentation="comparison"]'
    )
    const effective = container.querySelector<HTMLElement>(
      '[data-price-kind="effective"]'
    )
    const official = container.querySelector<HTMLElement>(
      '[data-price-kind="official"]'
    )

    expect(comparison).toBeInTheDocument()
    if (!comparison || !effective || !official) {
      throw new Error('Expected effective and official pricing sections')
    }
    expect(effective).toHaveTextContent('Current effective price')
    expect(effective).toHaveTextContent('Per 1M tokens')
    expect(effective).toHaveTextContent(/Input\s*\$?1(?:\.0+)?/)
    expect(effective).toHaveTextContent(/Output\s*\$?4(?:\.0+)?/)
    expect(official).toHaveTextContent('Official base price')
    expect(official).toHaveTextContent('Per 1M tokens')
    expect(official).not.toHaveTextContent('Ratio 1×')
    expect(official).toHaveTextContent(/Input\s*\$?4(?:\.0+)?/)
    expect(official).toHaveTextContent(/Output\s*\$?16(?:\.0+)?/)
    expect(within(effective).queryByText('Exclusive ratio 0.25×')).toBeNull()
    expect(screen.getByText('Exclusive ratio 0.25×')).toBeVisible()
  })

  test('stacks comparison sections on narrow cards and keeps two primary columns', () => {
    const { container } = renderList({ pricingPresentation: 'comparison' })
    const comparison = container.querySelector<HTMLElement>(
      '[data-slot="comparison-pricing-panel"]'
    )
    if (!comparison) throw new Error('Expected comparison pricing panel')
    const sections = comparison.querySelectorAll('[data-price-kind]')
    const primaryPrices = comparison.querySelectorAll(
      '[data-slot="comparison-primary-price-metrics"]'
    )

    expect(comparison).toHaveClass('grid-cols-1', '@min-[35rem]:grid-cols-2')
    expect(sections[0]).toHaveAttribute('data-price-kind', 'effective')
    expect(sections[1]).toHaveAttribute('data-price-kind', 'official')
    expect(primaryPrices).toHaveLength(2)
    primaryPrices.forEach((priceRow) => {
      expect(priceRow).toHaveClass('grid', 'grid-cols-2')
    })
  })

  test('keeps comparison cache prices inside wrapping rows', () => {
    const { container } = renderList({ pricingPresentation: 'comparison' })
    const effective = container.querySelector<HTMLElement>(
      '[data-price-kind="effective"]'
    )
    const official = container.querySelector<HTMLElement>(
      '[data-price-kind="official"]'
    )
    const cacheRows = container.querySelectorAll<HTMLElement>(
      '[data-slot="comparison-cache-price-metrics"]'
    )

    if (!effective || !official) {
      throw new Error('Expected effective and official pricing sections')
    }
    expect(within(effective).getByText('Cache Read')).toBeVisible()
    expect(within(effective).getByText('Cache Write')).toBeVisible()
    expect(within(official).getByText('Cache Read')).toBeVisible()
    expect(within(official).getByText('Cache Write')).toBeVisible()
    expect(cacheRows).toHaveLength(2)
    cacheRows.forEach((cacheRow) => {
      expect(cacheRow).toHaveClass(
        'min-w-0',
        'max-w-full',
        'flex-wrap',
        'overflow-hidden'
      )
    })
  })

  test('shows the per-request unit at the center for request pricing', () => {
    const { container } = renderList({
      pricing: { ...pricing, quota_type: 1, model_price: 0.8 },
      pricingPresentation: 'comparison',
    })
    const effective = container.querySelector<HTMLElement>(
      '[data-price-kind="effective"]'
    )
    const official = container.querySelector<HTMLElement>(
      '[data-price-kind="official"]'
    )

    if (!effective || !official) {
      throw new Error('Expected effective and official pricing sections')
    }
    expect(effective).toHaveTextContent('Per Request')
    expect(official).toHaveTextContent('Per Request')
    expect(official).not.toHaveTextContent('Ratio 1×')
  })

  test('does not guess an effective price for the Auto group', () => {
    const { container } = renderList({
      defaultRatio: null,
      modelRatios: {},
      pricingPresentation: 'comparison',
    })
    const effective = container.querySelector<HTMLElement>(
      '[data-price-kind="effective"]'
    )
    const official = container.querySelector<HTMLElement>(
      '[data-price-kind="official"]'
    )

    if (!effective || !official) {
      throw new Error('Expected effective and official pricing sections')
    }
    expect(effective).toHaveTextContent(
      'Billed by the group selected during routing'
    )
    expect(effective).toHaveTextContent(/Input\s*—/)
    expect(effective).toHaveTextContent(/Output\s*—/)
    expect(official).toHaveTextContent(/Input\s*\$?4(?:\.0+)?/)
    expect(screen.getByText('Auto')).toBeVisible()
  })

  test('shows effective token and cache prices in the compact API-key preview', () => {
    const { container } = renderList()
    const compact = container.querySelector<HTMLElement>(
      '[data-pricing-presentation="compact"]'
    )

    expect(
      container.querySelector('[data-pricing-presentation="comparison"]')
    ).not.toBeInTheDocument()
    expect(compact).toBeInTheDocument()
    if (!compact) throw new Error('Expected compact effective pricing')
    expect(compact).toHaveTextContent('Current effective price')
    expect(compact).toHaveTextContent('Per 1M tokens')
    expect(compact).toHaveTextContent(/Input\s*\$?1(?:\.0+)?/)
    expect(compact).toHaveTextContent(/Output\s*\$?4(?:\.0+)?/)
    expect(compact).toHaveTextContent(/Cache Read\s*\$?0\.1(?:0+)?/)
    expect(compact).toHaveTextContent(/Cache Write\s*\$?1\.25(?:0+)?/)
    expect(screen.queryByText('Official base price')).not.toBeInTheDocument()
    expect(screen.getByText('Exclusive ratio 0.25×')).toBeVisible()
  })

  test('keeps the compact price panel at 220px with cache prices below the primary row', () => {
    const { container } = renderList()
    const compact = container.querySelector<HTMLElement>(
      '[data-slot="compact-pricing-panel"]'
    )
    if (!compact) throw new Error('Expected compact pricing panel')
    const primaryPrices = compact.querySelector<HTMLElement>(
      '[data-slot="compact-primary-price-metrics"]'
    )
    const cachePrices = compact.querySelector<HTMLElement>(
      '[data-slot="compact-cache-price-metrics"]'
    )
    if (!primaryPrices || !cachePrices) {
      throw new Error('Expected separate primary and cache price rows')
    }
    expect(compact).toHaveClass(
      '@min-[33rem]:w-[13.75rem]',
      '@min-[33rem]:shrink-0',
      '@min-[33rem]:basis-[13.75rem]'
    )
    expect(primaryPrices).toHaveClass('grid', 'grid-cols-2')
    expect(cachePrices).toHaveClass(
      'min-w-0',
      'max-w-full',
      'flex-wrap',
      'overflow-hidden'
    )
    expect(primaryPrices.nextElementSibling).toBe(cachePrices)
    expect(within(primaryPrices).getByText('Input')).toBeVisible()
    expect(within(primaryPrices).getByText('Output')).toBeVisible()
    expect(within(primaryPrices).queryByText('Cache Read')).toBeNull()
    expect(within(primaryPrices).queryByText('Cache Write')).toBeNull()
    expect(within(cachePrices).getByText('Cache Read')).toBeVisible()
    expect(within(cachePrices).getByText('Cache Write')).toBeVisible()
  })

  test('omits protocol markers from model cards', () => {
    renderList()

    expect(screen.queryByText('OpenAI Compatible')).not.toBeInTheDocument()
    expect(screen.queryByText('Endpoint not specified')).not.toBeInTheDocument()
  })

  test('uses the group ratio in the compact API-key preview', () => {
    const { container } = renderList({ modelRatios: {} })
    const compact = container.querySelector<HTMLElement>(
      '[data-pricing-presentation="compact"]'
    )

    if (!compact) throw new Error('Expected compact effective pricing')
    expect(compact).toHaveTextContent(/Input\s*\$?2(?:\.0+)?/)
    expect(compact).toHaveTextContent(/Output\s*\$?8(?:\.0+)?/)
    expect(screen.getByText('Group Ratio 0.5×')).toBeVisible()
    expect(screen.queryByText(/Exclusive ratio/)).not.toBeInTheDocument()
  })

  test('formats ratio labels with the active interface locale', async () => {
    if (!i18next.hasResourceBundle('fr', 'translation')) {
      i18next.addResourceBundle('fr', 'translation', {})
    }
    await act(() => i18next.changeLanguage('fr'))
    const { container } = renderList({ defaultRatio: 1234.5, modelRatios: {} })
    const formattedRatio = new Intl.NumberFormat('fr', {
      maximumFractionDigits: 6,
    }).format(1234.5)

    expect(container.textContent).toContain(`${formattedRatio}×`)
  })

  test('does not guess a compact price for the Auto group', () => {
    const { container } = renderList({ defaultRatio: null, modelRatios: {} })
    const compact = container.querySelector<HTMLElement>(
      '[data-pricing-presentation="compact"]'
    )

    if (!compact) throw new Error('Expected compact effective pricing')
    expect(compact).toHaveTextContent('Current effective price')
    expect(compact).toHaveTextContent(
      'Billed by the group selected during routing'
    )
    expect(compact).not.toHaveTextContent(/\$?4(?:\.0+)?/)
    expect(screen.getByText('Auto')).toBeVisible()
  })

  test('shows the effective per-request price in the compact preview', () => {
    const { container } = renderList({
      pricing: { ...pricing, quota_type: 1, model_price: 0.8 },
    })
    const compact = container.querySelector<HTMLElement>(
      '[data-pricing-presentation="compact"]'
    )

    if (!compact) throw new Error('Expected compact effective pricing')
    expect(compact).toHaveTextContent('Per Request')
    expect(compact).toHaveTextContent(/\$?0\.2(?:0+)?/)
    expect(compact).not.toHaveTextContent(/\$?0\.8(?:0+)?/)
    expect(within(compact).queryByText('Cache Read')).toBeNull()
    expect(within(compact).queryByText('Cache Write')).toBeNull()
  })
})
