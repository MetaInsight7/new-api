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
import {
  DEFAULT_TOKEN_UNIT,
  QUOTA_TYPE_VALUES,
} from '@/features/pricing/constants'
import { getDynamicPricingSummary } from '@/features/pricing/lib/dynamic-price'
import {
  formatFixedPrice,
  formatGroupPrice,
} from '@/features/pricing/lib/price'
import type { PricingModel } from '@/features/pricing/types'

export type ModelAccessPricing = {
  requestPrice: string | null
  inputPrice: string | null
  outputPrice: string | null
  isTimePricing: boolean
}

export function getModelAccessPricing(
  model: PricingModel,
  now?: Date
): ModelAccessPricing {
  const dynamic = getDynamicPricingSummary(model, {
    tokenUnit: DEFAULT_TOKEN_UNIT,
    showCurrencySymbol: true,
    groupRatioMultiplier: 1,
    now,
  })

  if (dynamic) {
    return {
      requestPrice:
        dynamic.primaryEntries.find(
          (entry) => entry.unit === 'request' || entry.unit === 'image'
        )?.formatted ?? null,
      inputPrice:
        dynamic.primaryEntries.find((entry) => entry.field === 'inputPrice')
          ?.formatted ?? null,
      outputPrice:
        dynamic.primaryEntries.find((entry) => entry.field === 'outputPrice')
          ?.formatted ?? null,
      isTimePricing: dynamic.isTimePricing === true,
    }
  }

  return {
    requestPrice:
      model.quota_type === QUOTA_TYPE_VALUES.REQUEST
        ? formatFixedPrice(model, '__official__', false, 1, 1, {
            __official__: 1,
          })
        : null,
    inputPrice:
      model.quota_type === QUOTA_TYPE_VALUES.TOKEN
        ? formatGroupPrice(
            model,
            '__official__',
            'input',
            DEFAULT_TOKEN_UNIT,
            false,
            1,
            1,
            { __official__: 1 }
          )
        : null,
    outputPrice:
      model.quota_type === QUOTA_TYPE_VALUES.TOKEN
        ? formatGroupPrice(
            model,
            '__official__',
            'output',
            DEFAULT_TOKEN_UNIT,
            false,
            1,
            1,
            { __official__: 1 }
          )
        : null,
    isTimePricing: false,
  }
}
