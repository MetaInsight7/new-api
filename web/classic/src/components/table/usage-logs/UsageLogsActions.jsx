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
import { Space, Skeleton } from '@douyinfe/semi-ui';
import { Coins, Activity, Gauge } from 'lucide-react';
import { renderQuota } from '../../../helpers';
import CompactModeToggle from '../../common/ui/CompactModeToggle';
import { useMinimumLoadingTime } from '../../../hooks/common/useMinimumLoadingTime';

const StatChip = ({ tone, icon: Icon, label, value }) => (
  <div className={`dmit-stat-chip is-${tone}`}>
    <span className='dmit-stat-chip__icon'>
      <Icon size={16} />
    </span>
    <span className='dmit-stat-chip__body'>
      <span className='dmit-stat-chip__label'>{label}</span>
      <span className='dmit-stat-chip__value'>{value}</span>
    </span>
  </div>
);

const LogsActions = ({
  stat,
  loadingStat,
  showStat,
  compactMode,
  setCompactMode,
  t,
}) => {
  const showSkeleton = useMinimumLoadingTime(loadingStat);
  const needSkeleton = !showStat || showSkeleton;

  const placeholder = (
    <Space>
      <Skeleton.Title style={{ width: 140, height: 44, borderRadius: 12 }} />
      <Skeleton.Title style={{ width: 96, height: 44, borderRadius: 12 }} />
      <Skeleton.Title style={{ width: 96, height: 44, borderRadius: 12 }} />
    </Space>
  );

  return (
    <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
      <Skeleton loading={needSkeleton} active placeholder={placeholder}>
        <div className='flex flex-wrap items-center gap-2'>
          <StatChip
            tone='blue'
            icon={Coins}
            label={t('消耗额度')}
            value={renderQuota(stat.quota)}
          />
          <StatChip tone='violet' icon={Activity} label='RPM' value={stat.rpm} />
          <StatChip tone='emerald' icon={Gauge} label='TPM' value={stat.tpm} />
        </div>
      </Skeleton>

      <CompactModeToggle
        compactMode={compactMode}
        setCompactMode={setCompactMode}
        t={t}
      />
    </div>
  );
};

export default LogsActions;
