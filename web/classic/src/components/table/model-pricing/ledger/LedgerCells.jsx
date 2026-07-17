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
import i18next from 'i18next';
import { getModelCategories } from '../../../../helpers/render';
import { getLobeHubIcon } from '../../../../helpers/lobeIcon';
import { calculateModelPrice } from '../../../../helpers/utils';

/* ---------- 品牌图标盒 ----------
   与项目一致：优先按模型名匹配类别品牌图标（getModelCategories，
   与 renderModelTag 同源，返回 <OpenAI/>/<Claude.Color/> 等彩色图标）；
   退化为模型/供应商自定义图标（getLobeHubIcon）；再退化为名称首字母 */
const getModelBrandIcon = (model) => {
  try {
    const categories = getModelCategories(i18next.t);
    for (const [key, cat] of Object.entries(categories)) {
      if (key !== 'all' && typeof cat.filter === 'function' && cat.filter(model)) {
        return cat.icon || null;
      }
    }
  } catch (e) {
    /* ignore */
  }
  return null;
};

export const Pico = ({ model }) => {
  const brand = getModelBrandIcon(model);
  if (brand) return <div className='pico'>{brand}</div>;
  const iconName = model.icon || model.vendor_icon;
  if (iconName) return <div className='pico'>{getLobeHubIcon(iconName, 20)}</div>;
  const letters = (model.model_name || '?').slice(0, 2).toUpperCase();
  return <div className='pico pico-text'>{letters}</div>;
};

/* ---------- 计费类型标识 ---------- */
export const QuotaTag = ({ model, t }) =>
  model.quota_type === 1 ? (
    <span className='qt call'>
      <i />
      {t('按次')}
    </span>
  ) : (
    <span className='qt tok'>
      <i />
      {t('按量')}
    </span>
  );

/* ---------- 端点 / 标签 chips ---------- */
const TAG_COLORS = {
  推荐: '#f06f46',
  旗舰: '#326ef1',
  推理: '#15a77a',
  多模态: '#15a77a',
  便宜: '#15a77a',
  长上下文: '#7c3aed',
  绘图: '#f06f46',
};
const tagColor = (tag) => TAG_COLORS[tag] || '#686d78';

export const parseTags = (raw) => {
  if (!raw) return [];
  return String(raw)
    .split(/[,;|]+/)
    .map((s) => s.trim())
    .filter(Boolean);
};

export const Chips = ({ items, type, max = 2 }) => {
  const arr = Array.isArray(items) ? items : [];
  if (!arr.length) return <span className='cell-empty'>—</span>;
  const shown = arr.slice(0, max);
  return (
    <>
      {shown.map((x, i) =>
        type === 'tg' ? (
          <span
            key={i}
            className='tg'
            style={{
              color: tagColor(x),
              background: `${tagColor(x)}14`,
              border: `1px solid ${tagColor(x)}30`,
            }}
          >
            {x}
          </span>
        ) : (
          <span key={i} className='ep'>
            {x}
          </span>
        ),
      )}
      {arr.length > max && <span className='more'>+{arr.length - max}</span>}
    </>
  );
};

/* ---------- 价格计算 ---------- */
export const getPriceData = (model, group, ctx) =>
  calculateModelPrice({
    record: model,
    selectedGroup: group,
    groupRatio: ctx.groupRatio,
    tokenUnit: ctx.tokenUnit,
    displayPrice: ctx.displayPrice,
    currency: ctx.currency,
    quotaDisplayType: ctx.siteDisplayType,
  });

// 模型可用分组明细：[{ group, ratio, priceData }]，并标出最低倍率组下标
export const getGroupBreakdown = (model, ctx) => {
  const groups = Array.isArray(model.enable_groups) ? model.enable_groups : [];
  let bestIndex = 0;
  let minRatio = Number.POSITIVE_INFINITY;
  const rows = groups.map((g, i) => {
    const ratio = ctx.groupRatio?.[g];
    const r = ratio === undefined ? 1 : ratio;
    if (r < minRatio) {
      minRatio = r;
      bestIndex = i;
    }
    return { group: g, ratio: r, priceData: getPriceData(model, g, ctx) };
  });
  return { rows, bestIndex };
};

const unitSuffix = (pd) => `/1${pd.unitLabel || 'M'}`;

/* ---------- 输入 / 输出 价格单元 ---------- */
// showLow: 折叠行且当前为「最优/all」多组时展示「最低」标
export const InputCell = ({ model, priceData, showLow, t }) => {
  if (priceData.isDynamicPricing) {
    return <span className='dyn'>{t('动态计费')}</span>;
  }
  if (model.quota_type === 1) {
    return (
      <>
        <span className='v'>{priceData.price}</span>
        <span className='u'>/{t('次')}</span>
      </>
    );
  }
  if (priceData.isTokensDisplay) {
    return <span className='v'>×{priceData.inputRatio ?? '-'}</span>;
  }
  return (
    <>
      <span className='v'>{priceData.inputPrice}</span>
      <span className='u'>{unitSuffix(priceData)}</span>
      {showLow && <span className='low'>{t('最低')}</span>}
    </>
  );
};

export const OutputCell = ({ model, priceData, t }) => {
  if (priceData.isDynamicPricing || model.quota_type === 1) {
    return <span className='dash'>—</span>;
  }
  if (priceData.isTokensDisplay) {
    return <span className='v'>×{priceData.completionRatio ?? '-'}</span>;
  }
  return (
    <>
      <span className='v'>{priceData.completionPrice}</span>
      <span className='u'>{unitSuffix(priceData)}</span>
    </>
  );
};

// 子表内的分组价格（小号 vm/um）
export const SubInputCell = ({ model, priceData, t }) => {
  if (priceData.isDynamicPricing) return <span className='dyn'>{t('动态')}</span>;
  if (model.quota_type === 1) {
    return (
      <>
        <span className='vm'>{priceData.price}</span>
        <span className='um'>/{t('次')}</span>
      </>
    );
  }
  if (priceData.isTokensDisplay) {
    return <span className='vm'>×{priceData.inputRatio ?? '-'}</span>;
  }
  return (
    <>
      <span className='vm'>{priceData.inputPrice}</span>
      <span className='um'>{unitSuffix(priceData)}</span>
    </>
  );
};

export const SubOutputCell = ({ model, priceData }) => {
  if (priceData.isDynamicPricing || model.quota_type === 1) {
    return <span className='dash'>—</span>;
  }
  if (priceData.isTokensDisplay) {
    return <span className='vm'>×{priceData.completionRatio ?? '-'}</span>;
  }
  return (
    <>
      <span className='vm'>{priceData.completionPrice}</span>
      <span className='um'>{unitSuffix(priceData)}</span>
    </>
  );
};
