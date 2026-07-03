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
import { Typography, Card, Button, Input } from '@douyinfe/semi-ui';
import { Copy, Zap } from 'lucide-react';

const { Text } = Typography;

const InvitationCard = ({
  t,
  userState,
  renderQuota,
  setOpenTransfer,
  affLink,
  handleAffLinkClick,
  complianceConfirmed = true,
}) => {
  return (
    <Card className='!rounded-2xl wallet-card'>
      <div className='wallet-phead'>
        <span className='wallet-phead__t'>{t('邀请返利')}</span>
        <span className='wallet-phead__n'>Referral</span>
      </div>

      {/* 收益：账本 kv 行 */}
      <div className='wallet-kv'>
        <span className='wallet-kv__k'>{t('待使用收益')}</span>
        <span className='wallet-kv__v'>
          {renderQuota(userState?.user?.aff_quota || 0)}
        </span>
      </div>
      <div className='wallet-kv'>
        <span className='wallet-kv__k'>{t('累计收益')}</span>
        <span className='wallet-kv__v'>
          {renderQuota(userState?.user?.aff_history_quota || 0)}
        </span>
      </div>
      <div className='wallet-kv'>
        <span className='wallet-kv__k'>{t('已邀请')}</span>
        <span className='wallet-kv__v'>{userState?.user?.aff_count || 0}</span>
      </div>

      {/* 邀请链接 */}
      <Input
        value={affLink}
        readonly
        className='!rounded-lg mt-4 wallet-invite'
        prefix={
          <span
            style={{
              color: '#94a3b8',
              fontSize: 13,
              marginLeft: 12,
              marginRight: 10,
              whiteSpace: 'nowrap',
              flex: '0 0 auto',
            }}
          >
            {t('邀请链接')}
          </span>
        }
        suffix={
          <Button
            type='primary'
            theme='solid'
            onClick={handleAffLinkClick}
            icon={<Copy size={14} />}
            className='!rounded-lg'
          >
            {t('复制')}
          </Button>
        }
      />

      {/* 划转到余额 */}
      <Button
        type='tertiary'
        theme='outline'
        block
        disabled={
          !complianceConfirmed ||
          !userState?.user?.aff_quota ||
          userState?.user?.aff_quota <= 0
        }
        onClick={() => setOpenTransfer(true)}
        icon={<Zap size={14} />}
        className='!rounded-lg mt-3'
      >
        {userState?.user?.aff_quota > 0
          ? t('划转 {{amount}} 至余额', {
              amount: renderQuota(userState?.user?.aff_quota),
            })
          : t('划转到余额')}
      </Button>

      {!complianceConfirmed && (
        <Text type='tertiary' className='text-xs block mt-3'>
          {t('邀请奖励划转已禁用，管理员需先确认合规声明。')}
        </Text>
      )}
    </Card>
  );
};

export default InvitationCard;
