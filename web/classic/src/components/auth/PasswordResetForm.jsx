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
import {
  API,
  getLogo,
  getStoredValue,
  showError,
  showInfo,
  showSuccess,
} from '../../helpers';
import Turnstile from 'react-turnstile';
import { Button, Form } from '@douyinfe/semi-ui';
import Text from '@douyinfe/semi-ui/lib/es/typography/text';
import { IconMail } from '@douyinfe/semi-icons';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import AuthLayout from './AuthLayout';
import { AuthButtonContent, AuthFormHeader } from './AuthFormVisuals';
import { useRequestLifecycle } from '../../hooks/common/useRequestLifecycle';

const PasswordResetForm = () => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({
    email: '',
  });
  const { email } = inputs;

  const [loading, setLoading] = useState(false);
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [disableButton, setDisableButton] = useState(false);
  const [countdown, setCountdown] = useState(30);
  const { beginRequest, isCurrentRequest } = useRequestLifecycle();

  const logo = getLogo();

  useEffect(() => {
    let status = getStoredValue('status', '');
    if (status) {
      status = JSON.parse(status);
      if (status.turnstile_check) {
        setTurnstileEnabled(true);
        setTurnstileSiteKey(status.turnstile_site_key);
      }
    }
  }, []);

  useEffect(() => {
    let countdownInterval = null;
    if (disableButton && countdown > 0) {
      countdownInterval = setInterval(() => {
        setCountdown(countdown - 1);
      }, 1000);
    } else if (countdown === 0) {
      setDisableButton(false);
      setCountdown(30);
    }
    return () => clearInterval(countdownInterval);
  }, [disableButton, countdown]);

  function handleChange(value) {
    setInputs((inputs) => ({ ...inputs, email: value }));
  }

  async function handleSubmit(e) {
    if (!email) {
      showError(t('请输入邮箱地址'));
      return;
    }
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
      return;
    }
    const requestId = beginRequest('reset');
    setDisableButton(true);
    setLoading(true);
    try {
      const res = await API.get(
        `/api/reset_password?email=${encodeURIComponent(email)}&turnstile=${encodeURIComponent(turnstileToken)}`,
      );
      const { success, message } = res.data;
      if (isCurrentRequest('reset', requestId) && success) {
        showSuccess(t('重置邮件发送成功，请检查邮箱！'));
        setInputs({ ...inputs, email: '' });
      } else if (isCurrentRequest('reset', requestId)) {
        showError(message);
      }
    } catch (error) {
      if (isCurrentRequest('reset', requestId)) {
        showError(error?.message || t('发送失败，请重试'));
      }
    } finally {
      if (isCurrentRequest('reset', requestId)) setLoading(false);
    }
  }

  const renderTurnstile = () => {
    if (!turnstileEnabled) return null;

    return (
      <div className='auth-turnstile auth-turnstile-inline'>
        <Turnstile
          sitekey={turnstileSiteKey}
          onVerify={(token) => {
            setTurnstileToken(token);
          }}
        />
      </div>
    );
  };

  return (
    <AuthLayout>
      <div className='auth-form-shell' key='reset-request'>
        <AuthFormHeader
          title={t('找回密码')}
          subtitle={t('输入邮箱，我们把重置入口发给你。')}
          logo={logo}
        />
        <Form className='auth-minimal-form space-y-4'>
          <Form.Input
            field='email'
            noLabel
            label={t('邮箱')}
            placeholder={t('请输入您的邮箱地址')}
            name='email'
            type='email'
            autoComplete='email'
            value={email}
            onChange={handleChange}
            prefix={<IconMail />}
          />

          {renderTurnstile()}

          <div className='pt-2'>
            <Button
              theme='solid'
              className='auth-primary-button'
              type='primary'
              htmlType='submit'
              onClick={handleSubmit}
              loading={loading}
              disabled={disableButton}
            >
              <AuthButtonContent>
                {disableButton
                  ? `${t('重试')} (${countdown})`
                  : t('发送重置邮件')}
              </AuthButtonContent>
            </Button>
          </div>
        </Form>

        <div className='auth-copy-row'>
          <Text>
            {t('想起来了？')} <Link to='/login'>{t('马上登录')}</Link>
          </Text>
        </div>
      </div>
    </AuthLayout>
  );
};

export default PasswordResetForm;
