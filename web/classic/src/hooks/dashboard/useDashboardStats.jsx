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

import { useMemo } from 'react';
import { renderQuota } from '../../helpers';

const getQuotaWarningThreshold = (user) => {
  try {
    const setting = JSON.parse(user?.setting || '{}');
    return Number(setting.quota_warning_threshold || 500000);
  } catch (_) {
    return 500000;
  }
};

const getBalanceCaption = (user, t) => {
  if (user?.status && user.status !== 1) {
    return { text: t('账户状态异常'), tone: 'red' };
  }

  const quota = Number(user?.quota || 0);
  if (quota <= 0) {
    return { text: t('余额不足'), tone: 'red' };
  }

  if (quota <= getQuotaWarningThreshold(user)) {
    return { text: t('余额偏低'), tone: 'amber' };
  }

  return { text: t('可正常调用'), tone: 'green' };
};

export const useDashboardStats = (
  userState,
  consumeQuota,
  consumeTokens,
  times,
  recentTokens,
  adminUsageSummary,
  activeTimeRange,
  isAdminUser,
  adminUsername,
  performanceMetrics,
  navigate,
  t,
) => {
  const user = userState?.user || {};
  const balanceCaption = getBalanceCaption(user, t);
  const trimmedAdminUsername = String(adminUsername || '').trim();
  const hasAdminUserFilter = Boolean(trimmedAdminUsername);
  const activeUsers = hasAdminUserFilter
    ? Number(times || 0) > 0 ||
      Number(consumeQuota || 0) > 0 ||
      Number(consumeTokens || 0) > 0
      ? 1
      : 0
    : Number(adminUsageSummary?.activeUsers || 0);

  const statsData = useMemo(() => {
    if (isAdminUser) {
      return [
        {
          title: hasAdminUserFilter ? t('消耗') : t('全站消耗'),
          value: renderQuota(consumeQuota),
          caption: t('当前筛选范围'),
          statusText: hasAdminUserFilter
            ? `${t('用户筛选')} · ${trimmedAdminUsername}`
            : t('全站口径'),
          statusTone: 'blue',
          icon: 'coins',
          tone: 'amber',
          captionTone: 'amber',
        },
        {
          title: hasAdminUserFilter ? t('请求') : t('全站请求'),
          value: Number(times || 0).toLocaleString(),
          caption: `${t('平均RPM')} ${performanceMetrics?.avgRPM || '0'}`,
          icon: 'activity',
          tone: 'green',
          captionTone: 'green',
        },
        {
          title: hasAdminUserFilter ? t('Tokens') : t('全站 Tokens'),
          value: isNaN(consumeTokens)
            ? 0
            : Number(consumeTokens).toLocaleString(),
          caption: `${t('平均TPM')} ${performanceMetrics?.avgTPM || '0'}`,
          icon: 'tokens',
          tone: 'cyan',
          captionTone: 'cyan',
        },
        {
          title: t('活跃用户'),
          value: activeUsers.toLocaleString(),
          caption: t('当前范围有调用记录'),
          icon: 'users',
          tone: 'blue',
          captionTone: 'green',
        },
      ];
    }

    const isTodayRange = activeTimeRange === 'today';
    return [
      {
        title: t('当前余额'),
        value: renderQuota(user?.quota),
        caption: t('点击进入充值'),
        statusText: balanceCaption.text,
        statusTone: balanceCaption.tone,
        icon: 'wallet',
        tone: 'blue',
        captionTone: balanceCaption.tone,
        onClick: () => navigate('/console/topup'),
      },
      {
        title: isTodayRange ? t('今日消耗') : t('消耗'),
        value: renderQuota(consumeQuota),
        caption: `${t('历史消耗')} ${renderQuota(user?.used_quota || 0)}`,
        icon: 'coins',
        tone: 'amber',
        captionTone: 'amber',
      },
      {
        title: isTodayRange ? t('今日请求') : t('请求'),
        value: Number(times || 0).toLocaleString(),
        caption: `${t('历史请求')} ${Number(user?.request_count || 0).toLocaleString()}`,
        icon: 'activity',
        tone: 'green',
        captionTone: 'green',
      },
      {
        title: isTodayRange ? t('今日 Tokens') : t('Tokens'),
        value: isNaN(consumeTokens)
          ? 0
          : Number(consumeTokens).toLocaleString(),
        caption: `${t('近 30 天 Tokens')} ${Number(recentTokens || 0).toLocaleString()}`,
        icon: 'tokens',
        tone: 'cyan',
        captionTone: 'cyan',
      },
    ];
  }, [
    activeTimeRange,
    activeUsers,
    balanceCaption.text,
    balanceCaption.tone,
    hasAdminUserFilter,
    isAdminUser,
    trimmedAdminUsername,
    user?.quota,
    user?.used_quota,
    user?.request_count,
    times,
    consumeQuota,
    consumeTokens,
    recentTokens,
    performanceMetrics?.avgRPM,
    performanceMetrics?.avgTPM,
    navigate,
    t,
  ]);

  return {
    statsData,
    summaryTitle: isAdminUser
      ? hasAdminUserFilter
        ? t('筛选用户概览')
        : t('站点运营概览')
      : t('用量概览'),
    summaryNote: isAdminUser
      ? hasAdminUserFilter
        ? t('当前筛选范围，按用户筛选聚合')
        : t('当前筛选范围，全站聚合口径')
      : t('当前范围统计，历史项展示账户累计口径'),
  };
};

