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

import React from 'react';
import { Card } from '@douyinfe/semi-ui';
import { PieChart } from 'lucide-react';
import { VChart } from '@visactor/react-vchart';

const ChartsPanel = ({
  activeChartTab,
  setActiveChartTab,
  spec_line,
  spec_model_line,
  spec_pie,
  spec_rank_bar,
  spec_user_rank,
  spec_user_trend,
  isAdminUser,
  CARD_PROPS,
  CHART_CONFIG,
  FLEX_CENTER_GAP2,
  t,
}) => {
  const chartTabs = [
    { key: '1', label: t('消耗分布') },
    { key: '2', label: t('调用趋势') },
    { key: '3', label: t('次数分布') },
    { key: '4', label: t('次数排行') },
    ...(isAdminUser
      ? [
          { key: '5', label: t('用户排行') },
          { key: '6', label: t('用户趋势') },
        ]
      : []),
  ];

  const renderChart = () => {
    const commonProps = {
      options: CHART_CONFIG,
      className: 'dashboard-chart-render',
      width: '100%',
      height: '100%',
    };

    if (activeChartTab === '1') {
      return <VChart {...commonProps} spec={spec_line} />;
    }
    if (activeChartTab === '2') {
      return <VChart {...commonProps} spec={spec_model_line} />;
    }
    if (activeChartTab === '3') {
      return <VChart {...commonProps} spec={spec_pie} />;
    }
    if (activeChartTab === '4') {
      return <VChart {...commonProps} spec={spec_rank_bar} />;
    }
    if (activeChartTab === '5' && isAdminUser) {
      return <VChart {...commonProps} spec={spec_user_rank} />;
    }
    if (activeChartTab === '6' && isAdminUser) {
      return <VChart {...commonProps} spec={spec_user_trend} />;
    }
    return <VChart {...commonProps} spec={spec_line} />;
  };

  return (
    <Card
      {...CARD_PROPS}
      className='dashboard-chart-card !rounded-2xl'
      title={
        <div className='dashboard-chart-header'>
          <div className={`dashboard-chart-title ${FLEX_CENTER_GAP2}`}>
            <PieChart size={16} />
            {t('模型数据分析')}
          </div>
          <div
            className='dashboard-chart-segments'
            role='tablist'
            aria-label={t('模型数据分析')}
          >
            {chartTabs.map((tab) => (
              <button
                key={tab.key}
                type='button'
                role='tab'
                aria-selected={activeChartTab === tab.key}
                className={`dashboard-chart-segment ${
                  activeChartTab === tab.key ? 'is-active' : ''
                }`}
                onClick={() => setActiveChartTab(tab.key)}
              >
                {tab.label}
              </button>
            ))}
          </div>
        </div>
      }
      bodyStyle={{ padding: 0 }}
    >
      <div className='dashboard-chart-body'>{renderChart()}</div>
    </Card>
  );
};

export default ChartsPanel;

