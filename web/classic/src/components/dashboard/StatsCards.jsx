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
import { Skeleton } from '@douyinfe/semi-ui';
import {
  Activity,
  ArrowDownRight,
  ArrowUpRight,
  Coins,
  Minus,
  Users,
  Wallet,
  Zap,
} from 'lucide-react';

const trendIconMap = {
  up: ArrowUpRight,
  down: ArrowDownRight,
  flat: Minus,
};

const COUNT_UP_DURATION_MS = 650;
const METRIC_VALUE_PATTERN =
  /^([^0-9+-]*)([+-]?(?:\d[\d,]*|\d*)(?:\.\d+)?)(.*)$/;

const getReducedMotionPreference = () => {
  if (typeof window === 'undefined' || !window.matchMedia) return true;
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
};

const cancelMetricAnimation = (animationFrame) => {
  if (
    typeof window === 'undefined' ||
    !window.cancelAnimationFrame ||
    animationFrame.current === null
  ) {
    return;
  }

  window.cancelAnimationFrame(animationFrame.current);
  animationFrame.current = null;
};

const clearMetricTimer = (timerRef) => {
  if (
    typeof window === 'undefined' ||
    !window.clearTimeout ||
    timerRef.current === null
  ) {
    return;
  }

  window.clearTimeout(timerRef.current);
  timerRef.current = null;
};

const parseMetricValue = (value) => {
  const text = String(value ?? '');
  const match = text.match(METRIC_VALUE_PATTERN);

  if (!match || match[2] === '' || match[2] === '+' || match[2] === '-') {
    return null;
  }

  const numberText = match[2];
  const number = Number(numberText.replace(/,/g, ''));
  if (!Number.isFinite(number)) return null;

  const decimalPart = numberText.split('.')[1];
  return {
    prefix: match[1] || '',
    suffix: match[3] || '',
    decimals: decimalPart ? decimalPart.length : 0,
    useGrouping: numberText.includes(','),
    number,
  };
};

const formatMetricValue = (template, number) => {
  const fixedNumber = number.toFixed(template.decimals);
  const [integerPart, decimalPart] = fixedNumber.split('.');
  const formattedInteger = template.useGrouping
    ? Number(integerPart).toLocaleString()
    : integerPart;

  return `${template.prefix}${formattedInteger}${
    decimalPart ? `.${decimalPart}` : ''
  }${template.suffix}`;
};

const getTrendSignature = (trend) => {
  if (!trend) return 'none';
  return [trend.direction || 'flat', trend.value || '', trend.label || ''].join(
    '|',
  );
};

const TrendChip = ({ trend }) => {
  if (!trend) return null;

  const TrendIcon = trendIconMap[trend.direction] || Minus;
  return (
    <span
      className={`dashboard-metric-trend is-${trend.direction || 'flat'}`}
      title={trend.label}
      aria-label={trend.label}
    >
      <TrendIcon size={13} strokeWidth={2.5} aria-hidden='true' />
      <span>{trend.value}</span>
    </span>
  );
};

const MetricValue = ({ value, width = 88, height = 28, trend, loading }) => {
  const [displayValue, setDisplayValue] = useState(value);
  const [displayTrend, setDisplayTrend] = useState(trend);
  const displayedValue = useRef(value);
  const hasDisplayedValue = useRef(value !== undefined && value !== null);
  const previousValue = useRef(value);
  const latestTrend = useRef(trend);
  const hasPendingTrend = useRef(false);
  const animationFrame = useRef(null);
  const trendTimer = useRef(null);
  const trendSignature = getTrendSignature(trend);

  const updateDisplayValue = (nextValue) => {
    displayedValue.current = nextValue;
    setDisplayValue(nextValue);
  };

  const updateDisplayTrend = (nextTrend) => {
    hasPendingTrend.current = false;
    setDisplayTrend(nextTrend);
  };

  useEffect(() => {
    if (value === undefined || value === null) return undefined;

    hasDisplayedValue.current = true;

    if (previousValue.current !== value) {
      cancelMetricAnimation(animationFrame);

      const fromMetric = parseMetricValue(displayedValue.current);
      const toMetric = parseMetricValue(value);
      const canCountUp =
        typeof window !== 'undefined' &&
        window.requestAnimationFrame &&
        fromMetric &&
        toMetric &&
        fromMetric.prefix === toMetric.prefix &&
        fromMetric.suffix === toMetric.suffix &&
        !getReducedMotionPreference();

      if (!canCountUp) {
        updateDisplayValue(value);
        clearMetricTimer(trendTimer);
        updateDisplayTrend(latestTrend.current);
        previousValue.current = value;
        return undefined;
      }

      const startedAt = window.performance?.now?.() ?? Date.now();
      const distance = toMetric.number - fromMetric.number;
      const decimals = Math.max(fromMetric.decimals, toMetric.decimals);
      const template = { ...toMetric, decimals };

      const tick = (now) => {
        const progress = Math.min((now - startedAt) / COUNT_UP_DURATION_MS, 1);
        const easedProgress = 1 - Math.pow(1 - progress, 3);
        const nextNumber = fromMetric.number + distance * easedProgress;
        updateDisplayValue(formatMetricValue(template, nextNumber));

        if (progress < 1) {
          animationFrame.current = window.requestAnimationFrame(tick);
          return;
        }

        animationFrame.current = null;
        updateDisplayValue(value);
        clearMetricTimer(trendTimer);
        updateDisplayTrend(latestTrend.current);
      };

      animationFrame.current = window.requestAnimationFrame(tick);
      previousValue.current = value;
      return () => cancelMetricAnimation(animationFrame);
    }

    updateDisplayValue(value);
    return undefined;
  }, [value]);

  useEffect(() => {
    latestTrend.current = trend;
    clearMetricTimer(trendTimer);

    if (animationFrame.current !== null) {
      hasPendingTrend.current = true;
      return;
    }

    trendTimer.current = window.setTimeout(() => {
      trendTimer.current = null;
      updateDisplayTrend(latestTrend.current);
    }, COUNT_UP_DURATION_MS);
  }, [loading, trendSignature]);

  useEffect(() => {
    if (animationFrame.current !== null || !hasPendingTrend.current) {
      return;
    }

    clearMetricTimer(trendTimer);
    updateDisplayTrend(latestTrend.current);
  }, [loading]);

  useEffect(
    () => () => {
      cancelMetricAnimation(animationFrame);
      clearMetricTimer(trendTimer);
    },
    [],
  );

  const shouldShowSkeleton = loading && !hasDisplayedValue.current;
  const displayTrendSignature = getTrendSignature(displayTrend);

  return (
    <Skeleton
      loading={shouldShowSkeleton}
      active
      placeholder={<Skeleton.Title style={{ width, height, margin: 0 }} />}
    >
      <span
        className={`dashboard-metric-value-line${
          displayTrend ? ' has-trend' : ''
        }`}
      >
        <span
          className='dashboard-metric-value-number'
          title={String(displayValue)}
        >
          {displayValue}
        </span>
        {displayTrend && (
          <span className='dashboard-metric-trend-row'>
            <TrendChip key={displayTrendSignature} trend={displayTrend} />
          </span>
        )}
      </span>
    </Skeleton>
  );
};

const iconMap = {
  wallet: Wallet,
  coins: Coins,
  activity: Activity,
  tokens: Zap,
  users: Users,
};

const toneColorMap = {
  blue: '#2563eb',
  amber: '#d97706',
  green: '#059669',
  cyan: '#0891b2',
  red: '#ef4444',
};

const StatsCards = ({ statsData = [], loading, t }) => {
  return (
    <div className='dashboard-usage-grid'>
      {statsData.map((item, index) => {
        const Icon = iconMap[item.icon] || Wallet;
        const color = toneColorMap[item.tone] || '#2563eb';
        const Wrapper = item.onClick ? 'button' : 'div';
        const wrapperProps = item.onClick
          ? {
              type: 'button',
              onClick: item.onClick,
              'aria-label': item.actionLabel || item.caption || item.title,
            }
          : {};

        return (
          <Wrapper
            key={item.icon || item.title || index}
            className={`dashboard-usage-item${item.onClick ? ' is-clickable' : ''}`}
            title={item.caption}
            {...wrapperProps}
          >
            <div className='dashboard-usage-item__header'>
              <Icon size={14} style={{ color }} />
              <span className='dashboard-usage-item__label' style={{ color }}>
                {item.title}
              </span>
            </div>
            <div className='dashboard-usage-item__value'>
              <MetricValue
                value={item.value}
                width={88}
                height={24}
                trend={item.trend}
                loading={loading}
              />
            </div>
            {item.caption && (
              <div className='dashboard-usage-item__caption'>{item.caption}</div>
            )}
          </Wrapper>
        );
      })}
    </div>
  );
};

export default StatsCards;

