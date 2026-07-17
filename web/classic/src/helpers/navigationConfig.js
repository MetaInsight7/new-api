/*
Copyright (C) 2025 QuantumNous

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

// Keep the public header configuration in one place. The option predates the
// object form, so normalisation here protects every consumer from malformed
// or partially upgraded settings.
export const DEFAULT_HEADER_NAV_MODULES = Object.freeze({
  home: true,
  console: true,
  pricing: { enabled: true, requireAuth: false },
  rankings: { enabled: true, requireAuth: false },
  docs: true,
  about: true,
});

const cloneDefaults = () => ({
  ...DEFAULT_HEADER_NAV_MODULES,
  pricing: { ...DEFAULT_HEADER_NAV_MODULES.pricing },
  rankings: { ...DEFAULT_HEADER_NAV_MODULES.rankings },
});

const normalizeGatedModule = (value, fallback) => {
  if (typeof value === 'boolean' || typeof value === 'string') {
    const enabled = value === true || value === 'true';
    return { enabled, requireAuth: false };
  }
  if (!value || typeof value !== 'object') return { ...fallback };
  return {
    enabled: value.enabled !== false && value.enabled !== 'false',
    requireAuth: value.requireAuth === true || value.requireAuth === 'true',
  };
};

const normalizeSimpleModule = (value, fallback) => {
  if (typeof value === 'boolean') return value;
  if (typeof value === 'string') return value === 'true';
  return fallback;
};

export const normalizeHeaderNavModules = (rawConfig) => {
  let parsed = rawConfig;
  if (typeof rawConfig === 'string') {
    try {
      parsed = rawConfig ? JSON.parse(rawConfig) : null;
    } catch {
      parsed = null;
    }
  }

  const defaults = cloneDefaults();
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return defaults;
  }

  return {
    ...defaults,
    ...parsed,
    home: normalizeSimpleModule(parsed.home, defaults.home),
    console: normalizeSimpleModule(parsed.console, defaults.console),
    docs: normalizeSimpleModule(parsed.docs, defaults.docs),
    about: normalizeSimpleModule(parsed.about, defaults.about),
    pricing: normalizeGatedModule(parsed.pricing, defaults.pricing),
    rankings: normalizeGatedModule(parsed.rankings, defaults.rankings),
  };
};

export const isHeaderModuleEnabled = (modules, key) => {
  const value = modules?.[key];
  return typeof value === 'object' ? value.enabled === true : value === true;
};

export const headerModuleRequiresAuth = (modules, key) =>
  modules?.[key]?.requireAuth === true;
