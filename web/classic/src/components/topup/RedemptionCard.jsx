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
import { Typography, Card, Button, Form, Banner } from '@douyinfe/semi-ui';
import { IconGift } from '@douyinfe/semi-icons';

const { Text } = Typography;

// 兑换码充值 —— 账本风格扁平卡
const RedemptionCard = ({
  t,
  redemptionCode,
  setRedemptionCode,
  topUp,
  isSubmitting,
  topUpLink,
  openTopUpLink,
  enableRedemption = true,
}) => {
  return (
    <Card className='!rounded-2xl wallet-card'>
      <div className='wallet-phead'>
        <span className='wallet-phead__t'>{t('兑换码')}</span>
        <span className='wallet-phead__n'>Redeem</span>
      </div>

      {enableRedemption ? (
        <Form initValues={{ redemptionCode }}>
          <Form.Input
            field='redemptionCode'
            noLabel={true}
            placeholder={t('输入兑换码，立即到账')}
            value={redemptionCode}
            onChange={(value) => setRedemptionCode(value)}
            prefix={<IconGift />}
            suffix={
              <Button
                type='primary'
                theme='solid'
                onClick={topUp}
                loading={isSubmitting}
              >
                {t('兑换')}
              </Button>
            }
            showClear
            style={{ width: '100%' }}
            extraText={
              topUpLink && (
                <Text type='tertiary'>
                  {t('在找兑换码？')}
                  <Text
                    type='secondary'
                    underline
                    className='cursor-pointer'
                    onClick={openTopUpLink}
                  >
                    {t('购买兑换码')}
                  </Text>
                </Text>
              )
            }
          />
        </Form>
      ) : (
        <Banner
          type='warning'
          description={t('兑换码功能已禁用，管理员需先确认合规声明。')}
          closeIcon={null}
          className='!rounded-xl'
        />
      )}
    </Card>
  );
};

export default RedemptionCard;
