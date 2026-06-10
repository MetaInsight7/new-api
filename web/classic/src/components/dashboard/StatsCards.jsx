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
import { Card, Skeleton } from '@douyinfe/semi-ui';
import {
  Activity,
  Coins,
  DatabaseZap,
  UsersRound,
  WalletCards,
} from 'lucide-react';

const iconMap = {
  wallet: WalletCards,
  coins: Coins,
  activity: Activity,
  tokens: DatabaseZap,
  users: UsersRound,
};

const StatsCards = ({
  statsData,
  loading,
  CARD_PROPS,
  title,
  note,
  statusAction,
  statusTitle,
  t,
}) => {
  const balanceStatus = statsData[0] || {};
  const statusPillClass = `dashboard-health-pill is-${balanceStatus.statusTone || 'green'}${
    statusAction ? ' is-clickable' : ''
  }`;
  const statusPillContent = (
    <>
      <span />
      {balanceStatus.statusText}
    </>
  );

  return (
    <Card
      {...CARD_PROPS}
      className='dashboard-usage-card !rounded-2xl'
      bodyStyle={{ padding: 0 }}
    >
      <div className='dashboard-usage-card__body'>
        <div className='dashboard-usage-card__header'>
          <div>
            <div className='dashboard-usage-card__title'>
              {title || t('用量概览')}
            </div>
            <div className='dashboard-usage-card__note'>
              {note || t('当前范围统计，历史项展示账户累计口径')}
            </div>
          </div>
          {statusAction ? (
            <button
              type='button'
              className={statusPillClass}
              onClick={statusAction}
              title={statusTitle}
              aria-label={statusTitle}
            >
              {statusPillContent}
            </button>
          ) : (
            <div className={statusPillClass}>{statusPillContent}</div>
          )}
        </div>

        <div className='dashboard-metric-grid'>
          {statsData.map((item) => {
            const Icon = iconMap[item.icon] || Activity;
            const metricContent = (
              <>
                <div className={`dashboard-metric-card__icon is-${item.tone}`}>
                  <Icon size={18} />
                </div>
                <div className='dashboard-metric-card__content'>
                  <div className='dashboard-metric-card__label'>
                    {item.title}
                  </div>
                  <div className='dashboard-metric-card__value'>
                    <Skeleton
                      loading={loading}
                      active
                      placeholder={
                        <Skeleton.Title
                          style={{ width: 88, height: 28, margin: 0 }}
                        />
                      }
                    >
                      {item.value}
                    </Skeleton>
                  </div>
                  <div
                    className={`dashboard-metric-card__caption is-${item.captionTone || item.tone}`}
                    title={item.caption}
                  >
                    {item.caption}
                  </div>
                </div>
              </>
            );

            if (item.onClick) {
              return (
                <button
                  key={item.title}
                  type='button'
                  className='dashboard-metric-cell is-clickable'
                  onClick={item.onClick}
                  aria-label={item.actionLabel || item.caption || item.title}
                >
                  {metricContent}
                </button>
              );
            }

            return (
              <div key={item.title} className='dashboard-metric-cell'>
                {metricContent}
              </div>
            );
          })}
        </div>
      </div>
    </Card>
  );
};

export default StatsCards;

