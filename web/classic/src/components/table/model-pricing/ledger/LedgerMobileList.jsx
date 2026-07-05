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

import React, { useMemo, useState } from 'react';
import {
  Pico,
  QuotaTag,
  Chips,
  parseTags,
  getPriceData,
  getGroupBreakdown,
  InputCell,
  OutputCell,
  SubInputCell,
  SubOutputCell,
} from './LedgerCells';

const LedgerMobileList = (props) => {
  const {
    t,
    models,
    loading,
    filterGroup,
    pageSize,
    currentPage,
    setCurrentPage,
    ctx,
  } = props;

  const [expanded, setExpanded] = useState(() => new Set());

  const total = models.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const page = Math.min(currentPage, totalPages);
  const start = (page - 1) * pageSize;
  const pageModels = useMemo(
    () => models.slice(start, start + pageSize),
    [models, start, pageSize],
  );

  const toggle = (key) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });

  const from = total === 0 ? 0 : start + 1;
  const to = Math.min(start + pageSize, total);

  if (loading) return <div className='m-state'>{t('加载中…')}</div>;
  if (total === 0) return <div className='m-state'>{t('没有匹配的模型')}</div>;

  return (
    <>
      <div className='m-list'>
        {pageModels.map((m) => {
          const key = m.key || m.model_name;
          const groups = Array.isArray(m.enable_groups) ? m.enable_groups : [];
          const multi = groups.length > 1;
          const isOpen = expanded.has(key);
          const priceData = getPriceData(m, filterGroup, ctx);
          const showLow =
            filterGroup === 'all' &&
            multi &&
            m.quota_type === 0 &&
            !priceData.isDynamicPricing;

          return (
            <div className='m-card' key={key}>
              <div className='mc-head'>
                <Pico model={m} />
                <div className='mc-title'>
                  <div className='nm'>
                    <span className='nm-txt'>{m.model_name}</span>
                    {multi && (
                      <span
                        className='gpill'
                        onClick={() => toggle(key)}
                        style={{ cursor: 'pointer' }}
                      >
                        {groups.length} {t('组')} ▾
                      </span>
                    )}
                  </div>
                  <div className='vd'>{m.vendor_name || t('未知供应商')}</div>
                </div>
                <QuotaTag model={m} t={t} />
              </div>

              <div className='mc-prices'>
                <div className='mc-price'>
                  <div className='mc-plabel'>{t('输入价格')}</div>
                  <InputCell
                    model={m}
                    priceData={priceData}
                    showLow={showLow}
                    t={t}
                  />
                </div>
                <div className='mc-price'>
                  <div className='mc-plabel'>{t('输出价格')}</div>
                  <OutputCell model={m} priceData={priceData} t={t} />
                </div>
              </div>

              {(m.supported_endpoint_types?.length || parseTags(m.tags).length) >
                0 && (
                <div className='mc-tags'>
                  <Chips items={m.supported_endpoint_types} type='ep' max={3} />
                  <Chips items={parseTags(m.tags)} type='tg' max={3} />
                </div>
              )}

              {multi && isOpen && (
                <div className='mc-groups'>
                  {(() => {
                    const { rows, bestIndex } = getGroupBreakdown(m, ctx);
                    return rows.map((g, gi) => (
                      <div
                        className={`mc-grow ${gi === bestIndex ? 'best' : ''}`}
                        key={g.group}
                      >
                        {/* 左1：分组名 */}
                        <div className='mc-gname'>{g.group}</div>
                        {/* 右1：输入价 */}
                        <div className='mc-gpr'>
                          <span className='pl'>{t('输入')}</span>
                          <SubInputCell model={m} priceData={g.priceData} t={t} />
                        </div>
                        {/* 左2：倍率（左对齐，第二行） */}
                        <div className='mc-gratio'>
                          <span className='rbadge'>×{g.ratio.toFixed(1)}</span>
                          {gi === bestIndex && (
                            <span className='lowtag'>{t('最低')}</span>
                          )}
                        </div>
                        {/* 右2：输出价（与输入价上下对齐） */}
                        <div className='mc-gpr'>
                          <span className='pl'>{t('输出')}</span>
                          <SubOutputCell model={m} priceData={g.priceData} />
                        </div>
                      </div>
                    ));
                  })()}
                </div>
              )}
            </div>
          );
        })}
      </div>

      <div className='m-foot'>
        <span>{t('共 {{total}} 个模型', { total })}</span>
        <span className='m-page'>
          <button
            type='button'
            disabled={page <= 1}
            onClick={() => setCurrentPage(page - 1)}
          >
            {t('上一页')}
          </button>
          <span>
            {page} / {totalPages}
          </span>
          <button
            type='button'
            disabled={page >= totalPages}
            onClick={() => setCurrentPage(page + 1)}
          >
            {t('下一页')}
          </button>
        </span>
      </div>
    </>
  );
};

export default LedgerMobileList;
