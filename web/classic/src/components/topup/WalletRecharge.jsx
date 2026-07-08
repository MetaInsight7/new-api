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

import React, { useEffect, useState } from 'react';
import { Banner, Spin } from '@douyinfe/semi-ui';
import { SiAlipay, SiWechat, SiStripe } from 'react-icons/si';
import {
  CreditCard,
  Users,
  Gift,
  Copy,
  Zap,
  ArrowRight,
  Receipt,
} from 'lucide-react';
import { getCurrencyConfig } from '../../helpers/render';
import { API, timestamp2string } from '../../helpers';

const payIcon = (type) => {
  if (type === 'alipay') return <SiAlipay size={16} color='#1677FF' />;
  if (type === 'wxpay') return <SiWechat size={16} color='#07C160' />;
  if (type === 'stripe') return <SiStripe size={16} color='#635BFF' />;
  return <CreditCard size={16} />;
};

// 最近充值：支付方式 / 状态映射
const PM_MAP = {
  alipay: '支付宝',
  wxpay: '微信',
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
};
const ST_MAP = {
  success: { txt: '成功', cls: '' },
  pending: { txt: '处理中', cls: 'pend' },
  failed: { txt: '失败', cls: 'fail' },
  expired: { txt: '已过期', cls: 'fail' },
};

// 钱包充值：左(蓝 hero + 金额 + 支付 + 结算) | 右(邀请返利 / 兑换码 / 最近充值)
const WalletRecharge = ({
  t,
  userState,
  renderQuota,
  statusLoading,
  anyOnlineEnabled,
  presetAmounts,
  selectedPreset,
  selectPresetAmount,
  formatLargeNumber,
  priceRatio,
  topUpCount,
  minTopUp,
  setTopUpCount,
  setSelectedPreset,
  getAmount,
  renderAmount,
  amount,
  amountLoading,
  payMethods,
  preTopUp,
  paymentLoading,
  topupInfo,
  onOpenHistory,
  // 右列
  affLink,
  handleAffLinkClick,
  setOpenTransfer,
  complianceConfirmed = true,
  redemptionCode,
  setRedemptionCode,
  topUp,
  isSubmitting,
  enableRedemption = true,
}) => {
  const user = userState?.user || {};
  const { symbol } = getCurrencyConfig();
  const methods = payMethods || [];
  const [selectedPay, setSelectedPay] = useState('');
  const activePay = selectedPay || methods[0]?.type || '';

  const [recent, setRecent] = useState([]);

  useEffect(() => {
    if (!selectedPay && methods[0]?.type) setSelectedPay(methods[0].type);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [methods.length]);

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const res = await API.get('/api/user/topup/self?p=1&page_size=4');
        const { success, data } = res.data;
        if (alive && success) setRecent(data?.items || []);
      } catch {
        // 静默失败：无记录时整卡不渲染
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  const presetPrice = (preset) => {
    const discount =
      preset.discount || topupInfo?.discount?.[preset.value] || 1.0;
    return preset.value * priceRatio * discount;
  };

  const affQuota = user.aff_quota || 0;
  const showCheckout = !statusLoading && anyOnlineEnabled && methods.length > 0;

  return (
    <div className='wg'>
      {/* ===== 左：充值 ===== */}
      <div className='wg__main'>
        <div className='wg__hero'>
          <div className='wg__heromain'>
            <div className='wg__k'>{t('账户余额')}</div>
            <div className='wg__bal'>{renderQuota(user.quota)}</div>
            <div className='wg__meta'>
              {t('历史消耗')} {renderQuota(user.used_quota)} · {t('累计请求')}{' '}
              {user.request_count || 0}
            </div>
          </div>
          <button className='wg__herobtn' onClick={onOpenHistory}>
            {t('账单明细')}
          </button>
        </div>

        <div className='wg__body'>
          {statusLoading ? (
            <div className='py-10 flex justify-center'>
              <Spin size='large' />
            </div>
          ) : anyOnlineEnabled ? (
            <>
              <div className='wg__sec'>{t('选择加注额度')}</div>
              <div className='wg__amts'>
                {presetAmounts.map((preset, i) => {
                  const active = selectedPreset === preset.value;
                  return (
                    <button
                      key={i}
                      className={`wg__amt${active ? ' is-active' : ''}`}
                      onClick={() => selectPresetAmount(preset)}
                    >
                      <span className='wg__amtv'>
                        {formatLargeNumber(preset.value)}
                        <span className='u'>$</span>
                      </span>
                      <span className='wg__amtp'>
                        {symbol}
                        {presetPrice(preset).toFixed(2)}
                      </span>
                    </button>
                  );
                })}
              </div>

              <div className='wg__lab'>{t('或输入自定义金额')}</div>
              <div className='wg__custom'>
                <span className='pfx'>$</span>
                <input
                  type='number'
                  min={minTopUp}
                  value={topUpCount}
                  placeholder={t('输入金额')}
                  onChange={(e) => {
                    const v = parseInt(e.target.value, 10);
                    if (v && v >= 1) {
                      setTopUpCount(v);
                      setSelectedPreset(null);
                      getAmount(v);
                    }
                  }}
                />
              </div>

              {methods.length > 0 && (
                <>
                  <div className='wg__lab'>{t('支付方式')}</div>
                  <div className='wg__pays'>
                    {methods.map((m) => (
                      <button
                        key={m.type}
                        className={`wg__pay${activePay === m.type ? ' is-active' : ''}`}
                        onClick={() => setSelectedPay(m.type)}
                      >
                        {payIcon(m.type)}
                        {m.name}
                      </button>
                    ))}
                  </div>
                </>
              )}
            </>
          ) : (
            <Banner
              type='info'
              closeIcon={null}
              className='!rounded-xl'
              description={t(
                '管理员未开启在线充值功能，请联系管理员开启或使用兑换码充值。',
              )}
            />
          )}
        </div>

        {showCheckout && (
          <div className='wg__checkout'>
            <div className='wg__due'>
              <span className='k'>{t('应付合计')}</span>
              <span className='v'>{amountLoading ? '···' : renderAmount()}</span>
            </div>
            <button
              className='wg__paybtn'
              disabled={!activePay || paymentLoading}
              onClick={() => activePay && preTopUp(activePay)}
            >
              {paymentLoading ? t('处理中…') : t('立即支付')}
            </button>
          </div>
        )}
      </div>

      {/* ===== 右：账户工具 ===== */}
      <div className='wg__side'>
        {/* 邀请返利 */}
        <div className='wg__card'>
          <div className='wg__chead'>
            <span className='wg__cico'>
              <Users size={15} />
            </span>
            <span className='wg__ct'>{t('邀请返利')}</span>
            <span className='wg__cn'>Referral</span>
          </div>
          <div className='wg__kv'>
            <span className='k'>{t('待使用收益')}</span>
            <span className='v'>{renderQuota(affQuota)}</span>
          </div>
          <div className='wg__kv'>
            <span className='k'>{t('累计收益')}</span>
            <span className='v'>{renderQuota(user.aff_history_quota || 0)}</span>
          </div>
          <div className='wg__kv'>
            <span className='k'>{t('已邀请')}</span>
            <span className='v'>{user.aff_count || 0}</span>
          </div>
          <div className='wg__inviteinp'>
            <div className='wg__field' title={affLink}>
              {affLink || '—'}
            </div>
            <button className='wg__sbtn ghost' onClick={handleAffLinkClick}>
              <Copy size={13} />
              {t('复制')}
            </button>
          </div>
          <button
            className='wg__transfer'
            disabled={!complianceConfirmed || affQuota <= 0}
            onClick={() => setOpenTransfer(true)}
          >
            <Zap size={13} />
            {affQuota > 0
              ? t('划转 {{amount}} 至余额', { amount: renderQuota(affQuota) })
              : t('划转到余额')}
          </button>
        </div>

        {/* 最近充值 + 兑换码：并排两列 */}
        <div className={`wg__siderow${recent.length > 0 ? '' : ' is-single'}`}>
          {/* 最近充值 —— 有记录才渲染 */}
          {recent.length > 0 && (
            <div className='wg__card'>
              <div className='wg__chead'>
                <span className='wg__cico'>
                  <Receipt size={15} />
                </span>
                <span className='wg__ct'>{t('最近充值')}</span>
                <button className='wg__cnbtn' onClick={onOpenHistory}>
                  {t('全部')}
                  <ArrowRight size={12} />
                </button>
              </div>
              {recent.map((r) => {
                const st = ST_MAP[r.status] || { txt: r.status, cls: '' };
                const ok = r.status === 'success';
                return (
                  <div className='wg__log' key={r.id}>
                    <span className='lt'>
                      {PM_MAP[r.payment_method] || r.payment_method || '—'}
                      <span className='ld'>
                        {timestamp2string(r.create_time).slice(5, 16)}
                      </span>
                    </span>
                    {ok ? (
                      <span className='lv'>+${Number(r.amount).toFixed(2)}</span>
                    ) : (
                      <span className={`lv ${st.cls}`}>{st.txt}</span>
                    )}
                  </div>
                );
              })}
            </div>
          )}

          {/* 兑换码 */}
          <div className='wg__card'>
            <div className='wg__chead'>
              <span className='wg__cico'>
                <Gift size={15} />
              </span>
              <span className='wg__ct'>{t('兑换码')}</span>
              <span className='wg__cn'>Redeem</span>
            </div>
            {enableRedemption ? (
              <div className='wg__redeeminp'>
                <input
                  className='wg__rinput'
                  value={redemptionCode}
                  placeholder={t('输入兑换码，立即到账')}
                  onChange={(e) => setRedemptionCode(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !isSubmitting) topUp();
                  }}
                />
                <button
                  className='wg__sbtn'
                  onClick={topUp}
                  disabled={isSubmitting}
                >
                  {isSubmitting ? t('兑换中…') : t('兑换')}
                </button>
              </div>
            ) : (
              <Banner
                type='warning'
                closeIcon={null}
                className='!rounded-xl'
                description={t('兑换码功能已禁用，管理员需先确认合规声明。')}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default WalletRecharge;
