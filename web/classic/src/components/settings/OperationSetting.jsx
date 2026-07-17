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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Card, Spin } from '@douyinfe/semi-ui';
import SettingsGeneral from '../../pages/Setting/Operation/SettingsGeneral';
import SettingsHeaderNavModules from '../../pages/Setting/Operation/SettingsHeaderNavModules';
import SettingsSidebarModulesAdmin from '../../pages/Setting/Operation/SettingsSidebarModulesAdmin';
import SettingsContentSecurity from './SettingsContentSecurity';
import SettingsLog from '../../pages/Setting/Operation/SettingsLog';
import SettingsMonitoring from '../../pages/Setting/Operation/SettingsMonitoring';
import SettingsCreditLimit from '../../pages/Setting/Operation/SettingsCreditLimit';
import SettingsCheckin from '../../pages/Setting/Operation/SettingsCheckin';
import { API, showError, toBoolean } from '../../helpers';

const INITIAL_OPTIONS = Object.freeze({
  /* 额度相关 */
  QuotaForNewUser: 0,
  PreConsumedQuota: 0,
  QuotaForInviter: 0,
  QuotaForInvitee: 0,
  'quota_setting.enable_free_model_pre_consume': true,

  /* 通用设置 */
  TopUpLink: '',
  'general_setting.docs_link': '',
  'general_setting.custom_currency_symbol': '¤',
  'general_setting.custom_currency_exchange_rate': '',
  QuotaPerUnit: 0,
  USDExchangeRate: 0,
  RetryTimes: 0,
  'general_setting.quota_display_type': 'USD',
  DisplayInCurrencyEnabled: true,
  DisplayTokenStatEnabled: false,
  DefaultCollapseSidebar: false,
  DemoSiteEnabled: false,
  SelfUseModeEnabled: false,

  /* 顶栏模块管理 */
  HeaderNavModules: '',

  /* 左侧边栏模块管理（管理员） */
  SidebarModulesAdmin: '',

  /* 敏感词设置 */
  CheckSensitiveEnabled: false,
  CheckSensitiveOnPromptEnabled: false,
  SensitiveWords: '',
  ViolationAuditEnabled: false,

  /* 支付设置 */
  'payment_setting.compliance_confirmed': false,

  /* 日志设置 */
  LogConsumeEnabled: false,
  ErrorLogEnabled: false,

  /* 监控设置 */
  ChannelDisableThreshold: 0,
  QuotaRemindThreshold: 0,
  AutomaticDisableChannelEnabled: false,
  AutomaticEnableChannelEnabled: false,
  AutomaticDisableKeywords: '',
  AutomaticDisableStatusCodes: '401',
  AutomaticRetryStatusCodes:
    '100-199,300-399,401-407,409-499,500-503,505-523,525-599',
  'monitor_setting.auto_test_channel_enabled': false,
  'monitor_setting.auto_test_channel_minutes': 10,

  /* 签到设置 */
  'checkin_setting.enabled': false,
  'checkin_setting.min_quota': 1000,
  'checkin_setting.max_quota': 10000,

  /* 令牌设置 */
  'token_setting.max_user_tokens': 1000,
});

const OperationSetting = () => {
  const [inputs, setInputs] = useState(() => ({ ...INITIAL_OPTIONS }));
  const [loading, setLoading] = useState(false);
  const mountedRef = useRef(true);
  const requestSeqRef = useRef(0);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      requestSeqRef.current += 1;
    };
  }, []);

  const getOptions = useCallback(async (requestSeq) => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      const newInputs = { ...INITIAL_OPTIONS };
      (Array.isArray(data) ? data : []).forEach((item) => {
        if (Object.prototype.hasOwnProperty.call(newInputs, item.key)) {
          if (typeof INITIAL_OPTIONS[item.key] === 'boolean') {
            newInputs[item.key] = toBoolean(item.value);
          } else {
            newInputs[item.key] = item.value;
          }
        }
      });
      if (mountedRef.current && requestSeq === requestSeqRef.current) {
        setInputs(newInputs);
      }
    } else if (mountedRef.current && requestSeq === requestSeqRef.current) {
      showError(message);
    }
  }, []);

  const onRefresh = useCallback(async () => {
    const requestSeq = ++requestSeqRef.current;
    try {
      if (mountedRef.current) setLoading(true);
      await getOptions(requestSeq);
    } catch (error) {
      if (mountedRef.current && requestSeq === requestSeqRef.current) {
        showError('刷新失败');
      }
    } finally {
      if (mountedRef.current && requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [getOptions]);

  useEffect(() => {
    onRefresh();
  }, [onRefresh]);

  return (
    <>
      <Spin spinning={loading} size='large'>
        {/* 通用设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsGeneral options={inputs} refresh={onRefresh} />
        </Card>
        {/* 顶栏模块管理 */}
        <div style={{ marginTop: '10px' }}>
          <SettingsHeaderNavModules options={inputs} refresh={onRefresh} />
        </div>
        {/* 左侧边栏模块管理（管理员） */}
        <div style={{ marginTop: '10px' }}>
          <SettingsSidebarModulesAdmin options={inputs} refresh={onRefresh} />
        </div>
        {/* 屏蔽词过滤设置(已由「内容安全 / 违规审计」取代,保留组件不再渲染) */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsContentSecurity options={inputs} refresh={onRefresh} />
        </Card>
        {/* 日志设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsLog options={inputs} refresh={onRefresh} />
        </Card>
        {/* 监控设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsMonitoring options={inputs} refresh={onRefresh} />
        </Card>
        {/* 额度设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsCreditLimit options={inputs} refresh={onRefresh} />
        </Card>
        {/* 签到设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsCheckin options={inputs} refresh={onRefresh} />
        </Card>
      </Spin>
    </>
  );
};

export default OperationSetting;
