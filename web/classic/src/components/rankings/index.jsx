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
import { Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { AlertCircle, RefreshCw } from 'lucide-react';
import { useRankingsData } from '../../hooks/rankings/useRankingsData';
import ModelsSection from './ModelsSection';
import MarketShareSection from './MarketShareSection';
import PulseSection from './PulseSection';
import './rankings-dazi.css';

const PERIODS = [
  { key: 'today', label: '今天' },
  { key: 'week', label: '本周' },
  { key: 'month', label: '本月' },
  { key: 'year', label: '本年' },
  { key: 'all', label: '全部' },
];

export default function RankingsPage() {
  const { t } = useTranslation();
  const { period, changePeriod, snapshot, loading, error, retry } =
    useRankingsData('week');

  return (
    <div className='rankings-dazi'>
      <div className='shell wrappad'>
        <div>
          <span className='kicker'>
            <i />
            {t('排行榜 · Leaderboard · 实时统计')}
          </span>
          <h1 className='rk-title'>
            {t('大家都在用哪个，')}
            <em>{t('一眼看明白')}</em>
          </h1>
          <p className='rk-lead'>
            {t('发现平台上最受欢迎的模型和厂商，数据来自实时使用统计。')}
          </p>
        </div>

        <div className='rk-seg'>
          {PERIODS.map((p) => (
            <button
              key={p.key}
              className={period === p.key ? 'on' : ''}
              onClick={() => changePeriod(p.key)}
            >
              {t(p.label)}
            </button>
          ))}
        </div>

        {loading ? (
          <div className='flex justify-center' style={{ padding: '80px 0' }}>
            <Spin size='large' />
          </div>
        ) : error ? (
          <div className='rk-error' role='alert'>
            <span className='rk-error__icon'>
              <AlertCircle size={20} />
            </span>
            <div className='rk-error__copy'>
              <strong>{t('加载失败')}</strong>
              <span>{error}</span>
            </div>
            <button type='button' onClick={retry}>
              <RefreshCw size={15} />
              {t('重试')}
            </button>
          </div>
        ) : snapshot ? (
          <div className='rk-grid'>
            <ModelsSection
              history={snapshot.models_history}
              rows={snapshot.models}
              period={period}
            />
            <MarketShareSection
              history={snapshot.vendor_share_history}
              rows={snapshot.vendors}
              period={period}
            />
            <PulseSection
              movers={snapshot.top_movers}
              droppers={snapshot.top_droppers}
            />
          </div>
        ) : null}
      </div>

      <footer className='rk-footer'>
        <div className='shell'>
          <span>Token 搭子 · Model Passport for Vibe Coding</span>
          <span>tokendazi.com</span>
        </div>
      </footer>
    </div>
  );
}
