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

export const DEFAULT_LANGUAGE = 'zh-CN';

export const translationLoaders = {
  en: () => import('./locales/en.json'),
  'zh-TW': () => import('./locales/zh-TW.json'),
  fr: () => import('./locales/fr.json'),
  ru: () => import('./locales/ru.json'),
  ja: () => import('./locales/ja.json'),
  vi: () => import('./locales/vi.json'),
};

export async function loadTranslation(language, namespace = 'translation') {
  const loader = translationLoaders[language];
  if (!loader) {
    throw new Error(`Unsupported language: ${language}`);
  }

  const module = await loader();
  const resource = module.default || module;
  return resource[namespace] || resource.translation || resource;
}

export const dynamicTranslationBackend = {
  type: 'backend',
  init() {},
  read(language, namespace, callback) {
    loadTranslation(language, namespace)
      .then((resource) => callback(null, resource))
      .catch((error) => callback(error, false));
  },
};
