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

import React, { useContext, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  API,
  copy,
  showError,
  showInfo,
  showSuccess,
  setStatusData,
  prepareCredentialCreationOptions,
  buildRegistrationResult,
  isPasskeySupported,
  setUserData,
  renderQuota,
  isRoot,
  isAdmin,
  onGitHubOAuthClicked,
  onOIDCClicked,
  onLinuxDOOAuthClicked,
  onDiscordOAuthClicked,
  getStoredJSON,
  removeStoredValue,
  setStoredValue,
} from '../../helpers';
import { UserContext } from '../../context/User';
import { Modal, Select } from '@douyinfe/semi-ui';
import { IconMail, IconGithubLogo, IconShield } from '@douyinfe/semi-icons';
import { SiWechat, SiDiscord, SiTelegram, SiLinux } from 'react-icons/si';
import { Link2, Bell, SlidersHorizontal } from 'lucide-react';
import { useTranslation } from 'react-i18next';

// 导入子组件
import AccountManagement from './personal/cards/AccountManagement';
import SecuritySettings from './personal/cards/SecuritySettings';
import NotificationSettings from './personal/cards/NotificationSettings';
import CheckinCalendar from './personal/cards/CheckinCalendar';
import EmailBindModal from './personal/modals/EmailBindModal';
import WeChatBindModal from './personal/modals/WeChatBindModal';
import AccountDeleteModal from './personal/modals/AccountDeleteModal';
import ChangePasswordModal from './personal/modals/ChangePasswordModal';
import SecureVerificationModal from '../common/modals/SecureVerificationModal';
import { useSecureVerification } from '../../hooks/common/useSecureVerification';
import { useRequestLifecycle } from '../../hooks/common/useRequestLifecycle';

const PersonalSetting = () => {
  const [userState, userDispatch] = useContext(UserContext);
  let navigate = useNavigate();
  const { t, i18n } = useTranslation();
  const { beginRequest, isCurrentRequest } = useRequestLifecycle();

  const [inputs, setInputs] = useState({
    wechat_verification_code: '',
    email_verification_code: '',
    email: '',
    self_account_deletion_confirmation: '',
    original_password: '',
    set_new_password: '',
    set_new_password_confirmation: '',
  });
  const [status, setStatus] = useState({});
  const [showChangePasswordModal, setShowChangePasswordModal] = useState(false);
  const [showWeChatBindModal, setShowWeChatBindModal] = useState(false);
  const [showEmailBindModal, setShowEmailBindModal] = useState(false);
  const [showAccountDeleteModal, setShowAccountDeleteModal] = useState(false);
  const [showAccountModal, setShowAccountModal] = useState(false);
  const [showNotifyModal, setShowNotifyModal] = useState(false);
  const [turnstileEnabled, setTurnstileEnabled] = useState(false);
  const [turnstileSiteKey, setTurnstileSiteKey] = useState('');
  const [turnstileToken, setTurnstileToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [disableButton, setDisableButton] = useState(false);
  const [countdown, setCountdown] = useState(30);
  const [systemToken, setSystemToken] = useState('');
  const [passkeyStatus, setPasskeyStatus] = useState({ enabled: false });
  const [passkeyRegisterLoading, setPasskeyRegisterLoading] = useState(false);
  const [passkeyDeleteLoading, setPasskeyDeleteLoading] = useState(false);
  const [passkeySupported, setPasskeySupported] = useState(false);
  const [
    passkeyRequiredVerificationMethod,
    setPasskeyRequiredVerificationMethod,
  ] = useState(null);
  const [notificationSettings, setNotificationSettings] = useState({
    warningType: 'email',
    warningThreshold: 100000,
    webhookUrl: '',
    webhookSecret: '',
    notificationEmail: '',
    barkUrl: '',
    gotifyUrl: '',
    gotifyToken: '',
    gotifyPriority: 5,
    upstreamModelUpdateNotifyEnabled: false,
    acceptUnsetModelRatioModel: false,
    recordIpLog: false,
  });

  const {
    isModalVisible: isPasskeyVerificationModalVisible,
    verificationMethods: passkeyVerificationMethods,
    verificationState: passkeyVerificationState,
    startVerification: startPasskeyVerification,
    executeVerification: executePasskeyVerification,
    cancelVerification: cancelPasskeyVerification,
    setVerificationCode: setPasskeyVerificationCode,
    switchVerificationMethod: switchPasskeyVerificationMethod,
    checkVerificationMethods: checkPasskeyVerificationMethods,
  } = useSecureVerification({
    onSuccess: () => {
      setPasskeyRequiredVerificationMethod(null);
    },
  });

  const visiblePasskeyVerificationMethods = passkeyRequiredVerificationMethod
    ? {
        ...passkeyVerificationMethods,
        has2FA:
          passkeyRequiredVerificationMethod === '2fa' &&
          passkeyVerificationMethods.has2FA,
        hasPasskey:
          passkeyRequiredVerificationMethod === 'passkey' &&
          passkeyVerificationMethods.hasPasskey,
      }
    : passkeyVerificationMethods;

  useEffect(() => {
    const parsed = getStoredJSON('status', null);
    if (parsed) {
      setStatus(parsed);
      if (parsed.turnstile_check) {
        setTurnstileEnabled(true);
        setTurnstileSiteKey(parsed.turnstile_site_key);
      } else {
        setTurnstileEnabled(false);
        setTurnstileSiteKey('');
      }
    }
    // Always refresh status from server to avoid stale flags (e.g., admin just enabled OAuth)
    (async () => {
      const requestId = beginRequest('status');
      try {
        const res = await API.get('/api/status');
        if (!isCurrentRequest('status', requestId)) return;
        const { success, data } = res.data;
        if (success && data) {
          setStatus(data);
          setStatusData(data);
          if (data.turnstile_check) {
            setTurnstileEnabled(true);
            setTurnstileSiteKey(data.turnstile_site_key);
          } else {
            setTurnstileEnabled(false);
            setTurnstileSiteKey('');
          }
        }
      } catch (e) {
        // ignore and keep local status
      }
    })();

    getUserData();

    isPasskeySupported()
      .then(setPasskeySupported)
      .catch(() => setPasskeySupported(false));
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
    return () => clearInterval(countdownInterval); // Clean up on unmount
  }, [disableButton, countdown]);

  useEffect(() => {
    if (userState?.user?.setting) {
      let settings;
      try {
        settings = JSON.parse(userState.user.setting);
      } catch {
        settings = {};
      }
      setNotificationSettings({
        warningType: settings.notify_type || 'email',
        warningThreshold: settings.quota_warning_threshold || 500000,
        webhookUrl: settings.webhook_url || '',
        webhookSecret: settings.webhook_secret || '',
        notificationEmail: settings.notification_email || '',
        barkUrl: settings.bark_url || '',
        gotifyUrl: settings.gotify_url || '',
        gotifyToken: settings.gotify_token || '',
        gotifyPriority:
          settings.gotify_priority !== undefined ? settings.gotify_priority : 5,
        upstreamModelUpdateNotifyEnabled:
          settings.upstream_model_update_notify_enabled === true,
        acceptUnsetModelRatioModel:
          settings.accept_unset_model_ratio_model || false,
        recordIpLog: settings.record_ip_log || false,
      });
    }
  }, [userState?.user?.setting]);

  const handleInputChange = (name, value) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };

  const generateAccessToken = async () => {
    const res = await API.get('/api/user/token');
    const { success, message, data } = res.data;
    if (success) {
      setSystemToken(data);
      await copy(data);
      showSuccess(t('令牌已重置并已复制到剪贴板'));
    } else {
      showError(message);
    }
  };

  const loadPasskeyStatus = async () => {
    try {
      const res = await API.get('/api/user/passkey');
      const { success, data, message } = res.data;
      if (success) {
        setPasskeyStatus({
          enabled: data?.enabled || false,
          last_used_at: data?.last_used_at || null,
          backup_eligible: data?.backup_eligible || false,
          backup_state: data?.backup_state || false,
        });
      } else {
        showError(message);
      }
    } catch (error) {
      // 忽略错误，保留默认状态
    }
  };

  const startPasskeyManagementVerification = async (apiCall, options = {}) => {
    const methods = await checkPasskeyVerificationMethods();
    const requiredMethod = methods.has2FA
      ? '2fa'
      : methods.hasPasskey
        ? 'passkey'
        : null;

    if (!requiredMethod) {
      showError(t('您需要先启用两步验证或 Passkey 才能执行此操作'));
      return;
    }

    if (requiredMethod === 'passkey' && !methods.passkeySupported) {
      showInfo(t('当前设备不支持 Passkey'));
      return;
    }

    setPasskeyRequiredVerificationMethod(requiredMethod);
    await startPasskeyVerification(apiCall, {
      preferredMethod: requiredMethod,
      title: t('安全验证'),
      ...options,
    });
  };

  const startPasskeyRegistration = async () => {
    const methods = await checkPasskeyVerificationMethods();
    if (!methods.has2FA) {
      try {
        await registerPasskey();
      } catch (error) {
        showError(error.message || t('Passkey 注册失败，请重试'));
      }
      return;
    }

    setPasskeyRequiredVerificationMethod('2fa');
    await startPasskeyVerification(registerPasskey, {
      preferredMethod: '2fa',
      title: t('安全验证'),
    });
  };

  const registerPasskey = async () => {
    setPasskeyRegisterLoading(true);
    try {
      const beginRes = await API.post('/api/user/passkey/register/begin');
      const { success, message, data } = beginRes.data;
      if (!success) {
        throw new Error(message || t('无法发起 Passkey 注册'));
      }

      const publicKey = prepareCredentialCreationOptions(
        data?.options || data?.publicKey || data,
      );
      const credential = await navigator.credentials.create({ publicKey });
      const payload = buildRegistrationResult(credential);
      if (!payload) {
        throw new Error(t('Passkey 注册失败，请重试'));
      }

      const finishRes = await API.post(
        '/api/user/passkey/register/finish',
        payload,
      );
      if (!finishRes.data.success) {
        throw new Error(
          finishRes.data.message || t('Passkey 注册失败，请重试'),
        );
      }

      showSuccess(t('Passkey 注册成功'));
      await loadPasskeyStatus();
      return finishRes.data;
    } catch (error) {
      if (error?.name === 'AbortError') {
        showInfo(t('已取消 Passkey 注册'));
        return { cancelled: true };
      }
      throw new Error(error?.message || t('Passkey 注册失败，请重试'));
    } finally {
      setPasskeyRegisterLoading(false);
    }
  };

  const handleRegisterPasskey = async () => {
    if (!passkeySupported || !window.PublicKeyCredential) {
      showInfo(t('当前设备不支持 Passkey'));
      return;
    }
    await startPasskeyRegistration();
  };

  const removePasskey = async () => {
    setPasskeyDeleteLoading(true);
    try {
      const res = await API.delete('/api/user/passkey');
      const { success, message } = res.data;
      if (!success) {
        throw new Error(message || t('操作失败，请重试'));
      }

      showSuccess(t('Passkey 已解绑'));
      await loadPasskeyStatus();
      return res.data;
    } catch (error) {
      throw new Error(error?.message || t('操作失败，请重试'));
    } finally {
      setPasskeyDeleteLoading(false);
    }
  };

  const handleRemovePasskey = async () => {
    await startPasskeyManagementVerification(removePasskey);
  };

  const handlePasskeyVerificationCancel = () => {
    setPasskeyRequiredVerificationMethod(null);
    cancelPasskeyVerification();
  };

  const getUserData = async () => {
    const requestId = beginRequest('user');
    let res = await API.get(`/api/user/self`);
    if (!isCurrentRequest('user', requestId)) return;
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
      setUserData(data);
      await loadPasskeyStatus();
    } else {
      showError(message);
    }
  };

  const handleSystemTokenClick = async (e) => {
    e.target.select();
    await copy(e.target.value);
    showSuccess(t('系统令牌已复制到剪切板'));
  };

  const deleteAccount = async () => {
    if (inputs.self_account_deletion_confirmation !== userState.user.username) {
      showError(t('请输入你的账户名以确认删除！'));
      return;
    }

    const res = await API.delete('/api/user/self');
    const { success, message } = res.data;

    if (success) {
      showSuccess(t('账户已删除！'));
      await API.get('/api/user/logout');
      userDispatch({ type: 'logout' });
      removeStoredValue('user');
      navigate('/login');
    } else {
      showError(message);
    }
  };

  const bindWeChat = async () => {
    if (inputs.wechat_verification_code === '') return;
    const res = await API.post('/api/oauth/wechat/bind', {
      code: inputs.wechat_verification_code,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('微信账户绑定成功！'));
      setShowWeChatBindModal(false);
    } else {
      showError(message);
    }
  };

  const changePassword = async () => {
    // if (inputs.original_password === '') {
    //   showError(t('请输入原密码！'));
    //   return;
    // }
    if (inputs.set_new_password === '') {
      showError(t('请输入新密码！'));
      return;
    }
    if (inputs.original_password === inputs.set_new_password) {
      showError(t('新密码需要和原密码不一致！'));
      return;
    }
    if (inputs.set_new_password !== inputs.set_new_password_confirmation) {
      showError(t('两次输入的密码不一致！'));
      return;
    }
    const res = await API.put(`/api/user/self`, {
      original_password: inputs.original_password,
      password: inputs.set_new_password,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('密码修改成功！'));
      setShowWeChatBindModal(false);
    } else {
      showError(message);
    }
    setShowChangePasswordModal(false);
  };

  const sendVerificationCode = async () => {
    if (inputs.email === '') {
      showError(t('请输入邮箱！'));
      return;
    }
    setDisableButton(true);
    if (turnstileEnabled && turnstileToken === '') {
      showInfo(t('请稍后几秒重试，Turnstile 正在检查用户环境！'));
      return;
    }
    setLoading(true);
    const res = await API.get(
      `/api/verification?email=${inputs.email}&turnstile=${turnstileToken}`,
    );
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('验证码发送成功，请检查邮箱！'));
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const bindEmail = async () => {
    if (inputs.email_verification_code === '') {
      showError(t('请输入邮箱验证码！'));
      return;
    }
    setLoading(true);
    const res = await API.post('/api/oauth/email/bind', {
      email: inputs.email,
      code: inputs.email_verification_code,
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('邮箱账户绑定成功！'));
      setShowEmailBindModal(false);
      userState.user.email = inputs.email;
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const copyText = async (text) => {
    if (await copy(text)) {
      showSuccess(t('已复制：') + text);
    } else {
      // setSearchKeyword(text);
      Modal.error({ title: t('无法复制到剪贴板，请手动复制'), content: text });
    }
  };

  const handleNotificationSettingChange = (type, value) => {
    setNotificationSettings((prev) => ({
      ...prev,
      [type]: value.target
        ? value.target.value !== undefined
          ? value.target.value
          : value.target.checked
        : value, // handle checkbox properly
    }));
  };

  const saveNotificationSettings = async () => {
    try {
      const res = await API.put('/api/user/setting', {
        notify_type: notificationSettings.warningType,
        quota_warning_threshold: parseFloat(
          notificationSettings.warningThreshold,
        ),
        webhook_url: notificationSettings.webhookUrl,
        webhook_secret: notificationSettings.webhookSecret,
        notification_email: notificationSettings.notificationEmail,
        bark_url: notificationSettings.barkUrl,
        gotify_url: notificationSettings.gotifyUrl,
        gotify_token: notificationSettings.gotifyToken,
        gotify_priority: (() => {
          const parsed = parseInt(notificationSettings.gotifyPriority);
          return isNaN(parsed) ? 5 : parsed;
        })(),
        upstream_model_update_notify_enabled:
          notificationSettings.upstreamModelUpdateNotifyEnabled === true,
        accept_unset_model_ratio_model:
          notificationSettings.acceptUnsetModelRatioModel,
        record_ip_log: notificationSettings.recordIpLog,
      });

      if (res.data.success) {
        showSuccess(t('设置保存成功'));
        await getUserData();
      } else {
        showError(res.data.message);
      }
    } catch (error) {
      showError(t('设置保存失败'));
    }
  };

  const MONO =
    "'Geist Mono Variable','SFMono-Regular',ui-monospace,Menlo,monospace";
  const bentoIco = {
    width: 30,
    height: 30,
    borderRadius: 9,
    background: 'rgba(37,99,235,0.08)',
    color: '#2563eb',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    flex: '0 0 auto',
    fontSize: 15,
  };
  const bentoTitle = {
    fontSize: 14,
    fontWeight: 600,
    color: '#141a1f',
    letterSpacing: '-0.01em',
  };
  const bentoCard = {
    border: '1px solid #eef1f6',
    borderRadius: 14,
    padding: '16px 18px',
    background: '#fff',
    minWidth: 0,
  };
  const bentoHd = {
    display: 'flex',
    alignItems: 'center',
    gap: 9,
    marginBottom: 13,
  };
  const bentoFix = {
    marginLeft: 'auto',
    background: 'none',
    border: 0,
    cursor: 'pointer',
    fontSize: 12,
    fontWeight: 600,
    color: '#1d4ed8',
  };
  const checkRow = {
    display: 'flex',
    alignItems: 'center',
    gap: 9,
    padding: '9px 0',
    fontSize: 13,
    borderBottom: '1px solid #eef1f6',
  };
  const checkMk = {
    width: 18,
    height: 18,
    borderRadius: '50%',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 11,
    color: '#fff',
    flex: '0 0 auto',
  };
  const kvRow = {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    fontSize: 13,
    padding: '8px 0',
    borderBottom: '1px solid #eef1f6',
  };
  const chip = {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 7,
    height: 34,
    padding: '0 12px',
    border: '1px solid #e7e9ee',
    borderRadius: 10,
    fontSize: 12.5,
    fontWeight: 600,
    color: '#3c4650',
  };

  const user = userState?.user || {};
  const avatarText = (user.username || 'NA').slice(0, 2).toUpperCase();
  const roleLabel = isRoot()
    ? t('超级管理员')
    : isAdmin()
      ? t('管理员')
      : t('普通用户');
  const wpill = {
    display: 'inline-flex',
    alignItems: 'center',
    height: 21,
    padding: '0 9px',
    borderRadius: 999,
    fontSize: 11.5,
    fontWeight: 600,
    background: 'rgba(255,255,255,0.16)',
    color: '#fff',
  };
  const bindings = [
    {
      name: t('邮箱'),
      icon: <IconMail size='small' />,
      bound: !!user.email,
      enabled: true,
      onClick: () => setShowEmailBindModal(true),
    },
    {
      name: t('微信'),
      icon: <SiWechat size={15} />,
      bound: !!user.wechat_id,
      enabled: !!status.wechat_login,
      onClick: () => setShowWeChatBindModal(true),
    },
    {
      name: 'GitHub',
      icon: <IconGithubLogo size='small' />,
      bound: !!user.github_id,
      enabled: !!status.github_oauth,
      onClick: () => onGitHubOAuthClicked(status.github_client_id),
    },
    {
      name: 'Discord',
      icon: <SiDiscord size={15} />,
      bound: !!user.discord_id,
      enabled: !!status.discord_oauth,
      onClick: () => onDiscordOAuthClicked(status.discord_client_id),
    },
    {
      name: 'OIDC',
      icon: <IconShield size='small' />,
      bound: !!user.oidc_id,
      enabled: !!status.oidc_enabled,
      onClick: () =>
        onOIDCClicked(
          status.oidc_authorization_endpoint,
          status.oidc_client_id,
        ),
    },
    {
      name: 'Telegram',
      icon: <SiTelegram size={15} />,
      bound: !!user.telegram_id,
      enabled: !!status.telegram_oauth,
      // Telegram 需登录挂件，点开「全部管理」弹窗内完成
      onClick: () => setShowAccountModal(true),
    },
    {
      name: 'LinuxDO',
      icon: <SiLinux size={15} />,
      bound: !!user.linux_do_id,
      enabled: !!status.linuxdo_oauth,
      onClick: () => onLinuxDOOAuthClicked(status.linuxdo_client_id),
    },
  ];
  const notifyMethodLabel =
    { email: t('邮件'), webhook: 'Webhook', bark: 'Bark', gotify: 'Gotify' }[
      notificationSettings.warningType
    ] || t('邮件');

  const languageOptions = [
    { value: 'zh-CN', label: '简体中文' },
    { value: 'zh-TW', label: '繁體中文' },
    { value: 'en', label: 'English' },
    { value: 'fr', label: 'Français' },
    { value: 'ru', label: 'Русский' },
    { value: 'ja', label: '日本語' },
    { value: 'vi', label: 'Tiếng Việt' },
  ];
  const curLang =
    languageOptions.find((o) => o.value === i18n.language)?.value ||
    (String(i18n.language || '').startsWith('zh') ? 'zh-CN' : 'en');
  const handleLanguageChange = async (lang) => {
    i18n.changeLanguage(lang);
    setStoredValue('i18nextLng', lang);
    try {
      await API.put('/api/user/self', { language: lang });
      showSuccess(t('语言偏好已保存'));
    } catch (e) {
      // ignore
    }
  };

  const handleLogout = async () => {
    try {
      await API.get('/api/user/logout');
    } catch (e) {
      // ignore network error, still clear local session
    }
    showSuccess(t('注销成功!'));
    userDispatch({ type: 'logout' });
    removeStoredValue('user');
    navigate('/login');
  };

  const accountProps = {
    t,
    userState,
    status,
    systemToken,
    setShowEmailBindModal,
    setShowWeChatBindModal,
    generateAccessToken,
    handleSystemTokenClick,
    setShowChangePasswordModal,
    setShowAccountDeleteModal,
    passkeyStatus,
    passkeySupported,
    passkeyRegisterLoading,
    passkeyDeleteLoading,
    onPasskeyRegister: handleRegisterPasskey,
    onPasskeyDelete: handleRemovePasskey,
  };

  return (
    <div className='mt-[60px]'>
      <style>{`
        .dmit-personal-split{display:grid;grid-template-columns:288px 1fr;gap:16px;align-items:start}
        .dmit-personal-split .dpb-content{display:flex;flex-direction:column;gap:14px;min-width:0}
        .dmit-personal-split .dpb-row2{display:grid;grid-template-columns:1fr 1fr;gap:14px;align-items:start}
        @media(max-width:900px){
          .dmit-personal-split{grid-template-columns:1fr}
        }
        @media(max-width:640px){
          .dmit-personal-split .dpb-row2{grid-template-columns:1fr}
        }
      `}</style>
      <div className='flex justify-center'>
        <div className='w-full mx-auto px-2'>
          {/* 签到日历 - 仅在启用时显示 */}
          {status?.checkin_enabled && (
            <div className='mb-4 md:mb-6'>
              <CheckinCalendar
                t={t}
                status={status}
                turnstileEnabled={turnstileEnabled}
                turnstileSiteKey={turnstileSiteKey}
              />
            </div>
          )}

          {/* ===== 左侧彩色资料栏 + 右侧设置 ===== */}
          <div className='dmit-personal-split'>
            {/* 左侧 rail */}
            <aside
              className='dpb-rail'
              style={{
                background: '#2563eb',
                color: '#fff',
                borderRadius: 16,
                padding: '22px 20px 18px',
                position: 'relative',
                overflow: 'hidden',
              }}
            >
              <div
                style={{
                  position: 'absolute',
                  right: -50,
                  top: -70,
                  width: 200,
                  height: 200,
                  borderRadius: '50%',
                  background: 'rgba(255,255,255,0.06)',
                  pointerEvents: 'none',
                }}
              />
              <div
                style={{
                  position: 'relative',
                  zIndex: 1,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 14,
                }}
              >
                <div
                  style={{
                    width: 56,
                    height: 56,
                    borderRadius: 16,
                    background: 'rgba(255,255,255,0.18)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 20,
                    fontWeight: 700,
                    flex: '0 0 auto',
                  }}
                >
                  {avatarText}
                </div>
                <div style={{ minWidth: 0 }}>
                  <div
                    className='truncate'
                    style={{
                      fontSize: 20,
                      fontWeight: 700,
                      lineHeight: 1.15,
                      letterSpacing: '-0.01em',
                    }}
                  >
                    {user.username || '-'}
                  </div>
                  <div
                    style={{
                      display: 'flex',
                      flexWrap: 'wrap',
                      gap: 6,
                      marginTop: 7,
                    }}
                  >
                    <span style={wpill}>{roleLabel}</span>
                    <span style={wpill}>ID {user.id}</span>
                  </div>
                </div>
              </div>

              <div
                style={{
                  height: 1,
                  background: 'rgba(255,255,255,0.16)',
                  margin: '18px 0',
                }}
              />

              {[
                { k: t('当前余额'), v: renderQuota(user.quota), hl: true },
                { k: t('历史消耗'), v: renderQuota(user.used_quota) },
                {
                  k: t('请求次数'),
                  v: (user.request_count || 0).toLocaleString(),
                },
                { k: t('用户分组'), v: user.group || t('默认'), small: true },
              ].map((s, i) => (
                <div
                  key={i}
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'baseline',
                    padding: '9px 0',
                  }}
                >
                  <span
                    style={{ fontSize: 12.5, color: 'rgba(255,255,255,0.72)' }}
                  >
                    {s.k}
                  </span>
                  <span
                    style={{
                      fontFamily: MONO,
                      fontVariantNumeric: 'tabular-nums',
                      fontSize: 15,
                      fontWeight: 600,
                      letterSpacing: '-0.01em',
                    }}
                  >
                    {s.v}
                  </span>
                </div>
              ))}

              <div style={{ display: 'flex', gap: 8, marginTop: 14 }}>
                <button
                  onClick={() => navigate('/console/topup')}
                  style={{
                    flex: 1,
                    height: 36,
                    border: 0,
                    borderRadius: 9,
                    cursor: 'pointer',
                    background: '#fff',
                    color: '#1d4ed8',
                    fontSize: 13,
                    fontWeight: 700,
                  }}
                >
                  {t('去充值')}
                </button>
                <button
                  onClick={() => navigate('/console/topup?show_history=true')}
                  style={{
                    flex: 1,
                    height: 36,
                    border: 0,
                    borderRadius: 9,
                    cursor: 'pointer',
                    background: 'rgba(255,255,255,0.16)',
                    color: '#fff',
                    fontSize: 13,
                    fontWeight: 600,
                  }}
                >
                  {t('账单明细')}
                </button>
              </div>
              <div
                onClick={handleLogout}
                style={{
                  marginTop: 14,
                  textAlign: 'center',
                  fontSize: 12,
                  fontWeight: 500,
                  color: 'rgba(255,255,255,0.75)',
                  cursor: 'pointer',
                }}
              >
                {t('退出登录')}
              </div>
            </aside>

            {/* 右侧内容 */}
            <div className='dpb-content'>
              {/* 账户绑定（通栏） */}
              <div style={bentoCard}>
                <div style={bentoHd}>
                  <span style={bentoIco}>
                    <Link2 size={16} />
                  </span>
                  <span style={bentoTitle}>{t('账户绑定')}</span>
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                  {bindings.map((b) => {
                    const disabled = !b.bound && !b.enabled;
                    return (
                      <button
                        key={b.name}
                        type='button'
                        disabled={disabled}
                        title={
                          b.bound
                            ? t('已绑定，点击管理')
                            : b.enabled
                              ? t('点击绑定')
                              : t('未启用')
                        }
                        onClick={
                          b.bound
                            ? () => setShowAccountModal(true)
                            : b.enabled
                              ? b.onClick
                              : undefined
                        }
                        style={{
                          ...chip,
                          background: '#fff',
                          cursor: disabled ? 'not-allowed' : 'pointer',
                          opacity: disabled ? 0.5 : 1,
                          borderColor: b.bound
                            ? 'rgba(15,157,110,0.45)'
                            : '#e7e9ee',
                        }}
                      >
                        <span
                          style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            color: '#3c4650',
                          }}
                        >
                          {b.icon}
                        </span>
                        <span>{b.name}</span>
                        <span
                          style={{
                            width: 6,
                            height: 6,
                            borderRadius: '50%',
                            background: b.bound ? '#0f9d6e' : '#c2c8d0',
                          }}
                        />
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* 安全（左） + 通知/偏好（右） */}
              <div className='dpb-row2'>
                {/* 安全设置（功能直接放在页面上） */}
                <SecuritySettings {...accountProps} />

                {/* 通知 + 偏好（竖排） */}
                <div
                  style={{ display: 'flex', flexDirection: 'column', gap: 14 }}
                >
                  <div style={bentoCard}>
                    <div style={bentoHd}>
                      <span style={bentoIco}>
                        <Bell size={16} />
                      </span>
                      <span style={bentoTitle}>{t('通知')}</span>
                      <button
                        style={bentoFix}
                        onClick={() => setShowNotifyModal(true)}
                      >
                        {t('配置')} →
                      </button>
                    </div>
                    <div style={kvRow}>
                      <span style={{ color: '#6b7686' }}>{t('方式')}</span>
                      <span style={{ fontWeight: 600 }}>
                        {notifyMethodLabel}
                      </span>
                    </div>
                    <div style={{ ...kvRow, borderBottom: 'none' }}>
                      <span style={{ color: '#6b7686' }}>{t('预警阈值')}</span>
                      <span style={{ fontWeight: 600, fontFamily: MONO }}>
                        {renderQuota(notificationSettings.warningThreshold)}
                      </span>
                    </div>
                  </div>

                  <div style={bentoCard}>
                    <div style={bentoHd}>
                      <span style={bentoIco}>
                        <SlidersHorizontal size={16} />
                      </span>
                      <span style={bentoTitle}>{t('偏好')}</span>
                    </div>
                    <div style={{ ...kvRow, borderBottom: 'none' }}>
                      <span style={{ color: '#6b7686' }}>{t('界面语言')}</span>
                      <Select
                        value={curLang}
                        onChange={handleLanguageChange}
                        size='small'
                        style={{ width: 140 }}
                        optionList={languageOptions}
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* 账户与安全 · 复用现有组件的弹窗 */}
      <Modal
        title={t('账户与安全')}
        visible={showAccountModal}
        onCancel={() => setShowAccountModal(false)}
        footer={null}
        width={720}
        centered
        bodyStyle={{ padding: 0, maxHeight: '72vh', overflowY: 'auto' }}
      >
        <AccountManagement {...accountProps} />
      </Modal>

      {/* 通知设置 · 复用现有组件的弹窗 */}
      <Modal
        title={t('通知设置')}
        visible={showNotifyModal}
        onCancel={() => setShowNotifyModal(false)}
        footer={null}
        width={720}
        centered
        bodyStyle={{ padding: 0, maxHeight: '72vh', overflowY: 'auto' }}
      >
        <NotificationSettings
          t={t}
          notificationSettings={notificationSettings}
          handleNotificationSettingChange={handleNotificationSettingChange}
          saveNotificationSettings={saveNotificationSettings}
        />
      </Modal>

      {/* 模态框组件 */}
      <EmailBindModal
        t={t}
        showEmailBindModal={showEmailBindModal}
        setShowEmailBindModal={setShowEmailBindModal}
        inputs={inputs}
        handleInputChange={handleInputChange}
        sendVerificationCode={sendVerificationCode}
        bindEmail={bindEmail}
        disableButton={disableButton}
        loading={loading}
        countdown={countdown}
        turnstileEnabled={turnstileEnabled}
        turnstileSiteKey={turnstileSiteKey}
        setTurnstileToken={setTurnstileToken}
      />

      <WeChatBindModal
        t={t}
        showWeChatBindModal={showWeChatBindModal}
        setShowWeChatBindModal={setShowWeChatBindModal}
        inputs={inputs}
        handleInputChange={handleInputChange}
        bindWeChat={bindWeChat}
        status={status}
      />

      <AccountDeleteModal
        t={t}
        showAccountDeleteModal={showAccountDeleteModal}
        setShowAccountDeleteModal={setShowAccountDeleteModal}
        inputs={inputs}
        handleInputChange={handleInputChange}
        deleteAccount={deleteAccount}
        userState={userState}
        turnstileEnabled={turnstileEnabled}
        turnstileSiteKey={turnstileSiteKey}
        setTurnstileToken={setTurnstileToken}
      />

      <ChangePasswordModal
        t={t}
        showChangePasswordModal={showChangePasswordModal}
        setShowChangePasswordModal={setShowChangePasswordModal}
        inputs={inputs}
        handleInputChange={handleInputChange}
        changePassword={changePassword}
        turnstileEnabled={turnstileEnabled}
        turnstileSiteKey={turnstileSiteKey}
        setTurnstileToken={setTurnstileToken}
      />

      <SecureVerificationModal
        visible={isPasskeyVerificationModalVisible}
        verificationMethods={visiblePasskeyVerificationMethods}
        verificationState={passkeyVerificationState}
        onVerify={executePasskeyVerification}
        onCancel={handlePasskeyVerificationCancel}
        onCodeChange={setPasskeyVerificationCode}
        onMethodSwitch={switchPasskeyVerificationMethod}
        title={passkeyVerificationState.title}
        description={passkeyVerificationState.description}
      />
    </div>
  );
};

export default PersonalSetting;
