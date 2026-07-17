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

import { describe, expect, test } from 'bun:test';
import { createInstance } from 'i18next';
import zhCNTranslation from './locales/zh-CN.json';
import { supportedLanguages } from './language';
import {
  DEFAULT_LANGUAGE,
  dynamicTranslationBackend,
  loadTranslation,
  translationLoaders,
} from './resources';

describe('translation resources', () => {
  test('provides a lazy loader for every non-default language', () => {
    const lazyLanguages = supportedLanguages.filter(
      (language) => language !== DEFAULT_LANGUAGE,
    );
    expect(Object.keys(translationLoaders).sort()).toEqual(
      lazyLanguages.sort(),
    );
  });

  for (const language of Object.keys(translationLoaders)) {
    test(`loads ${language} translations on demand`, async () => {
      const translation = await loadTranslation(language);
      expect(Object.keys(translation).length).toBeGreaterThan(3000);
      expect(typeof translation['登录']).toBe('string');
    });
  }

  test('rejects unsupported languages', async () => {
    expect(loadTranslation('unsupported')).rejects.toThrow(
      'Unsupported language',
    );
  });

  test('switches languages through the lazy backend', async () => {
    const i18n = createInstance();
    await i18n.use(dynamicTranslationBackend).init({
      lng: DEFAULT_LANGUAGE,
      fallbackLng: DEFAULT_LANGUAGE,
      supportedLngs: supportedLanguages,
      partialBundledLanguages: true,
      resources: {
        [DEFAULT_LANGUAGE]: zhCNTranslation,
      },
    });

    expect(i18n.t('登录')).toBe('登录');
    await i18n.changeLanguage('en');
    expect(i18n.t('登录')).toBe('Sign in');
    expect(i18n.hasResourceBundle('en', 'translation')).toBe(true);
    await i18n.changeLanguage(DEFAULT_LANGUAGE);
    expect(i18n.t('登录')).toBe('登录');
  });
});
