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

import React, { useEffect, useMemo, useState } from 'react';
import {
  API,
  getStoredValue,
  isSafeExternalUrl,
  setStoredValue,
  sanitizeHtml,
  showError,
} from '../../../helpers';
import { Empty, Card, Spin, Typography } from '@douyinfe/semi-ui';
const { Title } = Typography;
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import MarkdownRenderer from '../markdown/MarkdownRenderer';

// Check whether content contains HTML.
const isHtmlContent = (content) => {
  if (!content || typeof content !== 'string') return false;

  const htmlTagRegex = /<\/?[a-z][\s\S]*>/i;
  return htmlTagRegex.test(content);
};

// Parse an HTML document and keep only its safe body content. Global <style>
// blocks are intentionally discarded so configured documents cannot restyle
// the surrounding application shell.
const parseHtmlDocument = (html) => {
  const tempDiv = document.createElement('div');
  tempDiv.innerHTML = html;

  const bodyContent = tempDiv.querySelector('body');
  const content = sanitizeHtml(bodyContent ? bodyContent.innerHTML : html);

  return { content };
};

/**
 * 通用文档渲染组件
 * @param {string} apiEndpoint - API 接口地址
 * @param {string} title - 文档标题
 * @param {string} cacheKey - 本地存储缓存键
 * @param {string} emptyMessage - 空内容时的提示消息
 */
const DocumentRenderer = ({ apiEndpoint, title, cacheKey, emptyMessage }) => {
  const { t } = useTranslation();
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(true);

  const htmlPayload = useMemo(() => {
    if (!isHtmlContent(content)) {
      return { content: '' };
    }
    return parseHtmlDocument(content);
  }, [content]);

  useEffect(() => {
    let cancelled = false;

    const loadContent = async () => {
      let cachedContent = '';
      cachedContent = getStoredValue(cacheKey, '');

      if (cachedContent && !cancelled) {
        setContent(cachedContent);
        setLoading(false);
      }

      try {
        const res = await API.get(apiEndpoint);
        const { success, message, data } = res.data;
        if (success && data) {
          if (cancelled) return;
          setContent(data);
          setStoredValue(cacheKey, data);
        } else if (!cachedContent && !cancelled) {
          showError(message || emptyMessage);
          setContent('');
        }
      } catch (error) {
        if (!cachedContent && !cancelled) {
          showError(emptyMessage);
          setContent('');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    loadContent();

    return () => {
      cancelled = true;
    };
  }, [apiEndpoint, cacheKey, emptyMessage]);

  // 显示加载状态
  if (loading) {
    return (
      <div className='classic-page-fill flex justify-center items-center'>
        <Spin size='large' />
      </div>
    );
  }

  // 如果没有内容，显示空状态
  if (!content || content.trim() === '') {
    return (
      <div className='classic-page-fill flex justify-center items-center bg-gray-50'>
        <Empty
          title={t('管理员未设置' + title + '内容')}
          image={
            <IllustrationConstruction style={{ width: 150, height: 150 }} />
          }
          darkModeImage={
            <IllustrationConstructionDark style={{ width: 150, height: 150 }} />
          }
          className='p-8'
        />
      </div>
    );
  }

  // 如果是 URL，显示链接卡片
  if (isSafeExternalUrl(content)) {
    return (
      <div className='classic-page-fill flex justify-center items-center bg-gray-50 p-4'>
        <Card className='max-w-md w-full'>
          <div className='text-center'>
            <Title heading={4} className='mb-4'>
              {title}
            </Title>
            <p className='text-gray-600 mb-4'>
              {t('管理员设置了外部链接，点击下方按钮访问')}
            </p>
            <a
              href={content.trim()}
              target='_blank'
              rel='noopener noreferrer'
              title={content.trim()}
              aria-label={`${t('访问' + title)}: ${content.trim()}`}
              className='inline-block px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors'
            >
              {t('访问' + title)}
            </a>
          </div>
        </Card>
      </div>
    );
  }

  // 如果是 HTML 内容，直接渲染
  if (isHtmlContent(content)) {
    return (
      <div className='classic-page-fill bg-gray-50'>
        <div className='max-w-4xl mx-auto py-12 px-4 sm:px-6 lg:px-8'>
          <div className='bg-white rounded-lg shadow-sm p-8'>
            <Title heading={2} className='text-center mb-8'>
              {title}
            </Title>
            <div
              className='prose prose-lg max-w-none'
              dangerouslySetInnerHTML={{ __html: htmlPayload.content }}
            />
          </div>
        </div>
      </div>
    );
  }

  // 其他内容统一使用 Markdown 渲染器
  return (
    <div className='classic-page-fill bg-gray-50'>
      <div className='max-w-4xl mx-auto py-12 px-4 sm:px-6 lg:px-8'>
        <div className='bg-white rounded-lg shadow-sm p-8'>
          <Title heading={2} className='text-center mb-8'>
            {title}
          </Title>
          <div className='prose prose-lg max-w-none'>
            <MarkdownRenderer content={content} />
          </div>
        </div>
      </div>
    </div>
  );
};

export default DocumentRenderer;
