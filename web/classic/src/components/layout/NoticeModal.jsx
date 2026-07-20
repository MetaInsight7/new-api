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

import React, { useEffect, useState, useContext, useMemo } from 'react';
import { Button, Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API } from '../../helpers/api';
import { renderSafeMarkdown } from '../../helpers/sanitize';
import { getRelativeTime } from '../../helpers/utils';
import { showError } from '../../helpers/notifications';
import { setStoredValue } from '../../helpers/siteStorage';
import { StatusContext } from '../../context/Status';
import { Bell, Megaphone } from 'lucide-react';
import './notice-dazi.css';

const NoticeModal = ({
  visible,
  onClose,
  isMobile,
  defaultTab = 'inApp',
  unreadKeys = [],
}) => {
  const { t } = useTranslation();
  const [noticeContent, setNoticeContent] = useState('');
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState(defaultTab);

  const [statusState] = useContext(StatusContext);

  const announcements = statusState?.status?.announcements || [];

  const unreadSet = useMemo(() => new Set(unreadKeys), [unreadKeys]);

  const getKeyForItem = (item) =>
    `${item?.publishDate || ''}-${(item?.content || '').slice(0, 30)}`;

  const processedAnnouncements = useMemo(() => {
    return (announcements || []).slice(0, 20).map((item) => {
      const pubDate = item?.publishDate ? new Date(item.publishDate) : null;
      const absoluteTime =
        pubDate && !isNaN(pubDate.getTime())
          ? `${pubDate.getFullYear()}-${String(pubDate.getMonth() + 1).padStart(2, '0')}-${String(pubDate.getDate()).padStart(2, '0')} ${String(pubDate.getHours()).padStart(2, '0')}:${String(pubDate.getMinutes()).padStart(2, '0')}`
          : item?.publishDate || '';
      return {
        key: getKeyForItem(item),
        type: item.type || 'default',
        time: absoluteTime,
        content: item.content,
        extra: item.extra,
        relative: getRelativeTime(item.publishDate),
        isUnread: unreadSet.has(getKeyForItem(item)),
      };
    });
  }, [announcements, unreadSet]);

  const handleCloseTodayNotice = () => {
    const today = new Date().toDateString();
    setStoredValue('notice_close_date', today);
    onClose();
  };

  const displayNotice = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/notice');

      const { success, message, data } = res.data;
      if (success) {
        if (data !== '') {
          const htmlNotice = renderSafeMarkdown(data);
          setNoticeContent(htmlNotice);
        } else {
          setNoticeContent('');
        }
      } else {
        showError(message);
      }
    } catch (error) {

      showError(error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible) {
      displayNotice();
    }
  }, [visible]);

  useEffect(() => {
    if (visible) {
      setActiveTab(defaultTab);
    }
  }, [defaultTab, visible]);

  const dotColor = (type) =>
    ({
      success: 'var(--nd-green)',
      warning: 'var(--nd-yellow)',
      error: 'var(--nd-coral)',
      ongoing: 'var(--nd-coral)',
    })[type] || 'var(--nd-blue)';

  const shortDate = (time) => {
    const m = /^\d{4}-(\d{2}-\d{2})/.exec(time || '');
    return m ? m[1] : (time || '').slice(0, 5);
  };

  const renderMarkdownNotice = () => {
    if (loading) {
      return <div className='nd-empty'>{t('加载中...')}</div>;
    }
    if (!noticeContent) {
      return <div className='nd-empty'>{t('暂无公告')}</div>;
    }
    return (
      <div
        className='nd-notice'
        dangerouslySetInnerHTML={{ __html: noticeContent }}
      />
    );
  };

  const renderAnnouncementTimeline = () => {
    if (processedAnnouncements.length === 0) {
      return <div className='nd-empty'>{t('暂无系统公告')}</div>;
    }
    return (
      <div className='nd-list'>
        {processedAnnouncements.map((item) => {
          const htmlContent = renderSafeMarkdown(item.content);
          const htmlExtra = item.extra ? renderSafeMarkdown(item.extra) : '';
          return (
            <div className='nd-item' key={item.key}>
              <div className='nd-date'>
                <b>{shortDate(item.time)}</b>
                {item.relative && <span>{item.relative}</span>}
              </div>
              <div className='nd-rail'>
                <span
                  className='nd-dot'
                  style={{ background: dotColor(item.type) }}
                />
                <span className='nd-line' />
              </div>
              <div className='nd-main'>
                {item.isUnread && <span className='nd-new'>NEW</span>}
                <div
                  className='nd-content'
                  dangerouslySetInnerHTML={{ __html: htmlContent }}
                />
                {htmlExtra && (
                  <div
                    className='nd-extra'
                    dangerouslySetInnerHTML={{ __html: htmlExtra }}
                  />
                )}
              </div>
            </div>
          );
        })}
      </div>
    );
  };

  const renderBody = () => {
    if (activeTab === 'inApp') {
      return renderMarkdownNotice();
    }
    return renderAnnouncementTimeline();
  };

  return (
    <Modal
      className='notice-dazi'
      title={
        <div>
          <div className='nd-head-row'>
            <span className='nd-ico'>
              <Megaphone size={20} />
            </span>
            <span className='nd-ttl'>
              {t('系统公告')}
              <span className='nd-sub'>Notices · {t('实时同步')}</span>
            </span>
          </div>
          <div className='nd-seg'>
            <button
              className={activeTab === 'inApp' ? 'on' : ''}
              onClick={() => setActiveTab('inApp')}
            >
              <Bell size={13} /> {t('通知')}
            </button>
            <button
              className={activeTab === 'system' ? 'on' : ''}
              onClick={() => setActiveTab('system')}
            >
              <Megaphone size={13} /> {t('系统公告')}
            </button>
          </div>
        </div>
      }
      visible={visible}
      onCancel={onClose}
      footer={
        <div className='nd-foot'>
          <span className='nd-hint'>
            {activeTab === 'system' && processedAnnouncements.length > 0
              ? t('共 {{n}} 条公告', { n: processedAnnouncements.length })
              : ''}
          </span>
          <div className='nd-btns'>
            <Button
              className='nd-btn nd-btn-ghost'
              onClick={handleCloseTodayNotice}
            >
              {t('今日关闭')}
            </Button>
            <Button className='nd-btn nd-btn-ink' onClick={onClose}>
              {t('知道了')}
            </Button>
          </div>
        </div>
      }
      size={isMobile ? 'full-width' : 'large'}
    >
      {renderBody()}
    </Modal>
  );
};

export default NoticeModal;
