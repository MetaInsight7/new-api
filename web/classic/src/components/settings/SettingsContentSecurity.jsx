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
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Button, Form, Spin, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess, toBoolean } from '../../helpers';
import { useRequestLifecycle } from '../../hooks/common/useRequestLifecycle';

// 内容安全 / 违规审计:运营设置里只放一个总开关;
// 词库、审计记录、监控等具体配置都在「违规审计」页面。
export default function SettingsContentSecurity(props) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const refForm = useRef();
  const { beginRequest, isCurrentRequest } = useRequestLifecycle();

  useEffect(() => {
    const v = toBoolean(props.options?.ViolationAuditEnabled);
    setEnabled(v);
    refForm.current?.setValues({ ViolationAuditEnabled: v });
  }, [props.options]);

  const onSubmit = () => {
    const requestId = beginRequest('save');
    setLoading(true);
    API.put('/api/option/', {
      key: 'ViolationAuditEnabled',
      value: String(enabled),
    })
      .then(async (res) => {
        if (res?.data?.success) {
          if (!isCurrentRequest('save', requestId)) return;
          showSuccess(t('保存成功'));
          await props.refresh?.();
        } else if (isCurrentRequest('save', requestId)) {
          showError(res?.data?.message || t('保存失败，请重试'));
        }
      })
      .catch(() => {
        if (isCurrentRequest('save', requestId))
          showError(t('保存失败，请重试'));
      })
      .finally(() => {
        if (isCurrentRequest('save', requestId)) setLoading(false);
      });
  };

  return (
    <Spin spinning={loading}>
      <Form
        values={{ ViolationAuditEnabled: enabled }}
        getFormApi={(api) => (refForm.current = api)}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('内容安全 / 违规审计')}>
          <Form.Switch
            field={'ViolationAuditEnabled'}
            label={t('启用违规审计')}
            size='default'
            checkedText='｜'
            uncheckedText='〇'
            onChange={(v) => setEnabled(v)}
          />
          <Typography.Text
            type='secondary'
            style={{ display: 'block', marginTop: 4 }}
          >
            {t(
              '开启后,用户输入将按「违规审计」页面配置的分类分级词库检查:高危拦截、低危放行并记录。词库、审计记录与监控请前往「违规审计」页面配置。',
            )}
          </Typography.Text>
          <div style={{ marginTop: 12, display: 'flex', gap: 8 }}>
            <Button size='default' theme='solid' onClick={onSubmit}>
              {t('保存')}
            </Button>
            <Button
              size='default'
              onClick={() => navigate('/console/violation-audit')}
            >
              {t('前往违规审计页面')}
            </Button>
          </div>
        </Form.Section>
      </Form>
    </Spin>
  );
}
