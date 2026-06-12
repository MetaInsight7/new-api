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
import { Card, Skeleton } from '@douyinfe/semi-ui';
import { ArrowDownRight, ArrowUpRight, Minus } from 'lucide-react';

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

const StatsCards = ({
  statsData = [],
  loading,
  CARD_PROPS,
  title,
  note,
  statusAction,
  statusTitle,
  rangeCaption,
  quickActions = [],
  t,
}) => {
  const displayStatsData = statsData;
  const displayQuickActions = quickActions;
  const balanceStatus = displayStatsData[0] || {};
  const primaryMetric = displayStatsData[0];
  const secondaryMetrics = displayStatsData.slice(1);
  const rateMetrics = secondaryMetrics.filter(
    (item) => item.rateLabel && item.rateValue !== undefined,
  );
  const primaryCaption = [primaryMetric?.caption, rangeCaption]
    .filter(Boolean)
    .join(' · ');
  const displayStatusAction = statusAction;
  const statusPillClass = `dashboard-health-pill is-${balanceStatus.statusTone || 'green'}${
    displayStatusAction ? ' is-clickable' : ''
  }`;
  const statusPillContent = (
    <>
      <span />
      {balanceStatus.statusText}
    </>
  );

  const renderMetricValue = (value, width = 88, height = 28, trend) => (
    <MetricValue
      value={value}
      width={width}
      height={height}
      trend={trend}
      loading={loading}
    />
  );

  const renderStatusPill = (className = '') => {
    const pillClassName = `${statusPillClass}${className ? ` ${className}` : ''}`;

    if (displayStatusAction) {
      return (
        <button
          type='button'
          className={pillClassName}
          onClick={displayStatusAction}
          title={statusTitle}
          aria-label={statusTitle}
        >
          {statusPillContent}
        </button>
      );
    }

    return <div className={pillClassName}>{statusPillContent}</div>;
  };

  const primaryContent = primaryMetric ? (
    <>
      <div
        className='dashboard-primary-metric__label'
        title={note || primaryMetric.title}
      >
        {primaryMetric.title}
      </div>
      <div className='dashboard-primary-metric__value'>
        {renderMetricValue(primaryMetric.value, 128, 40, primaryMetric.trend)}
      </div>
      <div className='dashboard-primary-metric__description'>
        <span>{primaryCaption || note || primaryMetric.title}</span>
      </div>
    </>
  ) : null;

  const ratePanel = rateMetrics.length > 0 && (
    <div className='dashboard-rate-metrics' aria-label={t('速率指标')}>
      {rateMetrics.map((item) => (
        <div className='dashboard-rate-metric' key={item.rateLabel}>
          <span>{item.rateLabel}</span>
          <strong>{item.rateValue}</strong>
        </div>
      ))}
    </div>
  );

  return (
    <Card
      {...CARD_PROPS}
      aria-label={title || t('用量概览')}
      className='dashboard-usage-card'
      bodyStyle={{ padding: 0 }}
    >
      <div className='dashboard-usage-card__body'>
        <div className='dashboard-usage-balance-row'>
          {primaryMetric?.onClick ? (
            <button
              type='button'
              className='dashboard-primary-metric is-clickable'
              onClick={primaryMetric.onClick}
              aria-label={
                primaryMetric.actionLabel ||
                primaryMetric.caption ||
                primaryMetric.title
              }
            >
              {primaryContent}
            </button>
          ) : (
            <div className='dashboard-primary-metric'>{primaryContent}</div>
          )}
          <div className='dashboard-usage-side'>
            {renderStatusPill('dashboard-usage-balance-status')}
            {ratePanel}
          </div>
        </div>

        <div className='dashboard-secondary-metrics'>
          {secondaryMetrics.map((item, index) => {
            const metricKey = item.icon || item.tone || item.title || index;
            const metricContent = (
              <>
                <div className='dashboard-metric-card__label'>{item.title}</div>
                <div className='dashboard-metric-card__value'>
                  {renderMetricValue(item.value, 88, 28, item.trend)}
                </div>
              </>
            );

            if (item.onClick) {
              return (
                <button
                  key={metricKey}
                  type='button'
                  className='dashboard-metric-cell is-clickable'
                  onClick={item.onClick}
                  title={item.caption}
                  aria-label={item.actionLabel || item.caption || item.title}
                >
                  {metricContent}
                </button>
              );
            }

            return (
              <div
                key={metricKey}
                className='dashboard-metric-cell'
                title={item.caption}
              >
                {metricContent}
              </div>
            );
          })}
        </div>

        {displayQuickActions.length > 0 && (
          <div className='dashboard-usage-actions'>
            {displayQuickActions.map((action) => {
              return (
                <button
                  key={action.label}
                  type='button'
                  className='dashboard-usage-action'
                  onClick={action.onClick}
                  title={action.label}
                  aria-label={action.label}
                >
                  <span>{action.label}</span>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </Card>
  );
};

export default StatsCards;

