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
import { Badge, Button } from '@douyinfe/semi-ui';
import {
  Bell,
  Globe,
  RefreshCw,
  Search,
  Users,
  Server,
  ScrollText,
  Wallet,
  KeyRound,
} from 'lucide-react';

// quickActions 的 icon 字符串 → Lucide 图标(对齐 DMIT：操作按钮带图标)
const ACTION_ICONS = {
  users: Users,
  channels: Server,
  logs: ScrollText,
  topup: Wallet,
  keys: KeyRound,
};

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  activeTimeRange,
  timeRangeOptions,
  handleTimeRangeChange,
  showSearchModal,
  refresh,
  loading,
  balanceStatus,
  quickActions,
  unreadCount,
  onNoticeOpen,
  t,
}) => {
  return (
    <div className='dashboard-user-bar mb-4'>
      <div className='dashboard-user-bar__left'>
        <div className='dashboard-user-bar__icon'>
          <Globe size={22} />
        </div>
        <div className='dashboard-user-bar__info'>
          <div
            className='dashboard-user-bar__greeting'
            style={{
              opacity: greetingVisible ? 1 : 0,
              transition: 'opacity 1s ease-in-out',
            }}
          >
            {getGreeting}
          </div>
          <div className='dashboard-user-bar__status'>
            <span
              className={`dashboard-status-dot is-${balanceStatus?.tone || 'green'}`}
            />
            <span
              className={`dashboard-status-label is-${balanceStatus?.tone || 'green'}`}
            >
              {balanceStatus?.text || t('运行中')}
            </span>
          </div>
        </div>
      </div>

      <div className='dashboard-user-bar__right'>
        {/* 第一行：原样的时间范围 + 搜索 + 刷新 */}
        <div className='dashboard-user-bar__controls'>
          <div
            className='dashboard-time-range'
            role='group'
            aria-label={t('时间范围')}
          >
            {timeRangeOptions.map((option) => {
              const active = activeTimeRange === option.value;
              return (
                <Button
                  key={option.value}
                  size='small'
                  theme={active ? 'solid' : 'borderless'}
                  type={active ? 'primary' : 'tertiary'}
                  onClick={() => handleTimeRangeChange(option.value)}
                >
                  {option.label}
                </Button>
              );
            })}
          </div>
          <Button
            type='tertiary'
            theme='borderless'
            icon={<Search size={16} />}
            onClick={showSearchModal}
            aria-label={t('筛选')}
            title={t('筛选')}
          />
          <Button
            type='tertiary'
            theme='borderless'
            icon={<RefreshCw size={16} />}
            onClick={refresh}
            loading={loading}
            aria-label={t('刷新')}
            title={t('刷新')}
          />
        </div>

        {/* 第二行：快捷入口 */}
        <div className='dashboard-user-bar__actions'>
          {quickActions.map((action) => {
            const Icon = ACTION_ICONS[action.icon];
            return (
              <Button
                key={action.label}
                theme='outline'
                type='tertiary'
                icon={Icon ? <Icon size={15} /> : undefined}
                onClick={action.onClick}
                className='dashboard-header-action'
              >
                {action.label}
              </Button>
            );
          })}
          {onNoticeOpen && (
            <Badge count={unreadCount || 0} overflowCount={99} type='danger'>
              <Button
                theme='outline'
                type='danger'
                icon={<Bell size={14} />}
                onClick={onNoticeOpen}
                className='dashboard-header-action is-notice'
              >
                {t('系统公告')}
              </Button>
            </Badge>
          )}
        </div>
      </div>
    </div>
  );
};

export default DashboardHeader;
