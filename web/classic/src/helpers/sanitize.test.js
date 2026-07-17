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

import { JSDOM } from 'jsdom';
import {
  isSafeExternalUrl,
  isSafeLinkUrl,
  isSafeMediaUrl,
  renderSafeMarkdown,
  sanitizeHtml,
} from './sanitize';

const dom = new JSDOM('<!doctype html><html><body></body></html>');
globalThis.window = dom.window;
globalThis.document = dom.window.document;

describe('sanitize helpers', () => {
  test('removes executable and document-level HTML while preserving safe markup', () => {
    const clean = sanitizeHtml(`
      <p class="ok" style="color:red">保留</p>
      <img src="x" onerror="alert(1)">
      <a href="javascript:alert(1)" target="_blank">bad</a>
      <a href="https://example.com" target="_blank">good</a>
      <script>alert(1)</script>
      <style>body { display: none }</style>
      <iframe src="https://example.com"></iframe>
      <form><input></form>
    `);

    expect(clean).not.toMatch(/<script|<style|<iframe|<form/i);
    expect(clean).not.toMatch(/onerror|javascript:/i);
    expect(clean).toContain('class="ok"');
    expect(clean).toContain('style="color:red"');
    expect(clean).toContain('rel="noopener noreferrer"');
  });

  test('sanitizes raw HTML embedded in Markdown', () => {
    const clean = renderSafeMarkdown(
      '# 标题\n\n<img src="x" onerror="alert(1)">',
    );

    expect(clean).toMatch(/<h1(?: [^>]*)?>标题<\/h1>/);
    expect(clean).not.toContain('onerror');
  });

  test('only accepts http and https external URLs', () => {
    expect(isSafeExternalUrl('https://example.com/docs')).toBe(true);
    expect(isSafeExternalUrl('http://example.com/docs')).toBe(true);
    expect(isSafeExternalUrl('javascript:alert(1)')).toBe(false);
    expect(isSafeExternalUrl('data:text/html,<script>alert(1)</script>')).toBe(
      false,
    );
    expect(isSafeExternalUrl('not a URL')).toBe(false);
  });

  test('allows safe relative links but rejects executable schemes', () => {
    expect(isSafeLinkUrl('/docs')).toBe(true);
    expect(isSafeLinkUrl('#section')).toBe(true);
    expect(isSafeLinkUrl('docs/getting-started')).toBe(true);
    expect(isSafeLinkUrl('https://example.com/docs')).toBe(true);
    expect(isSafeLinkUrl('//evil.example/docs')).toBe(false);
    expect(isSafeLinkUrl('javascript:alert(1)')).toBe(false);
    expect(isSafeLinkUrl('data:text/html,test')).toBe(false);
  });

  test('only accepts http and https media URLs', () => {
    expect(isSafeMediaUrl('https://cdn.example.com/file.mp4')).toBe(true);
    expect(isSafeMediaUrl('javascript:alert(1)')).toBe(false);
    expect(isSafeMediaUrl('data:text/html,unsafe')).toBe(false);
  });
});
