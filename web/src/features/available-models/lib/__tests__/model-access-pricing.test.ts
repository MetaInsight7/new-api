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
import { describe, expect, test } from 'vitest'

import type { PricingModel } from '@/features/pricing/types'

import { getModelAccessPricing } from '../model-access-pricing'

function pricingModel(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'example-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 3,
    enable_groups: ['default'],
    ...overrides,
  }
}

describe('Available Models price display', () => {
  test('uses the first dynamic token tier and includes its cache prices', () => {
    const pricing = getModelAccessPricing(
      pricingModel({
        billing_mode: 'tiered_expr',
        billing_expr:
          'len < 1000 ? tier("first", p * 3 + cr * 0.1 + cc * 3.75 + c * 9) : tier("later", p * 8 + cr * 0.2 + cc * 10 + c * 24)',
      })
    )

    expect(pricing.inputPrice).toMatch(/\$?3(?:\.0+)?$/)
    expect(pricing.outputPrice).toMatch(/\$?9(?:\.0+)?$/)
    expect(pricing.requestPrice).toBeNull()
    expect(pricing.cachePrices).toHaveLength(2)
    expect(pricing.cachePrices[0]).toMatchObject({ label: 'Cache Read' })
    expect(pricing.cachePrices[0]?.price).toMatch(/\$?0\.1$/)
    expect(pricing.cachePrices[1]).toMatchObject({ label: 'Cache Write' })
    expect(pricing.cachePrices[1]?.price).toMatch(/\$?3\.75$/)
    expect(pricing.isTimePricing).toBe(false)
  })

  test('uses only the first configured tier for mixed token and request pricing', () => {
    const tokenFirst = getModelAccessPricing(
      pricingModel({
        billing_mode: 'tiered_expr',
        billing_expr:
          'len < 1000 ? tier("token", p * 3 + c * 9) : tier("request", fixed(0.8))',
      })
    )
    const requestFirst = getModelAccessPricing(
      pricingModel({
        billing_mode: 'tiered_expr',
        billing_expr:
          'len < 1000 ? tier("request", fixed(0.8)) : tier("token", p * 3 + c * 9)',
      })
    )

    expect(tokenFirst.requestPrice).toBeNull()
    expect(tokenFirst.inputPrice).toMatch(/\$?3(?:\.0+)?$/)
    expect(tokenFirst.outputPrice).toMatch(/\$?9(?:\.0+)?$/)
    expect(requestFirst.requestPrice).toMatch(/\$?0\.8(?:0+)?$/)
    expect(requestFirst.inputPrice).toBeNull()
    expect(requestFirst.outputPrice).toBeNull()
  })

  test('keeps a configured free cache-write price visible', () => {
    const pricing = getModelAccessPricing(
      pricingModel({
        model_name: 'glm-5.2',
        model_ratio: 37.5,
        completion_ratio: 1,
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("standard", p * 8 + cr * 2 + cc * 0 + c * 28)',
      })
    )

    expect(pricing.cachePrices).toContainEqual(
      expect.objectContaining({
        key: 'cacheCreatePrice',
        label: 'Cache Write',
        price: expect.stringMatching(/\$?0(?:\.0+)?$/),
      })
    )
  })

  test('uses the first tier in the active time period', () => {
    const model = pricingModel({
      billing_mode: 'tiered_expr',
      billing_expr:
        'weekday("Asia/Shanghai") >= 1 && weekday("Asia/Shanghai") <= 5 && hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 18 ? tier("peak", p * 3 + c * 9) : tier("off_peak", p * 1.5 + c * 4.5)',
    })

    const peak = getModelAccessPricing(model, {
      now: new Date('2026-09-07T10:00:00+08:00'),
    })
    const offPeak = getModelAccessPricing(model, {
      now: new Date('2026-09-07T20:00:00+08:00'),
    })

    expect(peak.inputPrice).toMatch(/\$?3(?:\.0+)?$/)
    expect(peak.outputPrice).toMatch(/\$?9(?:\.0+)?$/)
    expect(offPeak.inputPrice).toMatch(/\$?1\.5$/)
    expect(offPeak.outputPrice).toMatch(/\$?4\.5$/)
    expect(peak.isTimePricing).toBe(true)
    expect(offPeak.isTimePricing).toBe(true)
  })

  test('keeps legacy request and token pricing formats', () => {
    const request = getModelAccessPricing(
      pricingModel({ quota_type: 1, model_price: 0.25 })
    )
    const token = getModelAccessPricing(
      pricingModel({ model_ratio: 2, completion_ratio: 4 })
    )

    expect(request.requestPrice).toMatch(/0\.25/)
    expect(request.inputPrice).toBeNull()
    expect(token.inputPrice).toMatch(/4/)
    expect(token.outputPrice).toMatch(/16/)
  })

  test('applies the effective ratio to token, request, and cache prices', () => {
    const token = getModelAccessPricing(
      pricingModel({
        model_ratio: 2,
        completion_ratio: 4,
        cache_ratio: 0.1,
        create_cache_ratio: 1.25,
      }),
      { ratio: 0.25 }
    )
    const request = getModelAccessPricing(
      pricingModel({ quota_type: 1, model_price: 0.8 }),
      { ratio: 0.25 }
    )

    expect(token.inputPrice).toMatch(/1(?:\.0+)?$/)
    expect(token.outputPrice).toMatch(/4(?:\.0+)?$/)
    expect(token.cachePrices[0]?.price).toMatch(/0\.1$/)
    expect(token.cachePrices[1]?.price).toMatch(/1\.25$/)
    expect(request.requestPrice).toMatch(/0\.2$/)
  })

  test('includes request prices exposed by a provider billing expression', () => {
    const pricing = getModelAccessPricing(
      pricingModel({
        billing_plugin_variants: [
          {
            plugin_key: 'example',
            plugin_name: 'Example',
            billing_expr: 'tier("provider", fixed(0.8))',
            billing_usage_schema: {},
          },
        ],
      }),
      { ratio: 0.25 }
    )

    expect(pricing.requestPrice).toMatch(/0\.2$/)
  })
})
