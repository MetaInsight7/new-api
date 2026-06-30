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
import {
  Typography,
  Card,
  Button,
  Input,
  Badge,
  Space,
} from '@douyinfe/semi-ui';
import { Copy, Users, BarChart2, TrendingUp, Gift, Zap } from 'lucide-react';
import StatTile from '../common/ui/StatTile';

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
    <Card className='!rounded-2xl border-0'>
      {/* 卡片头部 */}
      <div className='flex items-center mb-4'>
        <span
          className='mr-3 inline-flex items-center justify-center w-9 h-9 rounded-lg'
          style={{ background: 'rgba(5,150,105,0.1)', color: '#059669' }}
        >
          <Gift size={18} />
        </span>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('邀请奖励')}
          </Typography.Text>
          <div className='text-xs text-gray-500'>
            {t('邀请好友获得额外奖励')}
          </div>
        </div>
      </div>

      {/* 收益展示区域 */}
      <Space vertical style={{ width: '100%' }}>
        {/* 统计数据统一卡片 */}
        <Card className='!rounded-xl w-full'>
          <div className='mb-5'>
            <div className='flex justify-between items-center mb-3'>
              <Text strong style={{ fontSize: 14 }}>
                {t('收益统计')}
              </Text>
              <Button
                type='primary'
                theme='solid'
                size='small'
                disabled={
                  !complianceConfirmed ||
                  !userState?.user?.aff_quota ||
                  userState?.user?.aff_quota <= 0
                }
                onClick={() => setOpenTransfer(true)}
                className='!rounded-lg'
              >
                <Zap size={12} className='mr-1' />
                {t('划转到余额')}
              </Button>
            </div>
            {!complianceConfirmed && (
              <Text type='tertiary' className='text-xs block mb-3'>
                {t('邀请奖励划转已禁用，管理员需先确认合规声明。')}
              </Text>
            )}

            <div className='grid grid-cols-3 gap-3'>
              <StatTile
                tone='emerald'
                icon={TrendingUp}
                label={t('待使用收益')}
                value={renderQuota(userState?.user?.aff_quota || 0)}
              />
              <StatTile
                tone='blue'
                icon={BarChart2}
                label={t('总收益')}
                value={renderQuota(userState?.user?.aff_history_quota || 0)}
              />
              <StatTile
                tone='violet'
                icon={Users}
                label={t('邀请人数')}
                value={userState?.user?.aff_count || 0}
              />
            </div>
          </div>

          {/* 邀请链接部分 */}
          <Input
            value={affLink}
            readonly
            className='!rounded-lg'
            prefix={t('邀请链接')}
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
        </Card>

        {/* 奖励说明 */}
        <Card
          className='!rounded-xl w-full'
          title={<Text type='tertiary'>{t('奖励说明')}</Text>}
        >
          <div className='space-y-3'>
            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('邀请好友注册，好友充值后您可获得相应奖励')}
              </Text>
            </div>

            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('通过划转功能将奖励额度转入到您的账户余额中')}
              </Text>
            </div>

            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('邀请的好友越多，获得的奖励越多')}
              </Text>
            </div>
          </div>
        </Card>
      </Space>
    </Card>
  );
};

export default InvitationCard;
