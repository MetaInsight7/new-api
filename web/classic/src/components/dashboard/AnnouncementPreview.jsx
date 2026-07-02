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

import React, { useMemo } from 'react';
import { marked } from 'marked';
import { Megaphone } from 'lucide-react';
import { getRelativeTime } from '../../helpers';

// markdown -> 纯文本(用于摘要,避免在窄卡里渲染块级元素)
const toPlainText = (md) => {
  try {
    const html = marked.parse(md || '');
    if (typeof document === 'undefined') {
      return String(md || '')
        .replace(/[#>*_`~\-]/g, '')
        .trim();
    }
    const tmp = document.createElement('div');
    tmp.innerHTML = html;
    return (tmp.textContent || tmp.innerText || '').replace(/\s+/g, ' ').trim();
  } catch (_) {
    return String(md || '').trim();
  }
};

const formatAbsolute = (publishDate) => {
  if (!publishDate) return '';
  const d = new Date(publishDate);
  if (isNaN(d.getTime())) return String(publishDate);
  const pad = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

const AnnouncementPreview = ({ announcements = [], onViewAll, t }) => {
  const items = useMemo(
    () =>
      (announcements || []).slice(0, 20).map((a, idx) => ({
        key: `${a?.publishDate || ''}-${idx}`,
        text: toPlainText(a?.content),
        relative: getRelativeTime(a?.publishDate),
        absolute: formatAbsolute(a?.publishDate),
      })),
    [announcements],
  );

  if (items.length === 0) {
    return (
      <div className='dashboard-notice-empty'>
        <span className='dashboard-notice-empty__icon'>
          <Megaphone size={22} strokeWidth={1.75} />
        </span>
        <span>{t('暂无系统公告')}</span>
      </div>
    );
  }

  return (
    <div className='dashboard-notice-list'>
      <div className='dashboard-notice-timeline'>
        {items.map((item, idx) => (
          <button
            key={item.key}
            type='button'
            className={`dashboard-notice-item${idx === 0 ? ' is-new' : ''}`}
            onClick={onViewAll}
            title={item.text}
          >
            <span className='dashboard-notice-item__time' title={item.absolute}>
              {item.relative || item.absolute}
            </span>
            <span className='dashboard-notice-item__text'>{item.text}</span>
          </button>
        ))}
      </div>
    </div>
  );
};

export default AnnouncementPreview;
