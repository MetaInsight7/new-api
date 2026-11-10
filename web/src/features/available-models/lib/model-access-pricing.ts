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
import type { PriceType, PricingModel } from '@/features/pricing/types'

export type ModelAccessPricing = {
  requestPrice: string | null
  inputPrice: string | null
  outputPrice: string | null
  cachePrices: ModelAccessCachePrice[]
  isTimePricing: boolean
}

type ModelAccessCachePrice = {
  key: string
  label: string
  price: string
}

type ModelAccessPricingOptions = {
  now?: Date
  ratio?: number
}

function hasConfiguredRatio(value: number | null | undefined): boolean {
  return value !== null && value !== undefined && Number.isFinite(Number(value))
}

export function getModelAccessPricing(
  model: PricingModel,
  options: ModelAccessPricingOptions = {}
): ModelAccessPricing {
  const ratio = options.ratio ?? 1
  const dynamic = getDynamicPricingSummary(model, {
    tokenUnit: DEFAULT_TOKEN_UNIT,
    showCurrencySymbol: true,
    groupRatioMultiplier: ratio,
    now: options.now,
  })

  if (dynamic) {
    const mixedBillingUnit =
      dynamic.isMixedBilling && dynamic.tier && !('unitPrices' in dynamic.tier)
        ? (dynamic.tier.billingUnit ?? 'token')
        : null
    const displayEntries =
      mixedBillingUnit !== null
        ? dynamic.entries.filter((entry) =>
            mixedBillingUnit === 'request'
              ? entry.unit === 'request' || entry.unit === 'image'
              : entry.unit === 'token'
          )
        : dynamic.entries
    const formattedPrice = (entry: (typeof dynamic.entries)[number]) =>
      entry.formattedRange ?? entry.formatted
    const requestEntry = displayEntries.find(
      (entry) => entry.unit === 'request' || entry.unit === 'image'
    )
    const inputEntry = displayEntries.find(
      (entry) => entry.field === 'inputPrice'
    )
    const outputEntry = displayEntries.find(
      (entry) => entry.field === 'outputPrice'
    )
    const cacheReadEntry = displayEntries.find(
      (entry) => entry.unit === 'token' && entry.field === 'cacheReadPrice'
    )
    const cacheWriteEntry = displayEntries.find(
      (entry) =>
        entry.unit === 'token' &&
        (entry.field === 'cacheCreatePrice' ||
          entry.field === 'cacheCreate1hPrice' ||
          entry.field === 'cacheCreate12hPrice')
    )
    const cachePrices: ModelAccessCachePrice[] = []
    if (cacheReadEntry) {
      cachePrices.push({
        key: cacheReadEntry.field,
        label: cacheReadEntry.shortLabel,
        price: formattedPrice(cacheReadEntry),
      })
    }
    if (cacheWriteEntry) {
      cachePrices.push({
        key: cacheWriteEntry.field,
        label: 'Cache Write',
        price: formattedPrice(cacheWriteEntry),
      })
    }

    return {
      requestPrice: requestEntry ? formattedPrice(requestEntry) : null,
      inputPrice: inputEntry ? formattedPrice(inputEntry) : null,
      outputPrice: outputEntry ? formattedPrice(outputEntry) : null,
      cachePrices,
      isTimePricing: dynamic.isTimePricing === true,
    }
  }

  const group = '__display__'
  const groupRatios = { [group]: ratio }
  const formatTokenPrice = (type: PriceType) =>
    formatGroupPrice(
      model,
      group,
      type,
      DEFAULT_TOKEN_UNIT,
      false,
      1,
      1,
      groupRatios
    )
  const cachePrices: ModelAccessCachePrice[] = []
  if (
    model.quota_type === QUOTA_TYPE_VALUES.TOKEN &&
    hasConfiguredRatio(model.cache_ratio)
  ) {
    cachePrices.push({
      key: 'cacheReadPrice',
      label: 'Cache Read',
      price: formatTokenPrice('cache'),
    })
  }
  if (
    model.quota_type === QUOTA_TYPE_VALUES.TOKEN &&
    hasConfiguredRatio(model.create_cache_ratio)
  ) {
    cachePrices.push({
      key: 'cacheCreatePrice',
      label: 'Cache Write',
      price: formatTokenPrice('create_cache'),
    })
  }

  return {
    requestPrice:
      model.quota_type === QUOTA_TYPE_VALUES.REQUEST
        ? formatFixedPrice(model, group, false, 1, 1, groupRatios)
        : null,
    inputPrice:
      model.quota_type === QUOTA_TYPE_VALUES.TOKEN
        ? formatTokenPrice('input')
        : null,
    outputPrice:
      model.quota_type === QUOTA_TYPE_VALUES.TOKEN
        ? formatTokenPrice('output')
        : null,
    cachePrices,
    isTimePricing: false,
  }
}
