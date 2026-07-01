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
import { Gauge, Globe } from 'lucide-react';
import { UPTIME_STATUS_MAP } from '../../constants/dashboard.constants';

// 服务可用性侧栏：从主卡挪到图表右侧，做成窄侧栏（主图 + 状态 rail 的经典排布）
const AvailabilityRail = ({ monitors, legend, t }) => (
  <aside className='dashboard-availability-rail'>
    <div className='dashboard-availability-rail__header'>
      <Gauge size={16} />
      <span>{t('服务可用性')}</span>
    </div>
    <div className='dashboard-availability-rail__body'>
      {monitors.length > 0 ? (
        <>
          <div className='dashboard-availability-list'>
            {monitors.map((monitor, idx) => {
              const pct = Math.min((monitor.uptime || 0) * 100, 100);
              const statusInfo =
                UPTIME_STATUS_MAP[monitor.status] || UPTIME_STATUS_MAP[1];
              return (
                <div
                  key={monitor.name || idx}
                  className='dashboard-availability-item'
                >
                  <div className='dashboard-availability-item__header'>
                    <div className='dashboard-availability-item__name'>
                      <Globe size={14} />
                      <span>{monitor.name}</span>
                    </div>
                    <span
                      className='dashboard-availability-item__percent'
                      style={{ color: statusInfo.color }}
                    >
                      {pct.toFixed(2)}%
                    </span>
                  </div>
                  <div className='dashboard-availability-bar'>
                    <div
                      className='dashboard-availability-bar__fill'
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
          <div className='dashboard-availability-legend'>
            {legend.map((item) => (
              <div
                key={item.status}
                className='dashboard-availability-legend__item'
              >
                <span
                  className='dashboard-availability-legend__dot'
                  style={{ backgroundColor: item.color }}
                />
                <span>{item.label}</span>
              </div>
            ))}
          </div>
        </>
      ) : (
        <div className='dashboard-availability-empty'>
          <Gauge size={24} />
          <span>{t('暂无监控数据')}</span>
        </div>
      )}
    </div>
  </aside>
);

export default AvailabilityRail;
