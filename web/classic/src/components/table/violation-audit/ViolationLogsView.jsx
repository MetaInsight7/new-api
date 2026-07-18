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
  Button,
  DatePicker,
  Input,
  Select,
  Modal,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import CardPro from '../../common/ui/CardPro';
import CardTable from '../../common/ui/CardTable';
import TableEmpty from '../../common/ui/TableEmpty';
import { createCardProPagination } from '../../../helpers/utils';
import { DATE_RANGE_PRESETS } from '../../../constants/console.constants';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import {
  VIOLATION_CATEGORIES,
  VIOLATION_SEVERITIES,
  VIOLATION_ACTIONS,
  categoryLabel,
  categoryColor,
  severityTag,
  actionTag,
  fmtTime,
  fmtTimeSplit,
} from './violationHelpers';

const tag = (label, color) => (
  <Tag color={color} size='small' shape='circle' type='light'>
    {label}
  </Tag>
);

const ViolationLogsView = ({ data }) => {
  const { t } = data;
  const isMobile = useIsMobile();
  const [detail, setDetail] = useState(null);

  const setF = (patch) => data.setFilters((prev) => ({ ...prev, ...patch }));

  const columns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        width: 100,
        render: (v) => {
          const s = fmtTimeSplit(v);
          return (
            <div>
              <div>{s.time}</div>
              {s.date && <div className='va-time-date'>{s.date}</div>}
            </div>
          );
        },
      },
      {
        title: t('用户'),
        dataIndex: 'username',
        width: 120,
        render: (v, r) => (
          <div>
            <div style={{ fontWeight: 600 }}>{v || '-'}</div>
            {r.user_group ? (
              <div className='va-usermeta'>{r.user_group}</div>
            ) : null}
          </div>
        ),
      },
      {
        title: 'IP',
        dataIndex: 'ip',
        width: 120,
        render: (v) => v || '-',
      },
      {
        title: t('模型'),
        dataIndex: 'model_name',
        width: 170,
        render: (v) => (
          <Typography.Text ellipsis={{ showTooltip: true }} style={{ maxWidth: 155 }}>
            {v || '-'}
          </Typography.Text>
        ),
      },
      {
        title: t('分类'),
        dataIndex: 'category',
        width: 80,
        render: (v) => (
          <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
            {(v || '')
              .split(',')
              .filter(Boolean)
              .map((c) => (
                <span key={c}>{tag(categoryLabel(t, c), categoryColor(c))}</span>
              ))}
          </div>
        ),
      },
      {
        title: t('分级'),
        dataIndex: 'severity',
        width: 70,
        render: (v) => {
          const m = severityTag(t, v);
          return tag(m.label, m.color);
        },
      },
      {
        title: t('处理'),
        dataIndex: 'action',
        width: 80,
        render: (v) => {
          const m = actionTag(t, v);
          return tag(m.label, m.color);
        },
      },
      {
        title: t('命中词'),
        dataIndex: 'matched_words',
        render: (v) => (
          <Typography.Text
            type='danger'
            ellipsis={{ showTooltip: true }}
          >
            {v}
          </Typography.Text>
        ),
      },
      {
        title: '',
        dataIndex: 'operate',
        width: 60,
        render: (_, r) => (
          <Button size='small' theme='borderless' type='primary' onClick={() => setDetail(r)}>
            {t('详情')}
          </Button>
        ),
      },
    ],
    [t],
  );

  const mobileCardRender = (r) => {
    const sev = severityTag(t, r.severity);
    const act = actionTag(t, r.action);
    return (
      <div className='va-mcard' onClick={() => setDetail(r)}>
        <div className='va-mcard__head'>
          <span style={{ fontWeight: 700 }}>{r.username || '-'}</span>
          {tag(act.label, act.color)}
          {tag(sev.label, sev.color)}
        </div>
        <div className='va-mcard__row'>
          <Typography.Text type='tertiary' size='small'>
            {r.model_name || '-'}
          </Typography.Text>
          <span className='va-mono va-mcard__time'>{fmtTime(r.created_at)}</span>
        </div>
        <Typography.Text type='danger' size='small' style={{ marginTop: 6, display: 'block' }}>
          {r.matched_words}
        </Typography.Text>
      </div>
    );
  };

  const header = (
    <div className='va-toolbar'>
      <div className='va-toolbar__title'>
        {t('审计记录')}
        <span className='va-toolbar__count'>
          {data.total} {t('条')}
        </span>
      </div>
    </div>
  );

  const handleReset = () => {
    data.setActiveTimeRange('7d');
    setF({ category: '', severity: '', action: '', username: '', model_name: '', request_id: '' });
    data.setShowFilterModal(false);
    setTimeout(() => data.loadLogs(1, data.pageSize), 0);
  };
  const handleSearch = () => {
    data.setActiveTimeRange(null);
    data.setShowFilterModal(false);
    setTimeout(() => data.loadLogs(1, data.pageSize), 0);
  };

  const filterModal = (
    <Modal
      title={t('筛选')}
      visible={data.showFilterModal}
      onCancel={() => data.setShowFilterModal(false)}
      width={isMobile ? '92%' : 560}
      footer={
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <Button type='tertiary' theme='light' size='small' onClick={handleReset}>
            {t('重置')}
          </Button>
          <Button
            type='primary'
            theme='solid'
            size='small'
            icon={<IconSearch />}
            onClick={handleSearch}
          >
            {t('查询')}
          </Button>
        </div>
      }
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <DatePicker
          type='dateTimeRange'
          className='w-full'
          size='small'
          value={data.filters.dateRange}
          onChange={(v) => {
            data.setActiveTimeRange(null);
            setF({ dateRange: v });
          }}
          placeholder={[t('开始时间'), t('结束时间')]}
          rangeSeparator=''
          presets={DATE_RANGE_PRESETS.map((p) => ({
            text: t(p.text),
            start: p.start(),
            end: p.end(),
          }))}
        />
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px 12px' }}>
          <Select
            size='small'
            placeholder={t('分类')}
            value={data.filters.category || undefined}
            onChange={(v) => setF({ category: v || '' })}
            showClear
          >
            {VIOLATION_CATEGORIES.map((c) => (
              <Select.Option key={c.value} value={c.value}>
                {t(c.label)}
              </Select.Option>
            ))}
          </Select>
          <Select
            size='small'
            placeholder={t('分级')}
            value={data.filters.severity || undefined}
            onChange={(v) => setF({ severity: v || '' })}
            showClear
          >
            {VIOLATION_SEVERITIES.map((c) => (
              <Select.Option key={c.value} value={c.value}>
                {t(c.label)}
              </Select.Option>
            ))}
          </Select>
          <Select
            size='small'
            placeholder={t('处理')}
            value={data.filters.action || undefined}
            onChange={(v) => setF({ action: v || '' })}
            showClear
          >
            {VIOLATION_ACTIONS.map((c) => (
              <Select.Option key={c.value} value={c.value}>
                {t(c.label)}
              </Select.Option>
            ))}
          </Select>
          <Input
            size='small'
            placeholder={t('用户名称')}
            prefix={<IconSearch />}
            value={data.filters.username}
            onChange={(v) => setF({ username: v })}
            showClear
          />
          <Input
            size='small'
            placeholder={t('模型名称')}
            prefix={<IconSearch />}
            value={data.filters.model_name}
            onChange={(v) => setF({ model_name: v })}
            showClear
          />
          <Input
            size='small'
            placeholder={t('Request ID')}
            prefix={<IconSearch />}
            value={data.filters.request_id}
            onChange={(v) => setF({ request_id: v })}
            showClear
          />
        </div>
      </div>
    </Modal>
  );

  return (
    <>
      <CardPro
        type='type1'
        className='dmit-flat-card'
        disableMobileCollapse
        actionsArea={header}
        paginationArea={createCardProPagination({
          currentPage: data.page,
          pageSize: data.pageSize,
          total: data.total,
          onPageChange: (p) => data.loadLogs(p, data.pageSize),
          onPageSizeChange: (s) => data.loadLogs(1, s),
          isMobile,
          t,
        })}
        t={t}
      >
        <CardTable
          className='dmit-flat-table'
          columns={columns}
          dataSource={data.logs}
          loading={data.loading}
          rowKey='id'
          size='middle'
          mobileCardRender={mobileCardRender}
          hidePagination
          empty={<TableEmpty title={t('暂无违规记录')} />}
        />
      </CardPro>

      <Modal
        title={t('违规详情')}
        visible={!!detail}
        onCancel={() => setDetail(null)}
        footer={null}
        width={isMobile ? '92%' : 640}
      >
        {detail && <ViolationDetail t={t} detail={detail} />}
      </Modal>

      {filterModal}
    </>
  );
};

// 详情:徽章行 + 双列信息 + 命中内容面板
const ViolationDetail = ({ t, detail }) => {
  const catTags = (detail.category || '').split(',').filter(Boolean);
  const sev = severityTag(t, detail.severity);
  const act = actionTag(t, detail.action);
  const kv = [
    [t('时间'), fmtTime(detail.created_at)],
    [t('用户'), `${detail.username || '-'} (#${detail.user_id})`],
    [t('分组'), detail.user_group || '-'],
    [t('API 密钥'), detail.token_name || '-'],
    [t('模型'), detail.model_name || '-'],
    ['IP', detail.ip || '-'],
    ['Request ID', detail.request_id || '-'],
    [
      t('命中词'),
      <Typography.Text type='danger'>{detail.matched_words || '-'}</Typography.Text>,
    ],
  ];
  return (
    <div className='va-detail'>
      <div className='va-detail__tags'>
        {catTags.map((c) => (
          <span key={c}>{tag(categoryLabel(t, c), categoryColor(c))}</span>
        ))}
        {tag(sev.label, sev.color)}
        {tag(act.label, act.color)}
      </div>
      <div className='va-detail__grid'>
        {kv.map(([k, v]) => (
          <div className='va-kv' key={k}>
            <div className='va-kv__k'>{k}</div>
            <div className='va-kv__v'>{v}</div>
          </div>
        ))}
      </div>
      <ViolationContent t={t} detail={detail} />
    </div>
  );
};

// 命中词高亮
const highlight = (text, matched) => {
  if (!text) return text;
  const words = (matched || '')
    .split(',')
    .map((w) => w.trim())
    .filter(Boolean);
  if (!words.length) return text;
  const esc = words.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
  const re = new RegExp(`(${esc.join('|')})`, 'gi');
  return String(text)
    .split(re)
    .map((p, i) =>
      words.some((w) => w.toLowerCase() === p.toLowerCase()) ? (
        <mark key={i} className='va-hl'>
          {p}
        </mark>
      ) : (
        <React.Fragment key={i}>{p}</React.Fragment>
      ),
    );
};

// 命中内容:优先展示结构化上下文的原始 JSON(美化缩进),无则退回命中片段;固定大小、可滚动
const ViolationContent = ({ t, detail }) => {
  let body = detail.snippet || '-';
  let isJson = false;
  if (detail.context) {
    try {
      body = JSON.stringify(JSON.parse(detail.context), null, 2);
      isJson = true;
    } catch (e) {
      body = detail.context;
    }
  }
  return (
    <div className='va-detail__block'>
      <div className='va-detail__block-title'>
        {t('命中内容')}
        <Typography.Text type='tertiary' size='small'>
          {isJson ? t('当前及前若干轮对话 JSON') : t('命中片段')}
        </Typography.Text>
      </div>
      <pre className='va-ctx__box'>{highlight(body, detail.matched_words)}</pre>
    </div>
  );
};

export default ViolationLogsView;
