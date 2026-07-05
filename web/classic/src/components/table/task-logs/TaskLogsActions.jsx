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
import { Button } from '@douyinfe/semi-ui';
import { SlidersHorizontal } from 'lucide-react';
import dayjs from 'dayjs';

const FMT = 'YYYY-MM-DD HH:mm:ss';

// 渲染在二级导航右侧(portal):快捷时间区间 + 筛选按钮。与使用日志一致。
const TaskLogsActions = ({
  formApi,
  refresh,
  activeTimeRange,
  setActiveTimeRange,
  setShowFilterModal,
  t,
}) => {
  const timeRanges = [
    { value: 'today', label: t('今日'), range: () => [dayjs().startOf('day'), dayjs()] },
    { value: '24h', label: t('近 24 小时'), range: () => [dayjs().subtract(24, 'hour'), dayjs()] },
    { value: '7d', label: t('近 7 天'), range: () => [dayjs().subtract(7, 'day'), dayjs()] },
    { value: '30d', label: t('近 30 天'), range: () => [dayjs().subtract(30, 'day'), dayjs()] },
  ];

  const handleRangeChange = (value) => {
    const opt = timeRanges.find((o) => o.value === value);
    if (!opt || !formApi) return;
    const [s, e] = opt.range();
    formApi.setValue('dateRange', [s.format(FMT), e.format(FMT)]);
    setActiveTimeRange(value);
    setTimeout(() => refresh && refresh(), 0);
  };

  return (
    <>
      <div className='log-time-range' role='group' aria-label={t('时间范围')}>
        {timeRanges.map((opt) => {
          const active = activeTimeRange === opt.value;
          return (
            <Button
              key={opt.value}
              size='small'
              theme={active ? 'solid' : 'borderless'}
              type={active ? 'primary' : 'tertiary'}
              onClick={() => handleRangeChange(opt.value)}
            >
              {opt.label}
            </Button>
          );
        })}
      </div>
      <Button
        type='tertiary'
        theme='light'
        size='small'
        icon={<SlidersHorizontal size={15} />}
        onClick={() => setShowFilterModal(true)}
      >
        {t('筛选')}
      </Button>
    </>
  );
};

export default TaskLogsActions;
