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

const LedgerTable = (props) => {
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

  return (
    <>
      <table>
        <colgroup>
          <col style={{ width: '26%' }} />
          <col style={{ width: '9%' }} />
          <col style={{ width: '19%' }} />
          <col style={{ width: '19%' }} />
          <col style={{ width: '15%' }} />
          <col style={{ width: '12%' }} />
        </colgroup>
        <thead>
          <tr>
            <th>{t('模型 / 供应商')}</th>
            <th>{t('计费')}</th>
            <th>{t('输入价格')}</th>
            <th>{t('输出价格')}</th>
            <th>{t('接口端点')}</th>
            <th>{t('模型标签')}</th>
          </tr>
        </thead>
        <tbody>
          {loading && (
            <tr>
              <td colSpan={6} className='m-state'>
                {t('加载中…')}
              </td>
            </tr>
          )}
          {!loading && total === 0 && (
            <tr>
              <td colSpan={6} className='m-state'>
                {t('没有匹配的模型')}
              </td>
            </tr>
          )}
          {!loading &&
            pageModels.map((m) => {
              const key = m.key || m.model_name;
              const groups = Array.isArray(m.enable_groups)
                ? m.enable_groups
                : [];
              const multi = groups.length > 1;
              const isOpen = expanded.has(key);
              const priceData = getPriceData(m, filterGroup, ctx);
              const showLow =
                filterGroup === 'all' &&
                multi &&
                m.quota_type === 0 &&
                !priceData.isDynamicPricing;

              const rows = [
                <tr
                  key={key}
                  className={`model ${multi ? 'exp' : ''}`}
                  onClick={() => multi && toggle(key)}
                >
                  <td>
                    <div className='mdl'>
                      <span
                        className={`chev ${multi ? '' : 'none'} ${
                          isOpen ? 'open' : ''
                        }`}
                      >
                        ▶
                      </span>
                      <Pico model={m} />
                      <div style={{ minWidth: 0, flex: 1 }}>
                        <div className='nm'>
                          <span className='nm-txt'>{m.model_name}</span>
                          {multi && (
                            <span className='gpill'>
                              {groups.length} {t('组')} ▾
                            </span>
                          )}
                        </div>
                        <div className='vd'>{m.vendor_name || t('未知供应商')}</div>
                      </div>
                    </div>
                  </td>
                  <td>
                    <QuotaTag model={m} t={t} />
                  </td>
                  <td>
                    <InputCell
                      model={m}
                      priceData={priceData}
                      showLow={showLow}
                      t={t}
                    />
                  </td>
                  <td>
                    <OutputCell model={m} priceData={priceData} t={t} />
                  </td>
                  <td className='cell-chips'>
                    <Chips items={m.supported_endpoint_types} type='ep' />
                  </td>
                  <td className='cell-chips'>
                    <Chips items={parseTags(m.tags)} type='tg' />
                  </td>
                </tr>,
              ];

              if (multi && isOpen) {
                const { rows: gRows, bestIndex } = getGroupBreakdown(m, ctx);
                rows.push(
                  <tr className='subwrap' key={`${key}-sub`}>
                    <td colSpan={6}>
                      <div className='subinner'>
                        <div className='subttl'>
                          {m.model_name} ·{' '}
                          {t('各令牌分组价格（共 {{n}} 组）', {
                            n: gRows.length,
                          })}
                        </div>
                        <table className='subtable'>
                          <colgroup>
                            <col style={{ width: '38%' }} />
                            <col style={{ width: '14%' }} />
                            <col style={{ width: '24%' }} />
                            <col style={{ width: '24%' }} />
                          </colgroup>
                          <tbody>
                            <tr className='sub-head'>
                              <td>{t('分组')}</td>
                              <td>{t('倍率')}</td>
                              <td>{t('输入价格')}</td>
                              <td>{t('输出价格')}</td>
                            </tr>
                            {gRows.map((g, gi) => (
                              <tr
                                key={g.group}
                                className={gi === bestIndex ? 'best' : ''}
                              >
                                <td>
                                  <span className='gname'>{g.group}</span>
                                  {gi === bestIndex && (
                                    <span className='lowtag'>{t('最低价')}</span>
                                  )}
                                </td>
                                <td>
                                  <span className='rbadge'>
                                    ×{g.ratio.toFixed(1)}
                                  </span>
                                </td>
                                <td>
                                  <SubInputCell
                                    model={m}
                                    priceData={g.priceData}
                                    t={t}
                                  />
                                </td>
                                <td>
                                  <SubOutputCell
                                    model={m}
                                    priceData={g.priceData}
                                  />
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </td>
                  </tr>,
                );
              }
              return rows;
            })}
        </tbody>
      </table>

      <div className='m-foot'>
        <span>
          {t('显示 {{from}}–{{to}} / 共 {{total}} 个模型', { from, to, total })}
        </span>
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

export default LedgerTable;
