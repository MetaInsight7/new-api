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
import {
  Typography,
  Card,
  Button,
  Banner,
  Skeleton,
  Form,
  Space,
  Row,
  Col,
  Spin,
  Tooltip,
  Tag,
  Tabs,
  TabPane,
} from '@douyinfe/semi-ui';
import { SiAlipay, SiWechat, SiStripe } from 'react-icons/si';
import {
  CreditCard,
  Coins,
  Wallet,
  BarChart2,
  TrendingUp,
  Receipt,
  Sparkles,
} from 'lucide-react';
import { IconGift } from '@douyinfe/semi-icons';
import { useMinimumLoadingTime } from '../../hooks/common/useMinimumLoadingTime';
import { useActualTheme } from '../../context/Theme';
import { getCurrencyConfig } from '../../helpers/render';
import SubscriptionPlansCard from './SubscriptionPlansCard';
import StatTile from '../common/ui/StatTile';

const { Text } = Typography;

const RechargeCard = ({
  t,
  enableOnlineTopUp,
  enableStripeTopUp,
  enableCreemTopUp,
  creemProducts,
  creemPreTopUp,
  presetAmounts,
  selectedPreset,
  selectPresetAmount,
  formatLargeNumber,
  priceRatio,
  topUpCount,
  minTopUp,
  renderQuotaWithAmount,
  getAmount,
  setTopUpCount,
  setSelectedPreset,
  renderAmount,
  amountLoading,
  payMethods,
  preTopUp,
  paymentLoading,
  payWay,
  redemptionCode,
  setRedemptionCode,
  topUp,
  isSubmitting,
  topUpLink,
  openTopUpLink,
  userState,
  renderQuota,
  statusLoading,
  topupInfo,
  onOpenHistory,
  enableWaffoTopUp,
  enableWaffoPancakeTopUp,
  subscriptionLoading = false,
  subscriptionPlans = [],
  billingPreference,
  onChangeBillingPreference,
  activeSubscriptions = [],
  allSubscriptions = [],
  reloadSubscriptionSelf,
  enableRedemption = true,
}) => {
  const onlineFormApiRef = useRef(null);
  const redeemFormApiRef = useRef(null);
  const initialTabSetRef = useRef(false);
  const showAmountSkeleton = useMinimumLoadingTime(amountLoading);
  const actualTheme = useActualTheme();
  const [activeTab, setActiveTab] = useState('topup');
  const shouldShowSubscription =
    !subscriptionLoading && subscriptionPlans.length > 0;
  const regularPayMethods = payMethods || [];

  useEffect(() => {
    if (initialTabSetRef.current) return;
    if (subscriptionLoading) return;
    setActiveTab(shouldShowSubscription ? 'subscription' : 'topup');
    initialTabSetRef.current = true;
  }, [shouldShowSubscription, subscriptionLoading]);

  useEffect(() => {
    if (!shouldShowSubscription && activeTab !== 'topup') {
      setActiveTab('topup');
    }
  }, [shouldShowSubscription, activeTab]);
  const topupContent = (
    <div style={{ width: '100%' }}>
      {/* 在线充值表单 */}
      {statusLoading ? (
          <div className='py-8 flex justify-center'>
            <Spin size='large' />
          </div>
        ) : enableOnlineTopUp ||
          enableStripeTopUp ||
          enableCreemTopUp ||
          enableWaffoTopUp ||
          enableWaffoPancakeTopUp ? (
          <Form
            getFormApi={(api) => (onlineFormApiRef.current = api)}
            initValues={{ topUpCount: topUpCount }}
          >
            <div className='space-y-4'>
              {(enableOnlineTopUp || enableStripeTopUp || enableWaffoTopUp) && (
                <Form.Slot
                  label={
                    <div className='flex items-center gap-2'>
                      <span>{t('选择充值额度')}</span>
                      {(() => {
                        const { symbol, rate, type } = getCurrencyConfig();
                        if (type === 'USD') return null;

                        return (
                          <span
                            style={{
                              color: 'var(--semi-color-text-2)',
                              fontSize: '12px',
                              fontWeight: 'normal',
                            }}
                          >
                            (1 $ = {rate.toFixed(2)} {symbol})
                          </span>
                        );
                      })()}
                    </div>
                  }
                >
                  <div className='wallet-rows'>
                    {presetAmounts.map((preset, index) => {
                      const discount =
                        preset.discount ||
                        topupInfo?.discount?.[preset.value] ||
                        1.0;
                      const originalPrice = preset.value * priceRatio;
                      const discountedPrice = originalPrice * discount;
                      const hasDiscount = discount < 1.0;
                      const actualPay = discountedPrice;
                      const save = originalPrice - discountedPrice;

                      // 根据当前货币类型换算显示金额和数量
                      const { symbol, rate, type } = getCurrencyConfig();
                      const statusStr = localStorage.getItem('status');
                      let usdRate = 7; // 默认CNY汇率
                      try {
                        if (statusStr) {
                          const s = JSON.parse(statusStr);
                          usdRate = s?.usd_exchange_rate || 7;
                        }
                      } catch (e) {}

                      let displayValue = preset.value; // 显示的数量
                      let displayActualPay = actualPay;
                      let displaySave = save;

                      if (type === 'USD') {
                        // 数量保持USD，价格从CNY转USD
                        displayActualPay = actualPay / usdRate;
                        displaySave = save / usdRate;
                      } else if (type === 'CNY') {
                        // 数量转CNY，价格已是CNY
                        displayValue = preset.value * usdRate;
                      } else if (type === 'CUSTOM') {
                        // 数量和价格都转自定义货币
                        displayValue = preset.value * rate;
                        displayActualPay = (actualPay / usdRate) * rate;
                        displaySave = (save / usdRate) * rate;
                      }

                      const active = selectedPreset === preset.value;
                      return (
                        <div
                          key={index}
                          className={`wallet-row ${active ? 'is-active' : ''}`}
                          onClick={() => {
                            selectPresetAmount(preset);
                            onlineFormApiRef.current?.setValue(
                              'topUpCount',
                              preset.value,
                            );
                          }}
                        >
                          <span className='wallet-row__amt'>
                            {formatLargeNumber(displayValue)}
                            <span className='u'>{symbol}</span>
                          </span>
                          <div className='wallet-row__meta'>
                            {hasDiscount && (
                              <span className='wallet-row__save'>
                                {t('折').includes('off')
                                  ? (
                                      (1 - parseFloat(discount)) *
                                      100
                                    ).toFixed(1)
                                  : (discount * 10).toFixed(1)}
                                {t('折')}
                              </span>
                            )}
                            <span className='wallet-row__pay wallet-mono'>
                              {symbol}
                              {displayActualPay.toFixed(2)}
                            </span>
                            <span className='wallet-row__chk' />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </Form.Slot>
              )}

              {(enableOnlineTopUp ||
                enableStripeTopUp ||
                enableWaffoTopUp ||
                enableWaffoPancakeTopUp) && (
                <div>
                  <div className='wallet-lab'>{t('或输入自定义数量')}</div>
                  <div className='wallet-qty'>
                    <Form.InputNumber
                      field='topUpCount'
                      label={null}
                      noLabel
                      size='large'
                      prefix={
                        <span
                          style={{
                            color: '#94a3b8',
                            fontSize: 13,
                            marginLeft: 10,
                            marginRight: 10,
                            whiteSpace: 'nowrap',
                            flex: '0 0 auto',
                          }}
                        >
                          {t('金额')}
                        </span>
                      }
                      disabled={
                        !enableOnlineTopUp &&
                        !enableStripeTopUp &&
                        !enableWaffoTopUp &&
                        !enableWaffoPancakeTopUp
                      }
                      placeholder={
                        t('充值数量，最低 ') + renderQuotaWithAmount(minTopUp)
                      }
                      value={topUpCount}
                      min={minTopUp}
                      max={999999999}
                      step={1}
                      precision={0}
                      onChange={async (value) => {
                        if (value && value >= 1) {
                          setTopUpCount(value);
                          setSelectedPreset(null);
                          await getAmount(value);
                        }
                      }}
                      onBlur={(e) => {
                        const value = parseInt(e.target.value);
                        if (!value || value < 1) {
                          setTopUpCount(1);
                          getAmount(1);
                        }
                      }}
                      formatter={(value) => (value ? `${value}` : '')}
                      parser={(value) =>
                        value ? parseInt(value.replace(/[^\d]/g, '')) : 0
                      }
                      style={{ width: '100%' }}
                    />
                  </div>
                </div>
              )}

              {/* Creem 充值区域 */}
              {enableCreemTopUp && creemProducts.length > 0 && (
                <Form.Slot label={t('Creem 充值')}>
                  <div className='grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3'>
                    {creemProducts.map((product, index) => (
                      <Card
                        key={index}
                        onClick={() => creemPreTopUp(product)}
                        className='cursor-pointer !rounded-2xl transition-all hover:shadow-md border-gray-200 hover:border-gray-300'
                        bodyStyle={{ textAlign: 'center', padding: '16px' }}
                      >
                        <div className='font-medium text-lg mb-2'>
                          {product.name}
                        </div>
                        <div className='text-sm text-gray-600 mb-2'>
                          {t('充值额度')}: {product.quota}
                        </div>
                        <div className='text-lg font-semibold text-blue-600'>
                          {product.currency === 'EUR' ? '€' : '$'}
                          {product.price}
                        </div>
                      </Card>
                    ))}
                  </div>
                </Form.Slot>
              )}

              {regularPayMethods.length > 0 && (
                <div className='wallet-pay-strip'>
                  <Space wrap>
                    {regularPayMethods.map((payMethod) => {
                      const minTopupVal = Number(payMethod.min_topup) || 0;
                      const isStripe = payMethod.type === 'stripe';
                      const isWaffo =
                        typeof payMethod.type === 'string' &&
                        payMethod.type.startsWith('waffo:');
                      const isWaffoPancake =
                        payMethod.type === 'waffo_pancake';
                      const disabled =
                        (!enableOnlineTopUp &&
                          !isStripe &&
                          !isWaffo &&
                          !isWaffoPancake) ||
                        (!enableStripeTopUp && isStripe) ||
                        (!enableWaffoTopUp && isWaffo) ||
                        (!enableWaffoPancakeTopUp && isWaffoPancake) ||
                        minTopupVal > Number(topUpCount || 0);

                      const buttonEl = (
                        <Button
                          key={payMethod.type}
                          theme='outline'
                          type='tertiary'
                          onClick={() => preTopUp(payMethod.type)}
                          disabled={disabled}
                          loading={
                            paymentLoading && payWay === payMethod.type
                          }
                          icon={
                            payMethod.type === 'alipay' ? (
                              <SiAlipay size={18} color='#1677FF' />
                            ) : payMethod.type === 'wxpay' ? (
                              <SiWechat size={18} color='#07C160' />
                            ) : payMethod.type === 'stripe' ? (
                              <SiStripe size={18} color='#635BFF' />
                            ) : payMethod.icon ? (
                              <img
                                src={payMethod.icon}
                                alt={payMethod.name}
                                style={{
                                  width: 18,
                                  height: 18,
                                  objectFit: 'contain',
                                }}
                              />
                            ) : payMethod.type === 'waffo_pancake' ? (
                              <img
                                src={
                                  actualTheme === 'dark'
                                    ? '/waffo-logo-dark.svg'
                                    : '/waffo-logo-light.svg'
                                }
                                alt='Waffo'
                                style={{
                                  width: 18,
                                  height: 18,
                                  objectFit: 'contain',
                                }}
                              />
                            ) : (
                              <CreditCard
                                size={18}
                                color={
                                  payMethod.color ||
                                  'var(--semi-color-text-2)'
                                }
                              />
                            )
                          }
                          className='!rounded-lg !px-4 !py-2'
                        >
                          {payMethod.name}
                        </Button>
                      );

                      return disabled &&
                        minTopupVal > Number(topUpCount || 0) ? (
                        <Tooltip
                          content={
                            t('此支付方式最低充值金额为') + ' ' + minTopupVal
                          }
                          key={payMethod.type}
                        >
                          {buttonEl}
                        </Tooltip>
                      ) : (
                        <React.Fragment key={payMethod.type}>
                          {buttonEl}
                        </React.Fragment>
                      );
                    })}
                  </Space>
                  <div className='wallet-due'>
                    <div className='wallet-due__k'>{t('应付金额')}</div>
                    <div className='wallet-due__v'>{renderAmount()}</div>
                  </div>
                </div>
              )}
            </div>
          </Form>
        ) : (
          <Banner
            type='info'
            description={t(
              '管理员未开启在线充值功能，请联系管理员开启或使用兑换码充值。',
            )}
            className='!rounded-xl'
            closeIcon={null}
          />
        )}
    </div>
  );

  return (
    <Card className='!rounded-2xl wallet-card'>
      <div className='wallet-phead'>
        <span className='wallet-phead__t'>{t('充值额度')}</span>
        <span className='wallet-phead__n'>Top up</span>
      </div>

      {shouldShowSubscription ? (
        <Tabs type='line' activeKey={activeTab} onChange={setActiveTab}>
          <TabPane
            tab={
              <div className='flex items-center gap-2'>
                <Sparkles size={16} />
                {t('订阅套餐')}
              </div>
            }
            itemKey='subscription'
          >
            <div className='py-2'>
              <SubscriptionPlansCard
                t={t}
                loading={subscriptionLoading}
                plans={subscriptionPlans}
                payMethods={payMethods}
                enableOnlineTopUp={enableOnlineTopUp}
                enableStripeTopUp={enableStripeTopUp}
                enableCreemTopUp={enableCreemTopUp}
                billingPreference={billingPreference}
                onChangeBillingPreference={onChangeBillingPreference}
                activeSubscriptions={activeSubscriptions}
                allSubscriptions={allSubscriptions}
                reloadSubscriptionSelf={reloadSubscriptionSelf}
                withCard={false}
              />
            </div>
          </TabPane>
          <TabPane
            tab={
              <div className='flex items-center gap-2'>
                <Wallet size={16} />
                {t('额度充值')}
              </div>
            }
            itemKey='topup'
          >
            <div className='py-2'>{topupContent}</div>
          </TabPane>
        </Tabs>
      ) : (
        topupContent
      )}
    </Card>
  );
};

export default RechargeCard;
