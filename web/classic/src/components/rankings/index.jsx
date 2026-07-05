import React from 'react';
import { Spin, Empty } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
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
  const { period, changePeriod, snapshot, loading, error } =
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
          <Empty title={t('加载失败')} description={error} />
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
