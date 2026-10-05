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

import { getModelAccessPricing } from '../lib/model-access-pricing'

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
  test('uses only the first dynamic token tier and ignores cache entries', () => {
    const pricing = getModelAccessPricing(
      pricingModel({
        billing_mode: 'tiered_expr',
        billing_expr:
          'len < 1000 ? tier("first", p * 3 + cr * 0.1 + c * 9) : tier("later", p * 8 + cr * 0.2 + c * 24)',
      })
    )

    expect(pricing.inputPrice).toMatch(/\$?3(?:\.0+)?$/)
    expect(pricing.outputPrice).toMatch(/\$?9(?:\.0+)?$/)
    expect(pricing.requestPrice).toBeNull()
    expect(pricing.isTimePricing).toBe(false)
  })

  test('uses the first tier in the active time period', () => {
    const model = pricingModel({
      billing_mode: 'tiered_expr',
      billing_expr:
        'weekday("Asia/Shanghai") >= 1 && weekday("Asia/Shanghai") <= 5 && hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 18 ? tier("peak", p * 3 + c * 9) : tier("off_peak", p * 1.5 + c * 4.5)',
    })

    const peak = getModelAccessPricing(
      model,
      new Date('2026-09-07T10:00:00+08:00')
    )
    const offPeak = getModelAccessPricing(
      model,
      new Date('2026-09-07T20:00:00+08:00')
    )

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
})
