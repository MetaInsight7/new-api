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
import { Wallet } from 'lucide-react';
import { renderQuota } from '../../helpers';

// 精简余额卡：占据原「服务可用性」的第三列卡槽。
// 补上页头只有状态点、缺具体余额数值的短板，并给出明确的充值入口。
const BalanceCard = ({ quota, usedQuota, balanceCaption, onTopUp, t }) => {
  const tone = balanceCaption?.tone || 'green';
  return (
    <div className='dashboard-balance-card'>
      <div className='dashboard-balance-card__amount'>{renderQuota(quota)}</div>
      <div className={`dashboard-balance-card__status is-${tone}`}>
        <span className='dashboard-balance-card__status-dot' />
        <span>{balanceCaption?.text || t('可正常调用')}</span>
      </div>
      {usedQuota != null && (
        <div className='dashboard-balance-card__meta'>
          {t('历史消耗')} {renderQuota(usedQuota)}
        </div>
      )}
      <Button
        block
        theme='solid'
        type='primary'
        icon={<Wallet size={15} />}
        className='dashboard-balance-card__cta'
        onClick={onTopUp}
      >
        {t('去充值')}
      </Button>
    </div>
  );
};

export default BalanceCard;
