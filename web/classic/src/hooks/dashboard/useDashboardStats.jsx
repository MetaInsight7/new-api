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
import { renderQuota, renderQuotaCompact } from '../../helpers';
import { formatDashboardTokenMetric } from '../../helpers/dashboardFormat';

const getQuotaWarningThreshold = (user) => {
  try {
    const setting = JSON.parse(user?.setting || '{}');
    return Number(setting.quota_warning_threshold || 500000);
  } catch (e) {
    console.error('Failed to parse quota_warning_threshold:', e);
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

// 当前所选时间范围的简短文案(用于随范围联动的副标题)
const getRangeLabel = (activeTimeRange, t) => {
  switch (activeTimeRange) {
    case 'last_24h':
      return t('近 24 小时');
    case 'last_7d':
      return t('近 7 天');
    case 'last_30d':
      return t('近 30 天');
    case 'custom':
      return t('自定义范围');
    case 'today':
    default:
      return t('今日');
  }
};

// 全站消耗副标题:今日精确到 00:00,其余显示"近 X 累计"
const getConsumeCaption = (activeTimeRange, t) => {
  if (activeTimeRange === 'today') return t('今日 00:00 至当前');
  return `${getRangeLabel(activeTimeRange, t)}累计`;
};

// 活跃用户副标题:随范围变
const getActiveUsersCaption = (activeTimeRange, t) =>
  `${getRangeLabel(activeTimeRange, t)}有调用记录`;

const formatTrendPercent = (percent) => {  const absPercent = Math.abs(percent);
  const digits = absPercent >= 10 ? 0 : 1;
  const prefix = percent > 0 ? '+' : percent < 0 ? '-' : '';
  return `${prefix}${absPercent.toFixed(digits)}%`;
};

const getMetricTrend = (currentValue, previousValue, t) => {
  const current = Number(currentValue || 0);
  const previous = Number(previousValue || 0);

  if (
    !Number.isFinite(current) ||
    !Number.isFinite(previous) ||
    previous <= 0
  ) {
    return null;
  }

  const percent = ((current - previous) / previous) * 100;
  if (!Number.isFinite(percent)) {
    return null;
  }

  const direction =
    Math.abs(percent) < 0.05 ? 'flat' : percent > 0 ? 'up' : 'down';
  const value = direction === 'flat' ? '0%' : formatTrendPercent(percent);

  return {
    direction,
    value,
    label: `${t('较上一周期')} ${value}`,
  };
};

export const useDashboardStats = (
  userState,
  consumeQuota,
  consumeTokens,
  times,
  recentTokens,
  comparisonSummary,
  adminUsageSummary,
  selectedAdminUser,
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
  const hasSelectedAdminUser =
    hasAdminUserFilter && selectedAdminUser?.username === trimmedAdminUsername;
  const metricTrends = useMemo(
    () => ({
      quota: getMetricTrend(consumeQuota, comparisonSummary?.quota, t),
      times: getMetricTrend(times, comparisonSummary?.times, t),
      tokens: getMetricTrend(consumeTokens, comparisonSummary?.tokens, t),
    }),
    [
      comparisonSummary?.quota,
      comparisonSummary?.times,
      comparisonSummary?.tokens,
      consumeQuota,
      consumeTokens,
      times,
      t,
    ],
  );
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
          value: renderQuotaCompact(consumeQuota),
          valueExact: renderQuota(consumeQuota),
          caption: getConsumeCaption(activeTimeRange, t),
          statusText: hasAdminUserFilter
            ? `${t('用户筛选')} · ${trimmedAdminUsername}`
            : t('全站口径'),
          statusTone: 'blue',
          icon: 'coins',
          tone: 'amber',
          captionTone: 'amber',
          trend: metricTrends.quota,
        },
        {
          title: hasAdminUserFilter ? t('请求') : t('全站请求'),
          value: Number(times || 0).toLocaleString(),
          caption: `${t('平均RPM')} ${performanceMetrics?.avgRPM || '0'}`,
          rateLabel: 'RPM',
          rateValue: performanceMetrics?.avgRPM || '0',
          icon: 'activity',
          tone: 'green',
          captionTone: 'green',
          trend: metricTrends.times,
        },
        {
          title: hasAdminUserFilter ? t('Tokens') : t('全站 Tokens'),
          value: formatDashboardTokenMetric(consumeTokens),
          caption: `${t('平均TPM')} ${performanceMetrics?.avgTPM || '0'}`,
          rateLabel: 'TPM',
          rateValue: performanceMetrics?.avgTPM || '0',
          icon: 'tokens',
          tone: 'cyan',
          captionTone: 'cyan',
          trend: metricTrends.tokens,
        },
        hasAdminUserFilter
          ? {
              title: t('当前余额'),
              value: hasSelectedAdminUser
                ? renderQuotaCompact(selectedAdminUser?.quota)
                : '--',
              valueExact: hasSelectedAdminUser
                ? renderQuota(selectedAdminUser?.quota)
                : undefined,
              caption: hasSelectedAdminUser
                ? `${t('历史消耗')} ${renderQuota(selectedAdminUser?.used_quota || 0)}`
                : t('暂无数据'),
              icon: 'wallet',
              tone: 'blue',
              captionTone: hasSelectedAdminUser ? 'green' : 'amber',
            }
          : {
              title: t('活跃用户'),
              value: activeUsers.toLocaleString(),
              caption: getActiveUsersCaption(activeTimeRange, t),
              icon: 'users',
              tone: 'blue',
              captionTone: 'green',
            },
      ];
    }

    const rangeLabel = getRangeLabel(activeTimeRange, t);
    return [
      {
        title: t('当前余额'),
        value: renderQuotaCompact(user?.quota),
        valueExact: renderQuota(user?.quota),
        statusText: balanceCaption.text,
        statusTone: balanceCaption.tone,
        icon: 'wallet',
        tone: 'blue',
        captionTone: balanceCaption.tone,
        variant: 'balance',
        topUpLabel: t('充值'),
        onTopUp: () => navigate('/console/topup'),
      },
      {
        title: `${rangeLabel}${t('消耗')}`,
        value: renderQuotaCompact(consumeQuota),
        valueExact: renderQuota(consumeQuota),
        caption: `${t('历史消耗')} ${renderQuota(user?.used_quota || 0)}`,
        icon: 'coins',
        tone: 'amber',
        captionTone: 'amber',
        trend: metricTrends.quota,
      },
      {
        title: `${rangeLabel}${t('请求')}`,
        value: Number(times || 0).toLocaleString(),
        caption: `${t('历史请求')} ${Number(user?.request_count || 0).toLocaleString()}`,
        icon: 'activity',
        tone: 'green',
        captionTone: 'green',
        trend: metricTrends.times,
      },
      {
        title: `${rangeLabel} Tokens`,
        value: formatDashboardTokenMetric(consumeTokens),
        caption: `${t('近 30 天累计')} ${formatDashboardTokenMetric(recentTokens)}`,
        icon: 'tokens',
        tone: 'cyan',
        captionTone: 'cyan',
        trend: metricTrends.tokens,
      },
    ];
  }, [
    activeTimeRange,
    activeUsers,
    balanceCaption.text,
    balanceCaption.tone,
    hasAdminUserFilter,
    hasSelectedAdminUser,
    isAdminUser,
    selectedAdminUser?.quota,
    selectedAdminUser?.used_quota,
    selectedAdminUser?.username,
    trimmedAdminUsername,
    user?.quota,
    user?.used_quota,
    user?.request_count,
    metricTrends.quota,
    metricTrends.times,
    metricTrends.tokens,
    times,
    consumeQuota,
    consumeTokens,
    recentTokens,
    performanceMetrics?.avgRPM,
    performanceMetrics?.avgTPM,
    navigate,
    t,
  ]);

  const quickActions = useMemo(() => {
    if (isAdminUser) {
      return [
        {
          label: t('用户管理'),
          icon: 'users',
          onClick: () => navigate('/console/user'),
        },
        {
          label: t('渠道管理'),
          icon: 'channels',
          onClick: () => navigate('/console/channel'),
        },
        {
          label: t('使用日志'),
          icon: 'logs',
          onClick: () => navigate('/console/log'),
        },
      ];
    }

    return [
      {
        label: t('充值'),
        icon: 'topup',
        onClick: () => navigate('/console/topup'),
      },
      {
        label: t('API 密钥管理'),
        icon: 'keys',
        onClick: () => navigate('/console/token'),
      },
      {
        label: t('使用日志'),
        icon: 'logs',
        onClick: () => navigate('/console/log'),
      },
    ];
  }, [isAdminUser, navigate, t]);

  return {
    statsData,
    quickActions,
    balanceCaption,
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
