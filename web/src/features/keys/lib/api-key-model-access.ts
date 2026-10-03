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
*/
import {
  getEffectiveTokenModels,
  getScopeModels,
  isCallableModel,
  resolveModelRatioContext,
  type ModelAccessModel,
  type ModelAccessScope,
  type UserModelAccess,
} from '@/features/available-models'

export type ApiKeyModelAccessState = {
  effectiveModels: ModelAccessModel[]
  modelRatios: Readonly<Record<string, number>>
  scope: ModelAccessScope | null
  scopeModels: ModelAccessModel[]
  defaultRatio: number | null
}

export function getApiKeyModelAccessState(
  access: UserModelAccess,
  group: string | null | undefined,
  modelLimits: readonly string[]
): ApiKeyModelAccessState {
  const scope =
    access.groups.find((candidate) => candidate.id === group) ?? null
  const scopeModels = getScopeModels(access, group)
  const effectiveModels = getEffectiveTokenModels(access, {
    group: group ?? null,
    model_limits_enabled: modelLimits.length > 0,
    model_limits: [...modelLimits],
  })
  const ratioContext = resolveModelRatioContext(access, group)

  return {
    effectiveModels,
    modelRatios: ratioContext.modelRatios,
    scope,
    scopeModels,
    defaultRatio: ratioContext.defaultRatio,
  }
}

export function getApiKeyModelAllowlistOptions(
  models: readonly ModelAccessModel[]
): Array<{ label: string; value: string }> {
  const options = new Map<string, { label: string; value: string }>()
  for (const model of models) {
    if (!isCallableModel(model)) continue
    if (!options.has(model.allowlist_match_key)) {
      options.set(model.allowlist_match_key, {
        label: model.id,
        value: model.allowlist_match_key,
      })
    }
  }
  return [...options.values()]
}
