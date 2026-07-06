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
import {
  Avatar,
  Space,
  Tag,
  Tooltip,
  Popover,
  Typography,
} from '@douyinfe/semi-ui';
import {
  renderGroup,
  renderQuota,
  stringToColor,
  getLogOther,
  renderModelTag,
  renderModelPriceSimple,
  renderTieredModelPriceSimple,
} from '../../../helpers';
import { IconHelpCircle } from '@douyinfe/semi-icons';
import { CircleAlert, Route, Sparkles } from 'lucide-react';

const colors = [
  'amber',
  'blue',
  'cyan',
  'green',
  'grey',
  'indigo',
  'light-blue',
  'lime',
  'orange',
  'pink',
  'purple',
  'red',
  'teal',
  'violet',
  'yellow',
];

function formatRatio(ratio) {
  if (ratio === undefined || ratio === null) {
    return '-';
  }
  if (typeof ratio === 'number') {
    return ratio.toFixed(4);
  }
  return String(ratio);
}

function buildChannelAffinityTooltip(affinity, t) {
  if (!affinity) {
    return null;
  }

  const keySource = affinity.key_source || '-';
  const keyPath = affinity.key_path || affinity.key_key || '-';
  const keyHint = affinity.key_hint || '';
  const keyFp = affinity.key_fp ? `#${affinity.key_fp}` : '';
  const keyText = `${keySource}:${keyPath}${keyFp}`;

  const lines = [
    t('渠道亲和性'),
    `${t('规则')}：${affinity.rule_name || '-'}`,
    `${t('分组')}：${affinity.selected_group || '-'}`,
    `${t('Key')}：${keyText}`,
    ...(keyHint ? [`${t('Key 摘要')}：${keyHint}`] : []),
  ];

  return (
    <div style={{ lineHeight: 1.6, display: 'flex', flexDirection: 'column' }}>
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  );
}

// Render functions
function renderType(type, t) {
  switch (type) {
    case 1:
      return (
        <Tag color='cyan' shape='circle'>
          {t('充值')}
        </Tag>
      );
    case 2:
      return (
        <Tag color='lime' shape='circle'>
          {t('消费')}
        </Tag>
      );
    case 3:
      return (
        <Tag color='orange' shape='circle'>
          {t('管理')}
        </Tag>
      );
    case 4:
      return (
        <Tag color='purple' shape='circle'>
          {t('系统')}
        </Tag>
      );
    case 5:
      return (
        <Tag color='red' shape='circle'>
          {t('错误')}
        </Tag>
      );
    case 6:
      return (
        <Tag color='teal' shape='circle'>
          {t('退款')}
        </Tag>
      );
    default:
      return (
        <Tag color='grey' shape='circle'>
          {t('未知')}
        </Tag>
      );
  }
}

function buildStreamStatusTooltip(ss, t) {
  if (!ss) return null;
  const lines = [
    t('流状态') + '：' + t('异常'),
    (ss.end_reason || 'unknown'),
  ];
  if (ss.error_count > 0) {
    lines.push(`${t('软错误')}: ${ss.error_count}`);
  }
  if (ss.end_error) {
    lines.push(ss.end_error);
  }
  return (
    <div style={{ lineHeight: 1.6, display: 'flex', flexDirection: 'column' }}>
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
    </div>
  );
}

function renderIsStream(bool, t, streamStatus) {
  const isError = streamStatus && streamStatus.status !== 'ok';

  if (bool) {
    return (
      <span style={{ position: 'relative', display: 'inline-block' }}>
        <Tag color='blue' shape='circle'>
          {t('流')}
        </Tag>
        {isError && (
          <Tooltip content={buildStreamStatusTooltip(streamStatus, t)}>
            <span
              style={{
                position: 'absolute',
                right: -4,
                top: -4,
                lineHeight: 1,
                color: '#ef4444',
                cursor: 'pointer',
                userSelect: 'none',
              }}
            >
              <CircleAlert
                size={14}
                strokeWidth={2.5}
                color='currentColor'
              />
            </span>
          </Tooltip>
        )}
      </span>
    );
  } else {
    return (
      <Tag color='purple' shape='circle'>
        {t('非流')}
      </Tag>
    );
  }
}

function renderUseTime(type, t) {
  const time = parseInt(type);
  if (time < 101) {
    return (
      <Tag color='green' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else if (time < 300) {
    return (
      <Tag color='orange' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else {
    return (
      <Tag color='red' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  }
}

function renderFirstUseTime(type, t) {
  let time = parseFloat(type) / 1000.0;
  time = time.toFixed(1);
  if (time < 3) {
    return (
      <Tag color='green' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else if (time < 10) {
    return (
      <Tag color='orange' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  } else {
    return (
      <Tag color='red' shape='circle'>
        {' '}
        {time} s{' '}
      </Tag>
    );
  }
}

function renderBillingTag(record, t) {
  const other = getLogOther(record.other);
  if (other?.billing_source === 'subscription') {
    return (
      <Tag color='green' shape='circle'>
        {t('订阅抵扣')}
      </Tag>
    );
  }
  return null;
}

function renderModelName(record, copyText, t) {
  let other = getLogOther(record.other);
  let modelMapped =
    other?.is_model_mapped &&
    other?.upstream_model_name &&
    other?.upstream_model_name !== '';
  if (!modelMapped) {
    return renderModelTag(record.model_name, {
      onClick: (event) => {
        copyText(event, record.model_name).then((r) => {});
      },
    });
  } else {
    return (
      <>
        <Space vertical align={'start'}>
          <Popover
            content={
              <div style={{ padding: 10 }}>
                <Space vertical align={'start'}>
                  <div className='flex items-center'>
                    <Typography.Text strong style={{ marginRight: 8 }}>
                      {t('请求并计费模型')}:
                    </Typography.Text>
                    {renderModelTag(record.model_name, {
                      onClick: (event) => {
                        copyText(event, record.model_name).then((r) => {});
                      },
                    })}
                  </div>
                  <div className='flex items-center'>
                    <Typography.Text strong style={{ marginRight: 8 }}>
                      {t('实际模型')}:
                    </Typography.Text>
                    {renderModelTag(other.upstream_model_name, {
                      onClick: (event) => {
                        copyText(event, other.upstream_model_name).then(
                          (r) => {},
                        );
                      },
                    })}
                  </div>
                </Space>
              </div>
            }
          >
            {renderModelTag(record.model_name, {
              onClick: (event) => {
                copyText(event, record.model_name).then((r) => {});
              },
              suffixIcon: (
                <Route
                  style={{ width: '0.9em', height: '0.9em', opacity: 0.75 }}
                />
              ),
            })}
          </Popover>
        </Space>
      </>
    );
  }
}

function toTokenNumber(value) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0;
  }
  return parsed;
}

function formatTokenCount(value) {
  return toTokenNumber(value).toLocaleString();
}

function getPromptCacheSummary(other) {
  if (!other || typeof other !== 'object') {
    return null;
  }

  const cacheReadTokens = toTokenNumber(other.cache_tokens);
  const cacheCreationTokens = toTokenNumber(other.cache_creation_tokens);
  const cacheCreationTokens5m = toTokenNumber(other.cache_creation_tokens_5m);
  const cacheCreationTokens1h = toTokenNumber(other.cache_creation_tokens_1h);

  const hasSplitCacheCreation =
    cacheCreationTokens5m > 0 || cacheCreationTokens1h > 0;
  const cacheWriteTokens = hasSplitCacheCreation
    ? cacheCreationTokens5m + cacheCreationTokens1h
    : cacheCreationTokens;

  if (cacheReadTokens <= 0 && cacheWriteTokens <= 0) {
    return null;
  }

  return {
    cacheReadTokens,
    cacheWriteTokens,
  };
}

function normalizeDetailText(detail) {
  return String(detail || '')
    .replace(/\n\r/g, '\n')
    .replace(/\r\n/g, '\n');
}

function getUsageLogGroupSummary(groupRatio, userGroupRatio, t) {
  const parsedUserGroupRatio = Number(userGroupRatio);
  const useUserGroupRatio =
    Number.isFinite(parsedUserGroupRatio) && parsedUserGroupRatio !== -1;
  const ratio = useUserGroupRatio ? userGroupRatio : groupRatio;
  if (ratio === undefined || ratio === null || ratio === '') {
    return '';
  }
  return `${useUserGroupRatio ? t('专属倍率') : t('分组')} ${formatRatio(ratio)}x`;
}

function renderCompactDetailSummary(summarySegments) {
  const segments = Array.isArray(summarySegments)
    ? summarySegments.filter((segment) => segment?.text)
    : [];
  if (!segments.length) {
    return null;
  }

  return (
    <div
      style={{
        maxWidth: 180,
        lineHeight: 1.35,
      }}
    >
      {segments.map((segment, index) => (
        <Typography.Text
          key={`${segment.text}-${index}`}
          type={segment.tone === 'secondary' ? 'tertiary' : undefined}
          size={segment.tone === 'secondary' ? 'small' : undefined}
          style={{
            display: 'block',
            maxWidth: '100%',
            fontSize: 13,
            marginTop: index === 0 ? 0 : 2,
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          {segment.text}
        </Typography.Text>
      ))}
    </div>
  );
}

function getUsageLogDetailSummary(record, text, billingDisplayMode, t) {
  const other = getLogOther(record.other);

  if (record.type === 6) {
    return {
      segments: [{ text: t('异步任务退款'), tone: 'primary' }],
    };
  }

  if (other == null || record.type !== 2) {
    return null;
  }

  if (
    other?.violation_fee === true ||
    Boolean(other?.violation_fee_code) ||
    Boolean(other?.violation_fee_marker)
  ) {
    const feeQuota = other?.fee_quota ?? record?.quota;
    const groupText = getUsageLogGroupSummary(
      other?.group_ratio,
      other?.user_group_ratio,
      t,
    );
    return {
      segments: [
        groupText ? { text: groupText, tone: 'primary' } : null,
        { text: t('违规扣费'), tone: 'primary' },
        {
          text: `${t('扣费')}：${renderQuota(feeQuota, 6)}`,
          tone: 'secondary',
        },
        text ? { text: `${t('详情')}：${text}`, tone: 'secondary' } : null,
      ].filter(Boolean),
    };
  }

  const summaryOpts = { ...other, displayMode: billingDisplayMode, outputMode: 'segments' };

  if (other?.billing_mode === 'tiered_expr') {
    return { segments: renderTieredModelPriceSimple(summaryOpts) };
  }

  return {
    segments: other?.claude
      ? renderModelPriceSimple({ ...summaryOpts, provider: 'claude' })
      : renderModelPriceSimple({ ...summaryOpts, provider: 'openai' }),
  };
}

// ---------------------------------------------------------------------------
// Composite-column renderers (DMIT ledger redesign — see .tmp/log-roles5.html /
// .tmp/log-admin.html prototypes). Merges the legacy 14 columns into role-aware
// composite cells. Inline styles mirror the prototype px values exactly.
// ---------------------------------------------------------------------------

const CELL_MONO = '"SFMono-Regular", ui-monospace, Menlo, monospace';
const CC = {
  ink: '#141a1f',
  ink2: '#3c4650',
  mut: '#6b7686',
  mut2: '#9aa4b2',
  purple: '#7c3aed',
};

const CONSUMPTION_TYPES = [0, 2, 5, 6];
const isConsumptionRow = (record) => CONSUMPTION_TYPES.includes(record?.type);

function compactRatio(r) {
  const n = Number(r);
  if (!Number.isFinite(n)) return null;
  return (Number.isInteger(n) ? n : parseFloat(n.toFixed(2))).toString();
}

function getGroupRatioValue(other) {
  const u = Number(other?.user_group_ratio);
  const useU = Number.isFinite(u) && u !== -1;
  const ratio = useU ? other?.user_group_ratio : other?.group_ratio;
  return compactRatio(ratio);
}

function getGroupName(record, other) {
  return record?.group || other?.group || '';
}

function renderRatioBadge(ratioText, isAdminUser) {
  if (!ratioText) return null;
  if (isAdminUser) {
    return (
      <span
        style={{
          display: 'inline-flex', alignItems: 'center', height: 16,
          padding: '0 6px', border: '1px solid #c7d2fe', borderRadius: 5,
          color: '#4f46e5', fontSize: 11, fontWeight: 600, lineHeight: 1,
          marginLeft: 4,
        }}
      >
        {ratioText}x
      </span>
    );
  }
  return (
    <span style={{ color: CC.mut, fontWeight: 600, marginLeft: 4 }}>
      {ratioText}x
    </span>
  );
}

function getTypeChipStyle(type, t) {
  const map = {
    1: { text: t('充值'), color: '#b45309', bg: '#fff7ed' },
    2: { text: t('消费'), color: '#2563eb', bg: '#eff4ff' },
    3: { text: t('管理'), color: '#6b7686', bg: '#f1f5f9' },
    4: { text: t('系统'), color: '#7c3aed', bg: '#f3efff' },
    5: { text: t('错误'), color: '#b91c1c', bg: '#fef2f2' },
    6: { text: t('退款'), color: '#0f9d6e', bg: '#ecfdf5' },
  };
  return map[type] || { text: t('未知'), color: '#6b7686', bg: '#f1f5f9' };
}

function renderTypeChip(type, t) {
  const c = getTypeChipStyle(type, t);
  return (
    <span
      style={{
        display: 'inline-flex', alignItems: 'center', height: 19,
        padding: '0 9px', borderRadius: 6, fontSize: 12, fontWeight: 600,
        color: c.color, background: c.bg,
      }}
    >
      {c.text}
    </span>
  );
}

function renderStreamTag(record, t) {
  if (!(record.type === 2 || record.type === 5)) return null;
  if (!record.is_stream) return null; // 非流 is shown in the 用时 cell instead
  const other = getLogOther(record.other);
  const ss = other?.stream_status;
  const isErr = ss && ss.status !== 'ok';
  return (
    <span
      style={{
        position: 'relative', display: 'inline-flex', alignItems: 'center',
        height: 19, padding: '0 8px', borderRadius: 999, background: '#eff4ff',
        color: '#2563eb', fontSize: 11, fontWeight: 600,
      }}
    >
      {t('流')}
      {isErr && (
        <Tooltip content={buildStreamStatusTooltip(ss, t)}>
          <span
            style={{
              position: 'absolute', right: -3, top: -3, width: 7, height: 7,
              borderRadius: '50%', background: '#ef4444', cursor: 'pointer',
            }}
          />
        </Tooltip>
      )}
    </span>
  );
}

function renderTimeCell(record) {
  const s = String(record.timestamp2string || '');
  const sp = s.split(' ');
  const time = sp.length > 1 ? sp[1] : s;
  const date = sp.length > 1 ? sp[0] : '';
  return (
    <div>
      <div style={{ fontSize: 13, color: CC.ink2, fontFamily: CELL_MONO, fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }}>
        {time}
      </div>
      {date && (
        <div style={{ fontSize: 11.5, color: CC.mut2, marginTop: 3, fontFamily: CELL_MONO, fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' }}>
          {date}
        </div>
      )}
    </div>
  );
}

function renderTokenGroupCell(record, ctx) {
  if (!isConsumptionRow(record)) return null;
  const other = getLogOther(record.other);
  const group = getGroupName(record, other);
  const ratio = getGroupRatioValue(other);
  return (
    <div>
      <div style={{ fontWeight: 500, color: CC.ink }} onClick={(e) => ctx.copyText(e, record.token_name)}>
        {record.token_name}
      </div>
      <div style={{ fontSize: 12, color: CC.mut2, marginTop: 4 }}>
        {group}
        {renderRatioBadge(ratio, false)}
      </div>
    </div>
  );
}

function renderUserCell(record, ctx) {
  const other = getLogOther(record.other);
  const group = getGroupName(record, other);
  const ratio = getGroupRatioValue(other);
  return (
    <div>
      <div
        style={{ fontWeight: 500, fontSize: 14, color: CC.ink, cursor: 'pointer' }}
        onClick={(e) => { e.stopPropagation(); ctx.showUserInfoFunc?.(record.user_id); }}
      >
        {record.username}
      </div>
      {isConsumptionRow(record) && (
        <div style={{ fontSize: 12, color: CC.mut2, marginTop: 4 }}>
          {record.token_name}
          {group ? ` · ${group}` : ''}
          {renderRatioBadge(ratio, true)}
        </div>
      )}
    </div>
  );
}

function renderChannelCell(record, ctx) {
  if (!isConsumptionRow(record)) return null;
  const other = getLogOther(record.other);
  const adminInfo = other?.admin_info || {};
  const useChannel = Array.isArray(adminInfo.use_channel) ? adminInfo.use_channel : null;
  const retried = useChannel && useChannel.length > 1;
  const affinity = adminInfo.channel_affinity;
  const isMultiKey = adminInfo?.is_multi_key;
  return (
    <div>
      <div style={{ whiteSpace: 'nowrap' }}>
        <Tooltip content={record.channel_name || ctx.t('未知渠道')}>
          <span style={{ fontWeight: 500, fontFamily: CELL_MONO, fontSize: 14, color: CC.ink2 }}>#{record.channel}</span>
        </Tooltip>
        {record.channel_name && (
          <span style={{ fontSize: 13, color: CC.ink, marginLeft: 5 }}>{record.channel_name}</span>
        )}
        {isMultiKey && (
          <span style={{ fontSize: 12, color: CC.mut2, marginLeft: 4 }}>key#{adminInfo.multi_key_index}</span>
        )}
      </div>
      {(retried || affinity) && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 5, fontSize: 12, color: CC.mut2, fontFamily: CELL_MONO, whiteSpace: 'nowrap' }}>
          {retried && (
            <span style={{ display: 'inline-flex', alignItems: 'center', height: 16, padding: '0 5px', borderRadius: 5, background: '#fff7ed', color: '#c2410c', fontSize: 11, fontWeight: 600 }}>
              {ctx.t('重试')} {useChannel.length - 1}
            </span>
          )}
          {retried && <span>{useChannel.map((c) => `#${c}`).join('→')}</span>}
          {affinity && (
            <Tooltip content={buildChannelAffinityTooltip(affinity, ctx.t)}>
              <span
                style={{ display: 'inline-flex', alignItems: 'center', gap: 3, color: '#6d28d9', cursor: 'pointer', fontFamily: 'inherit' }}
                onClick={(e) => { e.stopPropagation(); ctx.openChannelAffinityUsageCacheModal?.(affinity); }}
              >
                <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#7c3aed' }} />
                {ctx.t('亲和')}
              </span>
            </Tooltip>
          )}
        </div>
      )}
    </div>
  );
}

function renderReqModelCell(record, ctx) {
  const hasModel = record.model_name && isConsumptionRow(record);
  if (!hasModel) {
    // 非消费行(充值/管理/系统等):类型标签 + 描述文本。
    // 充值金额记录在 content 里(如「充值额度: 10，支付金额: 73.00」)，quota 恒为 0。
    return (
      <div>
        <div>{renderTypeChip(record.type, ctx.t)}</div>
        {record.content && (
          <div
            style={{
              marginTop: 6,
              fontSize: 13,
              color: CC.ink,
              fontWeight: 500,
              lineHeight: 1.45,
              wordBreak: 'break-all',
            }}
          >
            {record.content}
          </div>
        )}
      </div>
    );
  }
  const other = getLogOther(record.other);
  const dotColor = getTypeChipStyle(record.type, ctx.t).color;
  const modelMapped = other?.is_model_mapped && other?.upstream_model_name;
  const showRedirect = ctx.isAdminUser && modelMapped;
  return (
    <div>
      <div style={{ fontSize: 14, color: CC.ink, fontWeight: 500, wordBreak: 'break-all', lineHeight: 1.35 }}>
        <span style={{ width: 7, height: 7, borderRadius: '50%', display: 'inline-block', verticalAlign: 'middle', marginRight: 7, background: dotColor }} />
        <span onClick={(e) => ctx.copyText(e, record.model_name)}>{record.model_name}</span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 6 }}>
        {renderTypeChip(record.type, ctx.t)}
        {renderStreamTag(record, ctx.t)}
        {showRedirect && (
          <Popover
            content={
              <div style={{ padding: 10, lineHeight: 1.8 }}>
                <div>
                  <Typography.Text strong style={{ marginRight: 6 }}>{ctx.t('请求并计费模型')}:</Typography.Text>
                  {record.model_name}
                </div>
                <div>
                  <Typography.Text strong style={{ marginRight: 6 }}>{ctx.t('实际模型')}:</Typography.Text>
                  {other.upstream_model_name}
                </div>
              </div>
            }
          >
            <span style={{ display: 'inline-flex', alignItems: 'center', height: 19, padding: '0 8px', borderRadius: 5, background: '#f3efff', color: '#7c3aed', fontSize: 11, fontWeight: 600, cursor: 'pointer' }}>
              {ctx.t('实际模型')}
            </span>
          </Popover>
        )}
      </div>
    </div>
  );
}

function renderUsageCell(record, ctx) {
  if (!isConsumptionRow(record)) return null;
  const other = getLogOther(record.other);
  const cache = getPromptCacheSummary(other);
  return (
    <div>
      <div style={{ display: 'flex', gap: 18, alignItems: 'baseline', lineHeight: 1.5, color: CC.ink, flexWrap: 'nowrap' }}>
        <span style={{ whiteSpace: 'nowrap' }}>
          <span style={{ color: CC.ink, fontSize: 13 }}>{ctx.t('输入')}：</span>
          <span style={{ fontFamily: CELL_MONO, fontVariantNumeric: 'tabular-nums', fontSize: 14, color: CC.ink }}>{formatTokenCount(record.prompt_tokens)}</span>
        </span>
        <span style={{ whiteSpace: 'nowrap' }}>
          <span style={{ color: CC.ink, fontSize: 13 }}>{ctx.t('输出')}：</span>
          <span style={{ fontFamily: CELL_MONO, fontVariantNumeric: 'tabular-nums', fontSize: 14, color: CC.ink }}>{formatTokenCount(record.completion_tokens)}</span>
        </span>
      </div>
      {cache ? (
        <div style={{ display: 'flex', gap: 12, alignItems: 'baseline', marginTop: 5, fontSize: 12, color: CC.mut2, flexWrap: 'nowrap' }}>
          <span style={{ whiteSpace: 'nowrap' }}>
            <span style={{ color: CC.mut }}>{ctx.t('缓存')} ·</span>
            {ctx.t('输入')}：<span style={{ fontFamily: CELL_MONO, color: CC.purple, fontWeight: 600 }}>{formatTokenCount(cache.cacheReadTokens)}</span>
          </span>
          {cache.cacheWriteTokens > 0 && (
            <span style={{ whiteSpace: 'nowrap' }}>
              {ctx.t('输出')}：<span style={{ fontFamily: CELL_MONO, color: CC.purple, fontWeight: 600 }}>{formatTokenCount(cache.cacheWriteTokens)}</span>
            </span>
          )}
        </div>
      ) : (
        <div style={{ marginTop: 5, fontSize: 12, color: CC.mut2 }}>{ctx.t('无缓存')}</div>
      )}
    </div>
  );
}

function renderUseTimeCell(record, ctx) {
  if (!(record.type === 2 || record.type === 5)) return null;
  const other = getLogOther(record.other);
  const useTime = parseInt(record.use_time);
  const frt = other?.frt;
  let sub = null;
  if (record.is_stream && frt) {
    sub = `${ctx.t('首字')} ${(parseFloat(frt) / 1000).toFixed(1)}s`;
  } else if (!record.is_stream) {
    sub = ctx.t('非流');
  }
  return (
    <div>
      <div style={{ fontFamily: CELL_MONO, fontVariantNumeric: 'tabular-nums', fontSize: 14, color: CC.ink2, whiteSpace: 'nowrap' }}>
        {Number.isFinite(useTime) ? `${useTime}s` : ''}
      </div>
      {sub && (
        <div style={{ fontSize: 12, color: CC.mut2, marginTop: 5, fontFamily: CELL_MONO, whiteSpace: 'nowrap' }}>
          {sub}
        </div>
      )}
    </div>
  );
}

// 输入单价:按次计费用 model_price;按 token 计费用 model_ratio×2(= $/1M tokens 基准价)
function getInputUnitPrice(other) {
  const mp = other?.model_price;
  if (mp != null && Number(mp) !== -1) {
    return { value: Number(mp), perCall: true };
  }
  const mr = other?.model_ratio;
  if (mr != null && Number.isFinite(Number(mr))) {
    return { value: Number(mr) * 2.0, perCall: false };
  }
  return null;
}

function renderCostCell(record, ctx) {
  if (!isConsumptionRow(record)) return null;
  const other = getLogOther(record.other);
  const isSub = other?.billing_source === 'subscription';
  const isErr = record.type === 5;
  const price = getInputUnitPrice(other);
  // 紧凑:去掉多余小数(整数不补零,原样显示),token→/1M,按次→/次
  const fmtNum = (v) => parseFloat(Number(v).toFixed(6)).toString();
  const priceText = price
    ? `$${fmtNum(price.value)}/${price.perCall ? ctx.t('次') : '1M'}`
    : '';
  return (
    <div>
      <div style={{ fontFamily: CELL_MONO, fontVariantNumeric: 'tabular-nums', fontWeight: 500, fontSize: 16, letterSpacing: '-0.01em', color: isErr ? '#b91c1c' : CC.ink }}>
        {isSub ? (
          <Tooltip content={`${ctx.t('由订阅抵扣')}：${renderQuota(record.quota, 6)}`}>
            <span>{renderBillingTag(record, ctx.t)}</span>
          </Tooltip>
        ) : (
          renderQuota(record.quota, 6)
        )}
      </div>
      {priceText && (
        <div style={{ marginTop: 5, fontSize: 12, color: CC.mut, fontFamily: CELL_MONO, whiteSpace: 'nowrap' }}>
          {priceText}
        </div>
      )}
    </div>
  );
}

function renderDetailButton(record, ctx) {
  return (
    <span
      onClick={(e) => { e.stopPropagation(); ctx.onOpenDetail?.(record); }}
      style={{ display: 'inline-flex', alignItems: 'center', gap: 2, color: '#3557d6', fontSize: 13, fontWeight: 500, cursor: 'pointer', whiteSpace: 'nowrap' }}
    >
      {ctx.t('详情')}<span style={{ fontSize: 13 }}>›</span>
    </span>
  );
}

// ============ 移动端卡片 ============
const M_CARD = {
  background: '#fff',
  border: '1.5px solid #e2e5dc',
  borderRadius: 14,
  padding: '13px 14px',
};
const M_DOT = () => (
  <span style={{ width: 3, height: 3, borderRadius: '50%', background: '#d9ddd4', flex: '0 0 auto' }} />
);
function renderLogMobileCard(record, ctx) {
  const { t } = ctx;
  const timeStr = String(record.timestamp2string || '');

  // 非消费行(充值/管理/系统/退款):类型标签 + 时间 + content 描述(金额在 content 里)
  if (!isConsumptionRow(record)) {
    return (
      <div style={M_CARD}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10 }}>
          <div>{renderTypeChip(record.type, t)}</div>
          <span style={{ fontFamily: CELL_MONO, fontSize: 11.5, fontWeight: 700, color: CC.mut, whiteSpace: 'nowrap' }}>{timeStr}</span>
        </div>
        {record.content && (
          <div style={{ marginTop: 9, fontSize: 13, fontWeight: 600, color: CC.ink2, lineHeight: 1.5, wordBreak: 'break-all' }}>
            {record.content}
          </div>
        )}
      </div>
    );
  }

  const other = getLogOther(record.other);
  const isSub = other?.billing_source === 'subscription';
  const isErr = record.type === 5;
  const dotColor = getTypeChipStyle(record.type, t).color;
  const price = getInputUnitPrice(other);
  const fmtNum = (v) => parseFloat(Number(v).toFixed(6)).toString();
  const priceText = price ? `$${fmtNum(price.value)}/${price.perCall ? t('次') : '1M'}` : '';
  const groupName = getGroupName(record, other);
  const ratio = getGroupRatioValue(other);

  return (
    <div style={M_CARD}>
      {/* 顶部:模型名 + 花费 */}
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 10 }}>
        <div style={{ minWidth: 0, display: 'flex', alignItems: 'center', gap: 7 }}>
          <span style={{ width: 7, height: 7, borderRadius: '50%', flex: '0 0 auto', background: dotColor }} />
          <span onClick={(e) => ctx.copyText(e, record.model_name)} style={{ fontSize: 15, fontWeight: 800, color: CC.ink, letterSpacing: '-0.01em', wordBreak: 'break-all', lineHeight: 1.25 }}>
            {record.model_name}
          </span>
        </div>
        <div style={{ textAlign: 'right', flex: '0 0 auto' }}>
          <div style={{ fontFamily: CELL_MONO, fontSize: 16, fontWeight: 800, color: isErr ? '#b91c1c' : CC.ink, letterSpacing: '-0.02em', whiteSpace: 'nowrap' }}>
            {isSub ? renderBillingTag(record, t) : renderQuota(record.quota, 6)}
          </div>
          {priceText && (
            <div style={{ fontFamily: CELL_MONO, fontSize: 10.5, fontWeight: 700, color: CC.mut2, marginTop: 1 }}>{priceText}</div>
          )}
        </div>
      </div>

      {/* 标签 */}
      <div style={{ display: 'flex', gap: 6, marginTop: 8, flexWrap: 'wrap' }}>
        {renderTypeChip(record.type, t)}
        {renderStreamTag(record, t)}
      </div>

      {/* 元信息 */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, marginTop: 9, fontFamily: CELL_MONO, fontSize: 11.5, fontWeight: 700, color: CC.mut, flexWrap: 'wrap' }}>
        {ctx.isAdminUser && record.username && (<><span style={{ color: CC.ink2, fontWeight: 800 }}>{record.username}</span><M_DOT /></>)}
        {record.token_name && (<><span style={{ color: CC.ink2, fontWeight: 800 }}>{record.token_name}</span><M_DOT /></>)}
        {groupName && (<>{groupName}{ratio ? ` ×${ratio}` : ''}<M_DOT /></>)}
        <span>{timeStr}</span>
      </div>

      {/* 统计 */}
      <div style={{ display: 'flex', gap: 20, marginTop: 11, paddingTop: 11, borderTop: '1px dashed #e4e7de' }}>
        <div>
          <div style={{ fontSize: 11, fontWeight: 700, color: CC.mut }}>{t('输入')}</div>
          <div style={{ fontFamily: CELL_MONO, fontSize: 14, fontWeight: 800, color: CC.ink, marginTop: 2, fontVariantNumeric: 'tabular-nums' }}>{formatTokenCount(record.prompt_tokens)}</div>
        </div>
        <div>
          <div style={{ fontSize: 11, fontWeight: 700, color: CC.mut }}>{t('输出')}</div>
          <div style={{ fontFamily: CELL_MONO, fontSize: 14, fontWeight: 800, color: CC.ink, marginTop: 2, fontVariantNumeric: 'tabular-nums' }}>{formatTokenCount(record.completion_tokens)}</div>
        </div>
        <div>
          <div style={{ fontSize: 11, fontWeight: 700, color: CC.mut }}>{t('用时')}</div>
          <div style={{ fontFamily: CELL_MONO, fontSize: 14, fontWeight: 800, color: CC.ink, marginTop: 2, fontVariantNumeric: 'tabular-nums' }}>{record.use_time ? `${record.use_time}s` : '-'}</div>
        </div>
        <div style={{ marginLeft: 'auto', alignSelf: 'flex-end' }}>{renderDetailButton(record, ctx)}</div>
      </div>
    </div>
  );
}

export const getLogMobileCardRender = ({
  t,
  copyText,
  showUserInfoFunc,
  openChannelAffinityUsageCacheModal,
  isAdminUser,
  billingDisplayMode = 'price',
  onOpenDetail,
}) => {
  const ctx = {
    t,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    isAdminUser,
    billingDisplayMode,
    onOpenDetail,
  };
  return (record) => renderLogMobileCard(record, ctx);
};

export const getLogsColumns = ({
  t,
  COLUMN_KEYS,
  copyText,
  showUserInfoFunc,
  openChannelAffinityUsageCacheModal,
  isAdminUser,
  billingDisplayMode = 'price',
  onOpenDetail,
}) => {
  const ctx = {
    t,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    isAdminUser,
    billingDisplayMode,
    onOpenDetail,
  };

  const columns = [
    {
      key: COLUMN_KEYS.TIME,
      title: t('时间'),
      dataIndex: 'timestamp2string',
      width: isAdminUser ? 92 : 100,
      render: (_text, record) => renderTimeCell(record),
    },
  ];

  if (isAdminUser) {
    columns.push({
      key: COLUMN_KEYS.USER,
      title: t('用户'),
      dataIndex: 'username',
      width: 140,
      render: (_text, record) => renderUserCell(record, ctx),
    });
    columns.push({
      key: COLUMN_KEYS.CHANNEL,
      title: t('渠道'),
      dataIndex: 'channel',
      width: 116,
      render: (_text, record) => renderChannelCell(record, ctx),
    });
  } else {
    columns.push({
      key: COLUMN_KEYS.TOKEN_GROUP,
      title: t('令牌 / 分组'),
      dataIndex: 'token_name',
      width: 150,
      render: (_text, record) => renderTokenGroupCell(record, ctx),
    });
  }

  columns.push({
    key: COLUMN_KEYS.REQ_MODEL,
    title: t('请求 / 模型'),
    dataIndex: 'model_name',
    width: isAdminUser ? 204 : 244,
    render: (_text, record) => renderReqModelCell(record, ctx),
  });
  columns.push({
    key: COLUMN_KEYS.USAGE,
    title: t('用量'),
    dataIndex: 'prompt_tokens',
    width: isAdminUser ? 194 : 236,
    render: (_text, record) => renderUsageCell(record, ctx),
  });
  columns.push({
    key: COLUMN_KEYS.USE_TIME,
    title: t('用时'),
    dataIndex: 'use_time',
    width: isAdminUser ? 92 : 100,
    render: (_text, record) => renderUseTimeCell(record, ctx),
  });
  columns.push({
    key: COLUMN_KEYS.COST,
    title: t('花费'),
    dataIndex: 'quota',
    width: isAdminUser ? 132 : 150,
    render: (_text, record) => renderCostCell(record, ctx),
  });
  columns.push({
    key: COLUMN_KEYS.DETAILS,
    title: t('操作'),
    dataIndex: 'operate',
    width: isAdminUser ? 64 : 72,
    align: 'center',
    render: (_text, record) => renderDetailButton(record, ctx),
  });

  return columns;
};
