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
import { timestamp2string, renderQuota } from '../../../helpers';
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

const progressColor = (pct) => {
  if (pct >= 100) return '#2563eb';
  if (pct <= 10) return '#ef4444';
  if (pct <= 30) return '#f59e0b';
  return '#2563eb';
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
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 7, fontSize: 13, fontWeight: 500, color: c.color }}>
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
      <div style={{ fontWeight: 600, fontSize: 14, color: K.ink, wordBreak: 'break-all', lineHeight: 1.3 }}>
        {record.name || '-'}
      </div>
      <div style={{ marginTop: 6, fontSize: 13, color: K.ink2, whiteSpace: 'nowrap' }}>{expiry}</div>
    </div>
  );
}

// 剩余额度 / 总额度 + 进度条
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
        <span style={{ fontSize: 14, color: K.ink2, cursor: 'help' }}>{t('无限额度')}</span>
      </Popover>
    );
  }

  const percent = total > 0 ? (remain / total) * 100 : 0;
  const color = progressColor(percent);
  const R = 15;
  const C = 2 * Math.PI * R;
  const pct = Math.min(100, Math.max(0, percent));
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
      <div style={{ display: 'flex', alignItems: 'center', gap: 11, cursor: 'help' }}>
        <div style={{ position: 'relative', width: 38, height: 38, flex: '0 0 auto' }}>
          <svg width='38' height='38' viewBox='0 0 38 38'>
            <circle cx='19' cy='19' r={R} fill='none' stroke={K.line} strokeWidth='3' />
            <circle
              cx='19'
              cy='19'
              r={R}
              fill='none'
              stroke={color}
              strokeWidth='3'
              strokeLinecap='round'
              strokeDasharray={C}
              strokeDashoffset={C * (1 - pct / 100)}
              transform='rotate(-90 19 19)'
            />
          </svg>
          <span
            style={{
              position: 'absolute',
              inset: 0,
              display: 'grid',
              placeItems: 'center',
              fontFamily: MONO,
              fontSize: 10,
              fontWeight: 600,
              color: K.ink2,
              fontVariantNumeric: 'tabular-nums',
            }}
          >
            {percent.toFixed(0)}
          </span>
        </div>
        <div style={{ minWidth: 0 }}>
          <div
            style={{
              fontFamily: MONO,
              fontVariantNumeric: 'tabular-nums',
              fontSize: 15,
              fontWeight: 600,
              color: K.ink,
              lineHeight: 1.25,
              whiteSpace: 'nowrap',
            }}
          >
            {renderQuota(remain)}
          </div>
          <div
            style={{
              fontFamily: MONO,
              fontVariantNumeric: 'tabular-nums',
              fontSize: 15,
              fontWeight: 600,
              color: K.ink,
              marginTop: 2,
              lineHeight: 1.25,
              whiteSpace: 'nowrap',
            }}
          >
            {renderQuota(total)}
          </div>
        </div>
      </div>
    </Popover>
  );
}

// 分组 / 倍率(两行)
function renderGroupCell(record, t, groupRatios) {
  const group = record.group;
  const line = (label, value, valStyle) => (
    <div style={{ display: 'flex', alignItems: 'baseline', whiteSpace: 'nowrap' }}>
      <span style={{ color: K.ink2, fontSize: 12, flex: '0 0 auto' }}>{label}</span>
      <span style={{ color: K.ink2, fontSize: 14, ...valStyle }}>{value}</span>
    </div>
  );
  if (group === 'auto') {
    return (
      <div>
        <Tooltip content={t('当前分组为 auto，会自动选择最优分组，当一个组不可用时自动降级到下一个组（熔断机制）')}>
          {line(`${t('分组')}：`, t('智能熔断'), { color: '#7c3aed', fontWeight: 600, cursor: 'help' })}
        </Tooltip>
        <div style={{ marginTop: 6 }}>{line(`${t('倍率')}：`, t('自动'))}</div>
      </div>
    );
  }
  const ratio = groupRatios[group];
  return (
    <div>
      {line(`${t('分组')}：`, group || t('默认'))}
      <div style={{ marginTop: 6 }}>
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
      <span className='token-keyscroll' style={{ fontFamily: MONO, fontSize: 13, color: K.ink2, flex: 1, minWidth: 0, overflowX: 'auto', whiteSpace: 'nowrap' }}>
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
  const none = <span style={{ fontSize: 13, color: K.ink2 }}>{t('无限制')}</span>;
  // 去掉「模型」「IP」标题，只显示值(第一行模型、第二行 IP;IP 用等宽字体区分)
  const valLine = (items, isIp) =>
    items.length > 0 ? (
      <Tooltip content={items.join(', ')}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 5, minWidth: 0 }}>
          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0, fontSize: isIp ? 12 : 13, color: K.ink2, fontFamily: isIp ? MONO : undefined }}>{items[0]}</span>
          {items.length > 1 && (
            <span style={{ display: 'inline-flex', alignItems: 'center', height: 16, padding: '0 5px', borderRadius: 5, background: '#f1f5f9', color: K.ink2, fontSize: 11, fontWeight: 600, flex: '0 0 auto' }}>+{items.length - 1}</span>
          )}
        </span>
      </Tooltip>
    ) : none;
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center' }}>{valLine(models, false)}</div>
      <div style={{ marginTop: 7, display: 'flex', alignItems: 'center' }}>{valLine(ips, true)}</div>
    </div>
  );
}

// 创建 / 最后使用(两行同色同大小,精确到分钟)
function renderTimeCell(record, t) {
  const fmt = (ts) => String(timestamp2string(ts)).slice(0, 16);
  return (
    <div>
      <div style={{ fontFamily: MONO, fontVariantNumeric: 'tabular-nums', fontSize: 13, color: K.ink2, whiteSpace: 'nowrap' }}>
        {fmt(record.created_time)}
      </div>
      <div style={{ fontFamily: MONO, fontVariantNumeric: 'tabular-nums', fontSize: 13, marginTop: 5, whiteSpace: 'nowrap', color: record.accessed_time ? K.ink2 : K.mut2 }}>
        {record.accessed_time ? fmt(record.accessed_time) : t('从未使用')}
      </div>
    </div>
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
      title: t('名称'),
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
      title: t('剩余额度 / 总额度'),
      key: 'quota_usage',
      width: 144,
      render: (text, record) => renderQuotaCell(record, t),
    },
    {
      title: t('分组'),
      dataIndex: 'group',
      width: 110,
      render: (text, record) => renderGroupCell(record, t, groupRatios),
    },
    {
      title: t('密钥'),
      key: 'token_key',
      width: 176,
      render: (text, record) => renderKeyCell(record, ctx),
    },
    {
      title: t('模型 / IP 限制'),
      key: 'limits',
      width: 150,
      render: (text, record) => renderLimitsCell(record, t),
    },
    {
      title: t('创建 / 最后使用'),
      key: 'time',
      width: 150,
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
