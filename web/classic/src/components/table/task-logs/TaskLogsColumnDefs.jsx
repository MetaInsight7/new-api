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
import { Avatar, Tooltip } from '@douyinfe/semi-ui';
import {
  TASK_ACTION_FIRST_TAIL_GENERATE,
  TASK_ACTION_GENERATE,
  TASK_ACTION_REFERENCE_GENERATE,
  TASK_ACTION_TEXT_GENERATE,
  TASK_ACTION_REMIX_GENERATE,
} from '../../../constants/common.constant';
import { CHANNEL_OPTIONS } from '../../../constants/channel.constants';
import { stringToColor } from '../../../helpers/render';

// 与令牌 / 使用日志一致的账本式视觉语言(DMIT 扁平表)
const MONO = '"SFMono-Regular", ui-monospace, Menlo, monospace';
const K = {
  ink: '#141a1f',
  ink2: '#3c4650',
  mut: '#6b7686',
  mut2: '#9aa4b2',
  line: '#eef1f5',
};

const VIDEO_ACTIONS = [
  TASK_ACTION_GENERATE,
  TASK_ACTION_TEXT_GENERATE,
  TASK_ACTION_FIRST_TAIL_GENERATE,
  TASK_ACTION_REFERENCE_GENERATE,
  TASK_ACTION_REMIX_GENERATE,
];

// 类型(action)→ 文案 + 点色
function getActionMeta(action, t) {
  const video = { color: '#2563eb' };
  switch (action) {
    case 'MUSIC':
      return { text: t('生成音乐'), color: '#6b7686' };
    case 'LYRICS':
      return { text: t('生成歌词'), color: '#be185d' };
    case TASK_ACTION_GENERATE:
      return { text: t('图生视频'), ...video };
    case TASK_ACTION_TEXT_GENERATE:
      return { text: t('文生视频'), ...video };
    case TASK_ACTION_FIRST_TAIL_GENERATE:
      return { text: t('首尾生视频'), ...video };
    case TASK_ACTION_REFERENCE_GENERATE:
      return { text: t('参照生视频'), ...video };
    case TASK_ACTION_REMIX_GENERATE:
      return { text: t('视频Remix'), ...video };
    default:
      return { text: t('未知'), color: '#9aa4b2' };
  }
}

function getPlatformText(platform, t) {
  const opt = CHANNEL_OPTIONS.find(
    (o) => String(o.value) === String(platform),
  );
  if (opt) return opt.label;
  if (platform === 'suno') return 'Suno';
  return platform || '';
}

// 状态 → 信号灯配置(live=脉冲涟漪,用于进行/排队等活跃态)
function getStatusMeta(status, t) {
  const M = {
    SUCCESS: { text: t('成功'), dot: '#10b981', glow: 'rgba(16,185,129,.16)', color: '#0f9d6e', live: false, pct: 100 },
    FAILURE: { text: t('失败'), dot: '#ef4444', glow: 'rgba(239,68,68,.16)', color: '#b91c1c', live: false },
    IN_PROGRESS: { text: t('执行中'), dot: '#3b82f6', glow: 'rgba(59,130,246,.16)', color: '#2563eb', live: true },
    SUBMITTED: { text: t('队列中'), dot: '#f59e0b', glow: 'rgba(245,158,11,.16)', color: '#b45309', live: true },
    QUEUED: { text: t('排队中'), dot: '#f97316', glow: 'rgba(249,115,22,.16)', color: '#c2410c', live: true },
    NOT_START: { text: t('未启动'), dot: '#94a3b8', glow: 'rgba(148,163,184,.16)', color: '#6b7686', live: false, pct: 0 },
    '': { text: t('正在提交'), dot: '#94a3b8', glow: 'rgba(148,163,184,.16)', color: '#6b7686', live: true },
    UNKNOWN: { text: t('未知'), dot: '#94a3b8', glow: 'rgba(148,163,184,.16)', color: '#6b7686', live: false },
  };
  return M[status] || M.UNKNOWN;
}

function fmtMinute(tsSeconds) {
  const d = new Date(tsSeconds * 1000);
  const p = (n) => ('0' + n).slice(-2);
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

// 类型 / 任务(点色文案 + 平台徽标 / 任务 ID 单行代码)
function renderTaskCell(record, ctx) {
  const a = getActionMeta(record.action, ctx.t);
  const platform = getPlatformText(record.platform, ctx.t);
  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 7, fontSize: 14, color: K.ink, fontWeight: 500, whiteSpace: 'nowrap' }}>
          <span style={{ width: 7, height: 7, borderRadius: '50%', flex: '0 0 auto', background: a.color }} />
          {a.text}
        </span>
        {platform && (
          <span style={{ display: 'inline-flex', alignItems: 'center', height: 18, padding: '0 7px', borderRadius: 5, background: '#f1f5f9', color: K.ink2, fontSize: 11, fontWeight: 600, flex: '0 0 auto' }}>
            {platform}
          </span>
        )}
      </div>
      <div
        title={record.task_id}
        onClick={(e) => {
          e.stopPropagation();
          ctx.openContentModal(JSON.stringify(record, null, 2));
        }}
        style={{ marginTop: 6, fontSize: 12, fontFamily: MONO, color: K.mut, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', cursor: 'pointer' }}
      >
        {record.task_id || '-'}
      </div>
    </div>
  );
}

// 渠道(管理员):#id 代码,点按复制
function renderChannelCell(record, ctx) {
  const id = record.channel_id;
  if (id === undefined || id === null || id === '') {
    return <span style={{ fontSize: 13, color: K.mut2 }}>-</span>;
  }
  return (
    <span
      onClick={(e) => {
        e.stopPropagation();
        ctx.copyText(String(id));
      }}
      style={{ fontFamily: MONO, fontSize: 14, fontWeight: 500, color: K.ink2, cursor: 'pointer', whiteSpace: 'nowrap' }}
    >
      #{id}
    </span>
  );
}

// 用户(管理员):头像 + 用户名
function renderUserCell(record, ctx) {
  const name = String(record.username || '?');
  const clickable = record.user_id !== undefined && record.user_id !== null;
  return (
    <span
      style={{ display: 'inline-flex', alignItems: 'center', gap: 8, cursor: clickable ? 'pointer' : 'default', minWidth: 0 }}
      onClick={(e) => {
        if (!clickable) return;
        e.stopPropagation();
        ctx.showUserInfoFunc?.(record.user_id);
      }}
    >
      <Avatar size='extra-small' color={stringToColor(name)} style={{ fontSize: 11, flex: '0 0 auto' }}>
        {name.slice(0, 1)}
      </Avatar>
      <span style={{ fontSize: 14, color: K.ink, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
        {name}
      </span>
    </span>
  );
}

// 状态 / 进度(信号灯[活跃态涟漪] + 细进度条)
function renderStatusCell(record, ctx) {
  const s = getStatusMeta(record.status, ctx.t);
  // 进度条只在「执行中」有意义:成功/失败/排队/未启动等只显示状态灯
  let pct = null;
  if (record.status === 'IN_PROGRESS') {
    const raw = record.progress;
    if (typeof raw === 'string' && raw.includes('%')) {
      const n = parseInt(raw.replace('%', ''));
      if (!isNaN(n)) pct = n;
    }
  }
  const light = (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 7, fontSize: 13, fontWeight: 500, color: s.color }}>
      <span className={'dmit-sig' + (s.live ? ' dmit-sig--live' : '')} style={{ ['--dot']: s.dot, ['--glow']: s.glow }} />
      {s.text}
    </span>
  );
  // 无进度条 → 单行状态灯在单元格内垂直居中,不顶对齐悬空
  if (pct === null) {
    return <div style={{ display: 'flex', alignItems: 'center', minHeight: 40 }}>{light}</div>;
  }
  return (
    <div>
      {light}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 7 }}>
        <div style={{ flex: '0 0 auto', width: 96, height: 4, background: K.line, borderRadius: 2, overflow: 'hidden' }}>
          <div style={{ width: `${Math.min(100, Math.max(0, pct))}%`, height: '100%', background: '#2563eb', borderRadius: 2 }} />
        </div>
        <span style={{ fontSize: 11, color: K.mut2, fontFamily: MONO, fontVariantNumeric: 'tabular-nums' }}>{pct}%</span>
      </div>
    </div>
  );
}

// 提交 / 耗时
function renderTimeCell(record, ctx) {
  const submit = record.submit_time ? fmtMinute(record.submit_time) : '-';
  const dur =
    record.submit_time && record.finish_time ? record.finish_time - record.submit_time : null;
  return (
    <div>
      <div style={{ fontFamily: MONO, fontVariantNumeric: 'tabular-nums', fontSize: 13, color: K.ink2, whiteSpace: 'nowrap' }}>
        {submit}
      </div>
      <div style={{ marginTop: 5, fontSize: 12, color: K.mut2, fontFamily: MONO, whiteSpace: 'nowrap' }}>
        {dur !== null ? `${ctx.t('耗时')} ${dur}s` : '—'}
      </div>
    </div>
  );
}

// 详情(预览视频 / 预览音乐 / 失败详情 / 无)
const detailLink = (text, onClick, arrow) => (
  <span
    onClick={(e) => {
      e.stopPropagation();
      onClick();
    }}
    style={{ display: 'inline-flex', alignItems: 'center', gap: 2, color: '#3557d6', fontSize: 13, fontWeight: 500, cursor: 'pointer', whiteSpace: 'nowrap' }}
  >
    {text}
    {arrow && <span style={{ fontSize: 13 }}>›</span>}
  </span>
);

function renderDetailCell(record, ctx) {
  const t = ctx.t;
  const isSunoSuccess =
    record.platform === 'suno' &&
    record.status === 'SUCCESS' &&
    Array.isArray(record.data) &&
    record.data.some((c) => c.audio_url);
  if (isSunoSuccess) {
    return detailLink(t('预览音乐'), () => ctx.openAudioModal(record.data));
  }

  const isVideoTask = VIDEO_ACTIONS.includes(record.action);
  const resultUrl = record.result_url;
  const hasResultUrl = typeof resultUrl === 'string' && /^https?:\/\//.test(resultUrl);
  if (record.status === 'SUCCESS' && isVideoTask && hasResultUrl) {
    return detailLink(t('预览视频'), () => ctx.openVideoModal(resultUrl));
  }

  if (record.fail_reason) {
    return (
      <Tooltip content={String(record.fail_reason).slice(0, 200)}>
        {detailLink(t('详情'), () => ctx.openContentModal(record.fail_reason), true)}
      </Tooltip>
    );
  }
  return <span style={{ fontSize: 13, color: K.mut2 }}>{t('无')}</span>;
}

export const getTaskLogsColumns = ({
  t,
  COLUMN_KEYS,
  copyText,
  openContentModal,
  isAdminUser,
  openVideoModal,
  openAudioModal,
  showUserInfoFunc,
}) => {
  const ctx = {
    t,
    copyText,
    openContentModal,
    openVideoModal,
    openAudioModal,
    showUserInfoFunc,
    isAdminUser,
  };

  const columns = [
    {
      key: COLUMN_KEYS.TASK,
      title: t('类型 / 任务'),
      dataIndex: 'task_id',
      width: isAdminUser ? 258 : 300,
      render: (_text, record) => renderTaskCell(record, ctx),
    },
  ];

  if (isAdminUser) {
    columns.push({
      key: COLUMN_KEYS.CHANNEL,
      title: t('渠道'),
      dataIndex: 'channel_id',
      width: 88,
      render: (_text, record) => renderChannelCell(record, ctx),
    });
    columns.push({
      key: COLUMN_KEYS.USERNAME,
      title: t('用户'),
      dataIndex: 'username',
      width: 140,
      render: (_text, record) => renderUserCell(record, ctx),
    });
  }

  columns.push({
    key: COLUMN_KEYS.STATUS,
    title: t('状态 / 进度'),
    dataIndex: 'status',
    width: isAdminUser ? 190 : 210,
    render: (_text, record) => renderStatusCell(record, ctx),
  });
  columns.push({
    key: COLUMN_KEYS.TIME,
    title: t('提交 / 耗时'),
    dataIndex: 'submit_time',
    width: isAdminUser ? 150 : 170,
    render: (_text, record) => renderTimeCell(record, ctx),
  });
  columns.push({
    key: COLUMN_KEYS.DETAIL,
    title: t('详情'),
    dataIndex: 'fail_reason',
    width: isAdminUser ? 110 : 120,
    render: (_text, record) => renderDetailCell(record, ctx),
  });

  return columns;
};
