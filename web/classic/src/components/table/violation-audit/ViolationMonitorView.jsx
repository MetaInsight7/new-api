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
import React, { useEffect, useMemo } from 'react';
import { Spin, Empty } from '@douyinfe/semi-ui';
import { ShieldAlert, ShieldX, ShieldCheck } from 'lucide-react';
import { VChart } from '@visactor/react-vchart';
import { categoryLabel } from './violationHelpers';

// 监控视图:DMIT 统计 tile + 扁平卡包裹的图表。
const StatTile = ({ variant, icon, label, value }) => (
  <div className={`dmit-stat-tile is-${variant}`}>
    <div className='dmit-stat-tile__head'>
      <span className='dmit-stat-tile__icon'>{icon}</span>
      <span className='dmit-stat-tile__label'>{label}</span>
    </div>
    <div className='dmit-stat-tile__value'>{value}</div>
  </div>
);

const ChartCard = ({ title, wide, children }) => (
  <div className={`va-chart-card${wide ? ' va-chart-card--wide' : ''}`}>
    <div className='va-chart-card__title'>{title}</div>
    {children}
  </div>
);

const ViolationMonitorView = ({ data }) => {
  const { t } = data;

  useEffect(() => {
    data.loadStat();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const stat = data.stat;

  const trendSpec = useMemo(() => {
    const values = (stat?.trend || []).flatMap((p) => [
      { day: p.day, type: t('已拦截'), count: p.blocked },
      { day: p.day, type: t('已放行'), count: p.allowed },
    ]);
    return {
      type: 'line',
      data: [{ id: 'trend', values }],
      xField: 'day',
      yField: 'count',
      seriesField: 'type',
      legends: { visible: true },
      point: { visible: true },
      color: ['#dc2626', '#059669'],
    };
  }, [stat, t]);

  const donutSpec = (values) => ({
    type: 'pie',
    data: [{ id: 'd', values }],
    valueField: 'value',
    categoryField: 'type',
    outerRadius: 0.82,
    innerRadius: 0.55,
    legends: { visible: true },
    label: { visible: true },
  });

  const actionPieSpec = useMemo(
    () =>
      Object.assign(
        donutSpec([
          { type: t('已拦截'), value: stat?.blocked || 0 },
          { type: t('已放行'), value: stat?.allowed || 0 },
        ]),
        { color: ['#dc2626', '#059669'] },
      ),
    [stat, t],
  );

  const catPieSpec = useMemo(
    () =>
      donutSpec(
        (stat?.categories || []).map((c) => ({
          type: categoryLabel(t, c.name),
          value: c.count,
        })),
      ),
    [stat, t],
  );

  const barSpec = (values, color) => ({
    type: 'bar',
    data: [{ id: 'b', values }],
    xField: 'count',
    yField: 'name',
    direction: 'horizontal',
    color: [color],
    label: { visible: true },
  });

  const topUsersSpec = useMemo(
    () => barSpec((stat?.top_users || []).map((u) => ({ name: u.name, count: u.count })), '#2563eb'),
    [stat],
  );
  const topWordsSpec = useMemo(
    () => barSpec((stat?.top_words || []).map((w) => ({ name: w.name, count: w.count })), '#d97706'),
    [stat],
  );

  if (data.statLoading && !stat) {
    return (
      <div className='va-center'>
        <Spin size='large' />
      </div>
    );
  }
  if (!stat || stat.total === 0) {
    return (
      <Empty
        title={t('所选时间范围内暂无违规数据')}
        style={{ padding: 56 }}
      />
    );
  }

  const chartProps = { style: { height: 300 } };

  return (
    <div className='va-monitor'>
      <div className='va-kpis'>
        <StatTile
          variant='blue'
          icon={<ShieldAlert size={16} />}
          label={t('违规总数')}
          value={stat.total}
        />
        <StatTile
          variant='amber'
          icon={<ShieldX size={16} />}
          label={t('已拦截')}
          value={stat.blocked}
        />
        <StatTile
          variant='emerald'
          icon={<ShieldCheck size={16} />}
          label={t('已放行')}
          value={stat.allowed}
        />
      </div>

      <div className='va-charts'>
        <ChartCard title={t('违规趋势')} wide>
          <VChart spec={trendSpec} {...chartProps} />
        </ChartCard>
        <ChartCard title={t('拦截占比')}>
          <VChart spec={actionPieSpec} {...chartProps} />
        </ChartCard>
        <ChartCard title={t('分类占比')}>
          <VChart spec={catPieSpec} {...chartProps} />
        </ChartCard>
        <ChartCard title={t('Top 违规用户')}>
          <VChart spec={topUsersSpec} {...chartProps} />
        </ChartCard>
        <ChartCard title={t('Top 命中词')}>
          <VChart spec={topWordsSpec} {...chartProps} />
        </ChartCard>
      </div>
    </div>
  );
};

export default ViolationMonitorView;
