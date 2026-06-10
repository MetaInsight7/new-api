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

import React from 'react';
import { Badge, Button, Card, Tag, Timeline, Empty } from '@douyinfe/semi-ui';
import { Bell } from 'lucide-react';
import { marked } from 'marked';
import {
  IllustrationConstruction,
  IllustrationConstructionDark,
} from '@douyinfe/semi-illustrations';
import ScrollableContainer from '../common/ui/ScrollableContainer';

const AnnouncementsPanel = ({
  announcementData,
  announcementLegendData,
  CARD_PROPS,
  ILLUSTRATION_SIZE,
  variant = 'default',
  unreadCount = 0,
  onNoticeOpen,
  t,
}) => {
  const isDashboard = variant === 'dashboard';
  const displayAnnouncements = isDashboard
    ? announcementData.slice(0, 20)
    : announcementData;
  const cardClassName = `shadow-sm !rounded-2xl ${
    isDashboard
      ? 'dashboard-announcement-card dashboard-announcement-card--dashboard'
      : 'lg:col-span-2'
  }`;

  const renderLegendDot = (legend) => (
    <div
      className='w-2 h-2 rounded-full'
      style={{
        backgroundColor:
          legend.color === 'grey'
            ? '#8b9aa7'
            : legend.color === 'blue'
              ? '#3b82f6'
              : legend.color === 'green'
                ? '#10b981'
                : legend.color === 'orange'
                  ? '#f59e0b'
                  : legend.color === 'red'
                    ? '#ef4444'
                    : '#8b9aa7',
      }}
    />
  );

  const noticeOpenButton = (
    <Button
      size='small'
      type='tertiary'
      theme='light'
      icon={<Bell size={14} />}
      className='dashboard-announcement-open-btn'
      onClick={onNoticeOpen}
    >
      {t('查看')}
    </Button>
  );

  return (
    <Card
      {...CARD_PROPS}
      className={cardClassName}
      title={
        <div
          className={
            isDashboard
              ? 'dashboard-announcement-titlebar'
              : 'flex flex-col lg:flex-row lg:items-center lg:justify-between gap-2 w-full'
          }
        >
          <div className='flex items-center gap-2 min-w-0'>
            <Bell size={16} />
            <span className='truncate'>{t('系统公告')}</span>
            <Tag color='white' shape='circle'>
              {isDashboard
                ? `${displayAnnouncements.length}/20`
                : t('显示最新20条')}
            </Tag>
          </div>
          {isDashboard ? (
            <span className='dashboard-announcement-open-wrap'>
              {unreadCount > 0 ? (
                <Badge
                  count={unreadCount}
                  type='danger'
                  overflowCount={99}
                  className='dashboard-announcement-notice-badge'
                >
                  {noticeOpenButton}
                </Badge>
              ) : (
                noticeOpenButton
              )}
            </span>
          ) : (
            <div className='flex flex-wrap gap-3 text-xs'>
              {announcementLegendData.map((legend, index) => (
                <div
                  key={index}
                  className='flex items-center gap-1'
                  title={legend.label}
                >
                  {renderLegendDot(legend)}
                  <span className='text-gray-600'>{legend.label}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      }
      bodyStyle={{ padding: 0 }}
    >
      <ScrollableContainer
        maxHeight={
          isDashboard ? 'var(--dashboard-announcement-body-height)' : '24rem'
        }
        className={isDashboard ? 'dashboard-announcement-scroll' : ''}
      >
        {displayAnnouncements.length > 0 ? (
          <Timeline
            mode='left'
            className={isDashboard ? 'dashboard-announcement-timeline' : ''}
          >
            {displayAnnouncements.map((item, idx) => {
              const htmlExtra = item.extra ? marked.parse(item.extra) : '';
              return (
                <Timeline.Item
                  key={idx}
                  type={item.type || 'default'}
                  time={
                    isDashboard
                      ? item.relative || item.time
                      : `${item.relative ? item.relative + ' ' : ''}${item.time}`
                  }
                  extra={
                    item.extra ? (
                      <div
                        className={
                          isDashboard
                            ? 'dashboard-announcement-extra text-xs text-gray-500'
                            : 'text-xs text-gray-500'
                        }
                        dangerouslySetInnerHTML={{ __html: htmlExtra }}
                      />
                    ) : null
                  }
                >
                  <div>
                    <div
                      className={
                        isDashboard ? 'dashboard-announcement-content' : ''
                      }
                      dangerouslySetInnerHTML={{
                        __html: marked.parse(item.content || ''),
                      }}
                    />
                  </div>
                </Timeline.Item>
              );
            })}
          </Timeline>
        ) : (
          <div className='flex justify-center items-center py-8'>
            <Empty
              image={<IllustrationConstruction style={ILLUSTRATION_SIZE} />}
              darkModeImage={
                <IllustrationConstructionDark style={ILLUSTRATION_SIZE} />
              }
              title={t('暂无系统公告')}
              description={t('请联系管理员在系统设置中配置公告信息')}
            />
          </div>
        )}
      </ScrollableContainer>
    </Card>
  );
};

export default AnnouncementsPanel;
