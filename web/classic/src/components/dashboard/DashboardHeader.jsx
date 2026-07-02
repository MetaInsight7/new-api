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
import { RefreshCw, Search } from 'lucide-react';

const DashboardHeader = ({
  getGreeting,
  greetingVisible,
  activeTimeRange,
  timeRangeOptions,
  handleTimeRangeChange,
  showSearchModal,
  refresh,
  loading,
  t,
}) => {
  return (
    <div className='dashboard-user-bar mb-4'>
      <div
        className='dashboard-user-bar__greeting'
        style={{
          opacity: greetingVisible ? 1 : 0,
          transition: 'opacity 1s ease-in-out',
        }}
      >
        {getGreeting}
      </div>

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
    </div>
  );
};

export default DashboardHeader;
