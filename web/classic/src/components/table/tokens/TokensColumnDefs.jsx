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
import { Tooltip, Popover, Typography, Modal } from '@douyinfe/semi-ui';
import {
  timestamp2string,
  renderQuota,
  getModelCategories,
} from '../../../helpers';
import { Pencil, Power, Trash2, Eye, EyeOff, Copy } from 'lucide-react';

// 与使用日志一致的视觉语言 —— 原型见 .tmp/token-proto5.html
const MONO = '"SFMono-Regular", ui-monospace, Menlo, monospace';
const K = {
  ink: '#33394e',
  ink2: '#33394e',
  mut: '#6b7686',
  mut2: '#9aa4b2',
  line: '#eef1f5',
};

// 状态:发光信号灯 + 文字
function renderStatusLight(status, t) {
  const map = {
    1: { text: t('正常'), dot: '#10b981', glow: 'rgba(16,185,129,.16)', color: '#0f9d6e' },
    2: { text: t('已禁用'), dot: '#ef4444', glow: 'rgba(239,68,68,.16)', color: '#b91c1c' },
    3: { text: t('已过期'), dot: '#f59e0b', glow: 'rgba(245,158,11,.16)', color: '#b45309' },
    4: { text: t('已耗尽'), dot: '#94a3b8', glow: 'rgba(148,163,184,.16)', color: '#6b7686' },
  };
  const c = map[status] || { text: t('未知'), dot: '#94a3b8', glow: 'rgba(148,163,184,.16)', color: '#6b7686' };
  return (
    <span className='token-table-status' style={{ color: c.color }}>
      <span className='token-status-dot' style={{ ['--dot']: c.dot, ['--glow']: c.glow }} />
      {c.text}
    </span>
  );
}

// 名称 / 到期
function renderNameCell(record, t) {
  const expiry =
    record.expired_time === -1
      ? t('永不过期')
      : `${t('到期')} ${String(timestamp2string(record.expired_time)).split(' ')[0]}`;
  return (
    <div>
      <div className='token-table-primary token-table-name'>
        {record.name || '-'}
      </div>
      <div className='token-table-secondary'>{expiry}</div>
    </div>
  );
}

// 剩余额度 / 总额度（两行数字，详细数据保留在悬浮层）
function renderQuotaCell(record, t) {
  const { Paragraph } = Typography;
  const used = parseInt(record.used_quota) || 0;
  const remain = parseInt(record.remain_quota) || 0;
  const total = used + remain;

  if (record.unlimited_quota) {
    return (
      <Popover
        position='top'
        content={
          <div className='text-xs p-2'>
            <Paragraph copyable={{ content: renderQuota(used) }}>{t('已用额度')}: {renderQuota(used)}</Paragraph>
          </div>
        }
      >
        <div className='token-table-stack token-table-stack--interactive'>
          <div className='token-table-primary'>
            <span className='token-table-field-label'>{t('剩余')}：</span>
            {t('无限')}
          </div>
          <div className='token-table-secondary'>
            <span className='token-table-field-label'>{t('已用')}：</span>
            {renderQuota(used)}
          </div>
        </div>
      </Popover>
    );
  }

  const percent = total > 0 ? (remain / total) * 100 : 0;
  return (
    <Popover
      position='top'
      content={
        <div className='text-xs p-2'>
          <Paragraph copyable={{ content: renderQuota(used) }}>{t('已用额度')}: {renderQuota(used)}</Paragraph>
          <Paragraph copyable={{ content: renderQuota(remain) }}>{t('剩余额度')}: {renderQuota(remain)} ({percent.toFixed(0)}%)</Paragraph>
          <Paragraph copyable={{ content: renderQuota(total) }}>{t('总额度')}: {renderQuota(total)}</Paragraph>
        </div>
      }
    >
      <div className='token-table-stack token-table-stack--interactive'>
        <div className='token-table-primary'>
          <span className='token-table-field-label'>{t('剩余')}：</span>
          {renderQuota(remain)}
        </div>
        <div className='token-table-secondary'>
          <span className='token-table-field-label'>{t('已用')}：</span>
          {renderQuota(used)}
        </div>
      </div>
    </Popover>
  );
}

// 分组 / 倍率(两行)
function renderGroupCell(record, t, groupRatios) {
  const group = record.group;
  const line = (label, value, valStyle) => (
    <div className='token-table-line'>
      <span className='token-table-field-label'>{label}</span>
      <span style={valStyle}>{value}</span>
    </div>
  );
  if (group === 'auto') {
    return (
      <div>
        <Tooltip content={t('当前分组为 auto，会自动选择最优分组，当一个组不可用时自动降级到下一个组（熔断机制）')}>
          {line(`${t('分组')}：`, t('智能熔断'), { color: '#2563eb', fontWeight: 600, cursor: 'help' })}
        </Tooltip>
        <div className='token-table-secondary'>{line(`${t('倍率')}：`, t('自动'))}</div>
      </div>
    );
  }
  const ratio = groupRatios[group];
  return (
    <div>
      {line(`${t('分组')}：`, group || t('默认'))}
      <div className='token-table-secondary'>
        {line(`${t('倍率')}：`, ratio !== undefined ? `${ratio}x` : '-', { color: '#4f46e5', fontWeight: 600 })}
      </div>
    </div>
  );
}

// 密钥:代码行灰底框 + 灰图标右对齐
function renderKeyCell(record, ctx) {
  const revealed = !!ctx.showKeys[record.id];
  const loading = !!ctx.loadingTokenKeys[record.id];
  const keyValue =
    revealed && ctx.resolvedTokenKeys[record.id]
      ? ctx.resolvedTokenKeys[record.id]
      : record.key || '';
  const displayedKey = keyValue ? `sk-${keyValue}` : '-';
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 2, background: '#f6f8fa', border: '1px solid #eef1f5', borderRadius: 7, padding: '3px 4px 3px 10px', maxWidth: '100%' }}>
      <span className='token-keyscroll token-table-key'>
        {displayedKey}
      </span>
      <span style={{ display: 'flex', alignItems: 'center', gap: 1, flex: '0 0 auto' }}>
        <span
          className='token-keyic'
          title={revealed ? ctx.t('隐藏') : ctx.t('显示')}
          onClick={async (e) => {
            e.stopPropagation();
            await ctx.toggleTokenVisibility(record);
          }}
          style={loading ? { opacity: 0.5, pointerEvents: 'none' } : undefined}
        >
          {revealed ? <EyeOff size={14} /> : <Eye size={14} />}
        </span>
        <span
          className='token-keyic'
          title={ctx.t('复制密钥')}
          onClick={(e) => {
            e.stopPropagation();
            ctx.copyTokenKey(record);
          }}
        >
          <Copy size={14} />
        </span>
      </span>
    </div>
  );
}

// 模型 / IP 限制:默认 1 个 + N
function limLine(label, first, extra, isIp) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
      <span style={{ fontSize: 12, color: K.mut2, flex: '0 0 auto', width: 24 }}>{label}</span>
      <span style={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: 5 }}>
        {first == null ? (
          <span style={{ fontSize: 13, color: K.ink2 }}>{first === null ? '' : ''}</span>
        ) : (
          <>
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0, fontSize: isIp ? 11.5 : 12.5, color: K.ink2, fontFamily: isIp ? MONO : undefined }}>
              {first}
            </span>
            {extra > 0 && (
              <span style={{ display: 'inline-flex', alignItems: 'center', height: 16, padding: '0 5px', borderRadius: 5, background: '#f1f5f9', color: K.ink2, fontSize: 11, fontWeight: 600, flex: '0 0 auto', cursor: 'help' }}>
                +{extra}
              </span>
            )}
          </>
        )}
      </span>
    </div>
  );
}

function renderLimitsCell(record, t) {
  const models = record.model_limits_enabled && record.model_limits
    ? String(record.model_limits).split(',').filter(Boolean)
    : [];
  const ips = record.allow_ips && record.allow_ips.trim() !== ''
    ? String(record.allow_ips).split('\n').map((s) => s.trim()).filter(Boolean)
    : [];
  const categories = getModelCategories(t);
  const getModelIcon = (modelName) => {
    const category = Object.entries(categories).find(
      ([key, item]) =>
        key !== 'all' && item.filter({ model_name: modelName }),
    );
    return category?.[1]?.icon;
  };
  const valLine = (items, isModel = false) =>
    items.length > 0 ? (
      <Tooltip content={items.join(', ')}>
        <span className='token-table-limit-line'>
          {isModel && (
            <span className='usage-log-model-icon token-table-model-icon'>
              {getModelIcon(items[0]) || (
                <span className='usage-log-model-icon__fallback' />
              )}
            </span>
          )}
          <span className='token-table-limit-value'>{items[0]}</span>
          {items.length > 1 && (
            <span className='token-table-count-badge'>+{items.length - 1}</span>
          )}
        </span>
      </Tooltip>
    ) : (
      <span>{isModel ? t('全部模型') : t('不限制 IP')}</span>
    );
  return (
    <div className='token-table-stack'>
      <div className='token-table-limit-row'>{valLine(models, true)}</div>
      <div className='token-table-limit-row'>{valLine(ips)}</div>
    </div>
  );
}

// 最后使用（单行，精确到分钟）
function renderTimeCell(record, t) {
  if (!record.accessed_time) {
    return <span className='token-table-secondary token-table-secondary--single'>{t('从未使用')}</span>;
  }

  return (
    <span className='token-table-primary token-table-time'>
      {String(timestamp2string(record.accessed_time)).slice(0, 16)}
    </span>
  );
}

// 操作:无边框图标(编辑 / 开关 / 删除)
function renderOpsCell(record, ctx) {
  const t = ctx.t;
  const enabled = record.status === 1;
  return (
    <div style={{ display: 'inline-flex', alignItems: 'center', gap: 2 }}>
      <span
        className='token-op edit'
        title={t('编辑')}
        onClick={(e) => {
          e.stopPropagation();
          ctx.setEditingToken(record);
          ctx.setShowEdit(true);
        }}
      >
        <Pencil size={16} />
      </span>
      <span
        className='token-op pow'
        title={enabled ? t('禁用') : t('启用')}
        onClick={async (e) => {
          e.stopPropagation();
          await ctx.manageToken(record.id, enabled ? 'disable' : 'enable', record);
          await ctx.refresh();
        }}
      >
        <Power size={16} />
      </span>
      <span
        className='token-op del'
        title={t('删除')}
        onClick={(e) => {
          e.stopPropagation();
          Modal.confirm({
            title: t('确定是否要删除此令牌？'),
            content: t('此修改将不可逆'),
            onOk: async () => {
              await ctx.manageToken(record.id, 'delete', record);
              await ctx.refresh();
            },
          });
        }}
      >
        <Trash2 size={16} />
      </span>
    </div>
  );
}

// ============ 移动端卡片 ============
function renderTokenMobileCard(record, ctx, groupRatios) {
  const t = ctx.t;
  const group = record.group;
  const isAuto = group === 'auto';
  const ratio = groupRatios[group];
  const remainText = record.unlimited_quota ? t('无限') : renderQuota(record.remain_quota);
  const groupText = isAuto ? t('智能熔断') : (group || t('默认'));
  const ratioText = isAuto ? t('自动') : (ratio !== undefined ? `${ratio}x` : '-');
  const expiry =
    record.expired_time === -1
      ? t('永不过期')
      : `${t('到期')} ${String(timestamp2string(record.expired_time)).split(' ')[0]}`;

  const models = record.model_limits_enabled && record.model_limits
    ? String(record.model_limits).split(',').filter(Boolean)
    : [];
  const ips = record.allow_ips && record.allow_ips.trim() !== ''
    ? String(record.allow_ips).split('\n').map((s) => s.trim()).filter(Boolean)
    : [];
  const limitParts = [];
  if (models.length) limitParts.push(`${models.length} ${t('模型')}`);
  if (ips.length) limitParts.push(`${ips.length} IP`);
  const limitText = limitParts.length ? limitParts.join(' · ') : t('未限制模型/IP');

  const revealed = !!ctx.showKeys[record.id];
  const loading = !!ctx.loadingTokenKeys[record.id];
  const keyValue =
    revealed && ctx.resolvedTokenKeys[record.id]
      ? ctx.resolvedTokenKeys[record.id]
      : record.key || '';
  const displayedKey = keyValue ? `sk-${keyValue}` : '-';
  const keyIc = {
    width: 26, height: 26, flex: '0 0 auto', borderRadius: 6,
    border: '1px solid #34373d', display: 'grid', placeItems: 'center', color: '#c4c8cf',
  };

  return (
    <div style={{ background: '#fff', border: '1.5px solid #e2e5dc', borderRadius: 14, padding: '13px 14px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10 }}>
        <div style={{ fontSize: 16, fontWeight: 850, color: K.ink, letterSpacing: '-0.01em', wordBreak: 'break-all', minWidth: 0 }}>
          {record.name || '-'}
        </div>
        <div style={{ flex: '0 0 auto' }}>{renderStatusLight(record.status, t)}</div>
      </div>

      {/* 密钥 深色胶囊 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 11, padding: '9px 11px', borderRadius: 9, background: '#0f1013' }}>
        <span className='token-keyscroll' style={{ flex: 1, minWidth: 0, fontFamily: MONO, fontSize: 12.5, color: '#d9f5e2', fontWeight: 600, whiteSpace: 'nowrap', overflowX: 'auto' }}>
          {displayedKey}
        </span>
        <span
          title={revealed ? t('隐藏') : t('显示')}
          onClick={async (e) => { e.stopPropagation(); await ctx.toggleTokenVisibility(record); }}
          style={{ ...keyIc, ...(loading ? { opacity: 0.5, pointerEvents: 'none' } : {}) }}
        >
          {revealed ? <EyeOff size={13} /> : <Eye size={13} />}
        </span>
        <span
          title={t('复制密钥')}
          onClick={(e) => { e.stopPropagation(); ctx.copyTokenKey(record); }}
          style={keyIc}
        >
          <Copy size={13} />
        </span>
      </div>

      {/* 三格网格 */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 1, marginTop: 11, background: K.line, border: `1px solid ${K.line}`, borderRadius: 10, overflow: 'hidden' }}>
        {[[t('剩余额度'), remainText], [t('分组'), groupText], [t('倍率'), ratioText]].map(([l, v], i) => (
          <div key={i} style={{ background: '#fff', padding: '9px 10px' }}>
            <div style={{ fontSize: 10.5, fontWeight: 750, color: K.mut, whiteSpace: 'nowrap' }}>{l}</div>
            <div style={{ fontSize: 13, fontWeight: 800, color: K.ink, marginTop: 3, fontFamily: MONO, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{v}</div>
          </div>
        ))}
      </div>

      {/* 底部:过期/限制 + 操作 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10, marginTop: 11, paddingTop: 10, borderTop: '1px dashed #e4e7de' }}>
        <span style={{ fontFamily: MONO, fontSize: 11, fontWeight: 750, color: K.mut, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {expiry} · {limitText}
        </span>
        <div style={{ flex: '0 0 auto' }}>{renderOpsCell(record, ctx)}</div>
      </div>
    </div>
  );
}

export const getTokenMobileCardRender = ({
  t,
  showKeys,
  resolvedTokenKeys,
  loadingTokenKeys,
  toggleTokenVisibility,
  copyTokenKey,
  copyTokenConnectionString,
  manageToken,
  setEditingToken,
  setShowEdit,
  refresh,
  groupRatios = {},
}) => {
  const ctx = {
    t,
    showKeys,
    resolvedTokenKeys,
    loadingTokenKeys,
    toggleTokenVisibility,
    copyTokenKey,
    copyTokenConnectionString,
    manageToken,
    setEditingToken,
    setShowEdit,
    refresh,
  };
  return (record) => renderTokenMobileCard(record, ctx, groupRatios);
};

export const getTokensColumns = ({
  t,
  showKeys,
  resolvedTokenKeys,
  loadingTokenKeys,
  toggleTokenVisibility,
  copyTokenKey,
  copyTokenConnectionString,
  manageToken,
  onOpenLink,
  setEditingToken,
  setShowEdit,
  refresh,
  groupRatios = {},
}) => {
  const ctx = {
    t,
    showKeys,
    resolvedTokenKeys,
    loadingTokenKeys,
    toggleTokenVisibility,
    copyTokenKey,
    copyTokenConnectionString,
    manageToken,
    setEditingToken,
    setShowEdit,
    refresh,
  };
  return [
    {
      title: t('令牌名称'),
      dataIndex: 'name',
      width: 124,
      render: (text, record) => renderNameCell(record, t),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 78,
      render: (text, record) => renderStatusLight(record.status, t),
    },
    {
      title: t('密钥'),
      key: 'token_key',
      width: 194,
      render: (text, record) => renderKeyCell(record, ctx),
    },
    {
      title: t('额度'),
      key: 'quota_usage',
      width: 132,
      render: (text, record) => renderQuotaCell(record, t),
    },
    {
      title: t('分组'),
      dataIndex: 'group',
      width: 110,
      render: (text, record) => renderGroupCell(record, t, groupRatios),
    },
    {
      title: t('模型 / IP 限制'),
      key: 'limits',
      width: 156,
      render: (text, record) => renderLimitsCell(record, t),
    },
    {
      title: t('最后使用'),
      key: 'time',
      width: 132,
      render: (text, record) => renderTimeCell(record, t),
    },
    {
      title: t('操作'),
      dataIndex: 'operate',
      width: 116,
      align: 'center',
      render: (text, record) => renderOpsCell(record, ctx),
    },
  ];
};
