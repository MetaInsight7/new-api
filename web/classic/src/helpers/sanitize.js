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

import createDOMPurify from 'dompurify';
import { marked } from 'marked';

const SANITIZE_CONFIG = Object.freeze({
  ALLOWED_TAGS: [
    'a',
    'abbr',
    'address',
    'article',
    'aside',
    'b',
    'bdi',
    'bdo',
    'blockquote',
    'br',
    'caption',
    'cite',
    'code',
    'col',
    'colgroup',
    'dd',
    'del',
    'details',
    'dfn',
    'div',
    'dl',
    'dt',
    'em',
    'figcaption',
    'figure',
    'footer',
    'h1',
    'h2',
    'h3',
    'h4',
    'h5',
    'h6',
    'header',
    'hgroup',
    'hr',
    'i',
    'img',
    'ins',
    'kbd',
    'li',
    'main',
    'mark',
    'nav',
    'ol',
    'p',
    'picture',
    'pre',
    'q',
    'rp',
    'rt',
    'ruby',
    's',
    'samp',
    'section',
    'small',
    'source',
    'span',
    'strong',
    'sub',
    'summary',
    'sup',
    'table',
    'tbody',
    'td',
    'tfoot',
    'th',
    'thead',
    'time',
    'tr',
    'u',
    'ul',
    'var',
    'wbr',
  ],
  ADD_ATTR: ['target'],
  // Configured documents may contain images, but media and remote embeds are
  // intentionally excluded from the generic Markdown surface.
  FORBID_TAGS: [
    'audio',
    'embed',
    'form',
    'iframe',
    'input',
    'object',
    'script',
    'style',
    'textarea',
  ],
  FORBID_ATTR: ['srcdoc', 'formaction'],
});

const hasDocument = () => typeof globalThis.document !== 'undefined';

let purifier;

const getPurifier = () => {
  if (purifier) return purifier;

  if (!hasDocument()) return null;

  purifier =
    typeof createDOMPurify.sanitize === 'function'
      ? createDOMPurify
      : createDOMPurify(globalThis.window);

  purifier.addHook('afterSanitizeAttributes', (node) => {
    if (node.tagName === 'A') {
      if (node.getAttribute('target') === '_blank') {
        node.setAttribute('rel', 'noopener noreferrer');
      }
      const href = node.getAttribute('href');
      if (href && !isSafeResourceUrl(href)) {
        node.removeAttribute('href');
      }
    }

    if (['IMG', 'AUDIO', 'VIDEO', 'SOURCE'].includes(node.tagName)) {
      const src = node.getAttribute('src');
      if (src && !isSafeResourceUrl(src)) {
        node.removeAttribute('src');
      }
    }
  });

  return purifier;
};

const isSafeResourceUrl = (value) => {
  const raw = String(value ?? '').trim();
  if (
    !raw ||
    raw.startsWith('/') ||
    raw.startsWith('./') ||
    raw.startsWith('../')
  ) {
    return true;
  }

  try {
    const url = new URL(raw, globalThis.location?.origin || 'http://localhost');
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch {
    return false;
  }
};

export const sanitizeHtml = (html = '') =>
  getPurifier()?.sanitize(String(html ?? ''), SANITIZE_CONFIG) || '';

export const renderSafeMarkdown = (markdown = '') =>
  sanitizeHtml(marked.parse(String(markdown ?? '')));

// Only allow web URLs when configured content is rendered as an external link.
// This keeps javascript:, data:, and file: URLs out of user-facing anchors.
export const isSafeExternalUrl = (value) => {
  try {
    const url = new URL(String(value ?? '').trim());
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch {
    return false;
  }
};

export const isSafeLinkUrl = (value) => {
  const raw = String(value ?? '').trim();
  if (
    !raw ||
    raw.startsWith('#') ||
    (raw.startsWith('/') && !raw.startsWith('//')) ||
    raw.startsWith('./') ||
    raw.startsWith('../')
  ) {
    return true;
  }
  if (raw.startsWith('//')) return false;
  if (!/^[a-z][a-z\d+.-]*:/i.test(raw)) return true;
  return isSafeExternalUrl(raw);
};

export const isSafeMediaUrl = (value) => {
  try {
    const url = new URL(
      String(value ?? '').trim(),
      globalThis.location?.origin || 'http://localhost',
    );
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch {
    return false;
  }
};

export const isSafeImageUrl = (value) => isSafeMediaUrl(value);
