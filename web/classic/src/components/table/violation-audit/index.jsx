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

import React, { useEffect, useRef, useState } from 'react';
import { Tabs, TabPane, Button } from '@douyinfe/semi-ui';
import { ScrollText, BarChart3, ShieldAlert } from 'lucide-react';
import { SlidersHorizontal } from 'lucide-react';
import dayjs from 'dayjs';
import { useViolationAuditData } from '../../../hooks/violation-audit/useViolationAuditData';
import ViolationLogsView from './ViolationLogsView';
import ViolationMonitorView from './ViolationMonitorView';
import ViolationWordsView from './ViolationWordsView';
import './violation.css';

const FMT = 'YYYY-MM-DD HH:mm:ss';
const TIME_RANGES = [
  {
    value: 'today',
    label: '今日',
    range: () => [dayjs().startOf('day'), dayjs()],
  },
  {
    value: '24h',
    label: '近 24 小时',
    range: () => [dayjs().subtract(24, 'hour'), dayjs()],
  },
  {
    value: '7d',
    label: '近 7 天',
    range: () => [dayjs().subtract(7, 'day'), dayjs()],
  },
  {
    value: '30d',
    label: '近 30 天',
    range: () => [dayjs().subtract(30, 'day'), dayjs()],
  },
];

const ViolationAuditTable = () => {
  const data = useViolationAuditData();
  const { t } = data;
  const [tab, setTab] = useState('logs');
  const firstRun = useRef(true);

  // 时间范围改变 → 刷新当前 tab
  useEffect(() => {
    if (firstRun.current) {
      firstRun.current = false;
      return;
    }
    if (tab === 'logs') data.loadLogs(1, data.pageSize);
    else if (tab === 'monitor') data.loadStat();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data.filters.dateRange]);

  const tabLabel = (Icon, text) => (
    <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
      <Icon size={16} />
      {text}
    </span>
  );

  const pickQuickRange = (value) => {
    const opt = TIME_RANGES.find((o) => o.value === value);
    if (!opt) return;
    const [s, e] = opt.range();
    data.setActiveTimeRange(value);
    data.setFilters((prev) => ({
      ...prev,
      dateRange: [s.format(FMT), e.format(FMT)],
    }));
  };

  // 子 tab 右侧:快捷时间区间 + 筛选按钮(词库页不需要)
  const extra =
    tab === 'words' ? null : (
      <div className='va-actions'>
        <div
          className='log-time-range console-time-range'
          role='group'
          aria-label={t('时间范围')}
        >
          {TIME_RANGES.map((opt) => {
            const active = data.activeTimeRange === opt.value;
            return (
              <Button
                key={opt.value}
                size='small'
                theme={active ? 'solid' : 'borderless'}
                type={active ? 'primary' : 'tertiary'}
                onClick={() => pickQuickRange(opt.value)}
              >
                {t(opt.label)}
              </Button>
            );
          })}
        </div>
        {tab === 'logs' && (
          <Button
            type='tertiary'
            theme='light'
            size='small'
            icon={<SlidersHorizontal size={15} />}
            onClick={() => data.setShowFilterModal(true)}
          >
            {t('筛选')}
          </Button>
        )}
      </div>
    );

  return (
    <div className='va-wrap'>
      <Tabs
        type='line'
        activeKey={tab}
        onChange={setTab}
        tabBarExtraContent={extra}
      >
        <TabPane tab={tabLabel(ScrollText, t('审计记录'))} itemKey='logs'>
          {tab === 'logs' && (
            <div className='va-tab-body'>
              <ViolationLogsView data={data} />
            </div>
          )}
        </TabPane>
        <TabPane tab={tabLabel(BarChart3, t('监控'))} itemKey='monitor'>
          {tab === 'monitor' && (
            <div className='va-tab-body'>
              <ViolationMonitorView data={data} />
            </div>
          )}
        </TabPane>
        <TabPane tab={tabLabel(ShieldAlert, t('词库管理'))} itemKey='words'>
          {tab === 'words' && (
            <div className='va-tab-body'>
              <ViolationWordsView data={data} />
            </div>
          )}
        </TabPane>
      </Tabs>
    </div>
  );
};

export default ViolationAuditTable;
