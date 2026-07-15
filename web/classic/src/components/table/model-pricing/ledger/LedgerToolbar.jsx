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

import React, { useMemo, useRef, useState, useEffect } from 'react';

const SearchIcon = () => (
  <svg viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2.4'>
    <circle cx='11' cy='11' r='8' />
    <path d='m21 21-4.3-4.3' />
  </svg>
);

// 顶部供应商行内联展示的数量
const INLINE_VENDORS = 5;

const useOutsideClose = (ref, onClose) => {
  useEffect(() => {
    const handler = (e) => {
      if (ref.current && !ref.current.contains(e.target)) onClose();
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [ref, onClose]);
};

const LedgerToolbar = (props) => {
  const {
    t,
    searchValue,
    handleChange,
    handleCompositionStart,
    handleCompositionEnd,
    filterGroup,
    handleGroupClick,
    groupRatio,
    usableGroup,
    filterQuotaType,
    setFilterQuotaType,
    filterVendor,
    setFilterVendor,
    models,
  } = props;

  const [groupOpen, setGroupOpen] = useState(false);
  const [vendorOpen, setVendorOpen] = useState(false);
  const [vendorSearch, setVendorSearch] = useState('');
  const groupRef = useRef(null);
  const vendorRef = useRef(null);
  const searchRef = useRef(null);
  useOutsideClose(groupRef, () => setGroupOpen(false));
  useOutsideClose(vendorRef, () => {
    setVendorOpen(false);
    setVendorSearch('');
  });

  // 分组选项
  const groupKeys = useMemo(
    () => Object.keys(usableGroup || {}),
    [usableGroup],
  );
  const groupLabel = useMemo(() => {
    if (filterGroup === 'all') return t('全部 · 最优');
    const r = groupRatio?.[filterGroup];
    return `${filterGroup} · ×${r === undefined ? 1 : r}`;
  }, [filterGroup, groupRatio, t]);

  // 供应商计数（含未知）
  const vendorStats = useMemo(() => {
    const map = new Map();
    let unknown = 0;
    (models || []).forEach((m) => {
      if (m.vendor_name) {
        map.set(m.vendor_name, (map.get(m.vendor_name) || 0) + 1);
      } else {
        unknown += 1;
      }
    });
    const list = Array.from(map.entries())
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count);
    if (unknown > 0) list.push({ name: 'unknown', count: unknown });
    return list;
  }, [models]);

  const inline = vendorStats.slice(0, INLINE_VENDORS);
  const total = models?.length || 0;

  const filteredVendors = useMemo(() => {
    const q = vendorSearch.trim().toLowerCase();
    if (!q) return vendorStats;
    return vendorStats.filter((v) =>
      (v.name === 'unknown' ? t('未知供应商') : v.name)
        .toLowerCase()
        .includes(q),
    );
  }, [vendorStats, vendorSearch, t]);

  const vendorName = (name) =>
    name === 'unknown' ? t('未知供应商') : name;

  const pickGroup = (g) => {
    handleGroupClick(g);
    setGroupOpen(false);
  };
  const pickVendor = (name) => {
    setFilterVendor(name);
    setVendorOpen(false);
    setVendorSearch('');
  };

  return (
    <div className='m-top'>
      <div className='m-seal'>
        {t('LIVE')}
        <br />
        {t('实时同步')}
      </div>

      {/* 搜索 + 分组 + 计费 */}
      <div className='m-filter-row'>
        <div className='m-search'>
          <SearchIcon />
          <input
            ref={searchRef}
            value={searchValue}
            aria-label={t('搜索模型')}
            placeholder={t('搜索 GPT、Claude、Gemini…')}
            onChange={(e) => handleChange(e.target.value)}
            onCompositionStart={handleCompositionStart}
            onCompositionEnd={handleCompositionEnd}
          />
          <button
            type='button'
            className='s-btn'
            onClick={() => searchRef.current && searchRef.current.focus()}
          >
            {t('搜索')}
          </button>
        </div>

        <div className='fright'>
          <span className='hlabel'>{t('分组')}</span>
          <div className='sel-wrap' ref={groupRef} style={{ position: 'relative' }}>
            <span
              className='sel key'
              onClick={() => setGroupOpen((v) => !v)}
            >
              {groupLabel} <span className='cr'>▾</span>
            </span>
            {groupOpen && (
              <div className='vpop' style={{ width: 240 }}>
                <div className='vpop-body'>
                  <div
                    className={`vrow ${filterGroup === 'all' ? 'on' : ''}`}
                    onClick={() => pickGroup('all')}
                  >
                    <span className='vn'>{t('全部 · 最优')}</span>
                    {filterGroup === 'all' && <span className='ck'>✓</span>}
                  </div>
                  {groupKeys.map((g) => {
                    const r = groupRatio?.[g];
                    return (
                      <div
                        key={g}
                        className={`vrow ${filterGroup === g ? 'on' : ''}`}
                        onClick={() => pickGroup(g)}
                      >
                        <span className='vn'>{g}</span>
                        <span className='cnt'>×{r === undefined ? 1 : r}</span>
                        {filterGroup === g && <span className='ck'>✓</span>}
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>

          <span className='hlabel' style={{ marginLeft: 4 }}>
            {t('计费')}
          </span>
          <div className='seg'>
            <button
              className={filterQuotaType === 'all' ? 'on' : ''}
              onClick={() => setFilterQuotaType('all')}
            >
              {t('全部')}
            </button>
            <button
              className={filterQuotaType === 0 ? 'on' : ''}
              onClick={() => setFilterQuotaType(0)}
            >
              {t('按量')}
            </button>
            <button
              className={filterQuotaType === 1 ? 'on' : ''}
              onClick={() => setFilterQuotaType(1)}
            >
              {t('按次')}
            </button>
          </div>
        </div>
      </div>

      {/* 供应商 */}
      <div className='m-vendor-row'>
        <span className='hlabel'>{t('供应商')}</span>
        <div className='vendors'>
          <span
            className={`vpill ${filterVendor === 'all' ? 'on' : ''}`}
            onClick={() => setFilterVendor('all')}
          >
            {t('全部')}
          </span>
          {inline.map((v) => (
            <span
              key={v.name}
              className={`vpill ${filterVendor === v.name ? 'on' : ''}`}
              onClick={() => setFilterVendor(v.name)}
            >
              {vendorName(v.name)}
              <span className='vc'>{v.count}</span>
            </span>
          ))}
          {vendorStats.length > INLINE_VENDORS && (
            <div className='vmore-wrap' ref={vendorRef}>
              <span
                className='vpill more'
                onClick={() => setVendorOpen((v) => !v)}
              >
                {t('更多')} ▾
              </span>
              {vendorOpen && (
                <div className='vpop'>
                  <div className='vpop-search'>
                    <SearchIcon />
                    <input
                      autoFocus
                      value={vendorSearch}
                      placeholder={t('搜索供应商…')}
                      onChange={(e) => setVendorSearch(e.target.value)}
                    />
                  </div>
                  <div className='vpop-body'>
                    <div
                      className={`vrow ${filterVendor === 'all' ? 'on' : ''}`}
                      onClick={() => pickVendor('all')}
                    >
                      <span className='vn'>{t('全部供应商')}</span>
                      <span className='cnt'>{total}</span>
                      {filterVendor === 'all' && <span className='ck'>✓</span>}
                    </div>
                    {filteredVendors.map((v) => (
                      <div
                        key={v.name}
                        className={`vrow ${filterVendor === v.name ? 'on' : ''}`}
                        onClick={() => pickVendor(v.name)}
                      >
                        <span className='vn'>{vendorName(v.name)}</span>
                        <span className='cnt'>{v.count}</span>
                        {filterVendor === v.name && (
                          <span className='ck'>✓</span>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default LedgerToolbar;
