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
import { Button, Collapsible, Typography } from '@douyinfe/semi-ui';
import { IconChevronDown, IconChevronUp, IconCopy } from '@douyinfe/semi-icons';
import { getLogOther, renderQuota } from '../../../helpers';

const DetailItem = ({
  label,
  value,
  mono = false,
  onCopy,
  strong = false,
  fullWidth = false,
}) => {
  if (value === undefined || value === null || value === '') return null;
  return (
    <div
      className={`usage-log-detail__item${fullWidth ? ' usage-log-detail__item--full' : ''}`}
    >
      <div className='usage-log-detail__label'>{label}</div>
      <div
        className={`${mono ? 'usage-log-detail__value usage-log-detail__value--mono' : 'usage-log-detail__value'}${strong ? ' usage-log-detail__value--strong' : ''}`}
      >
        <span>{value}</span>
        {onCopy && (
          <button
            type='button'
            className='usage-log-detail__copy'
            onClick={onCopy}
            aria-label={`${label} copy`}
          >
            <IconCopy size='small' />
          </button>
        )}
      </div>
    </div>
  );
};

const DetailSection = ({ title, tone, children }) => (
  <section
    className={`usage-log-detail__section${tone ? ` usage-log-detail__section--${tone}` : ''}`}
  >
    <h4>{title}</h4>
    <div className='usage-log-detail__grid'>{children}</div>
  </section>
);

const UsageLogInlineDetail = ({
  record,
  detailData = [],
  isAdminUser,
  copyText,
  t,
}) => {
  const [billingOpen, setBillingOpen] = useState(false);
  const [extraOpen, setExtraOpen] = useState(false);
  const other = getLogOther(record.other) || {};
  const adminInfo = other.admin_info || {};
  const type = Number(record.type);
  const isConsumption = type === 0 || type === 2;
  const isError = type === 5;
  const isRefund = type === 6;
  const isTopup = type === 1;
  const isOperation = type === 3 || type === 4;
  const detailValue = (key) =>
    detailData.find((item) => item?.key === t(key))?.value;
  const billingItems = useMemo(
    () =>
      isConsumption
        ? detailData.filter((item) => item?.key === t('计费过程'))
        : [],
    [detailData, isConsumption, t],
  );
  const extraItems = useMemo(
    () => {
      const structuredKeys = [
        'Request ID',
        '渠道信息',
        '请求路径',
        '缓存 Tokens',
        '缓存创建 Tokens',
        '任务ID',
        '失败原因',
        '订单支付方式',
        '回调支付方式',
        '回调调用者IP',
        '服务器IP',
        '节点名称',
        '系统版本',
        '审计信息',
        '操作管理员',
      ].map((key) => t(key));
      return (
      detailData.filter(
        (item) =>
          item?.key !== t('计费过程') &&
            !structuredKeys.includes(item?.key),
        )
      );
    },
    [detailData, t],
  );
  const cacheRead = Number(other.cache_tokens) || 0;
  const cacheCreate =
    (Number(other.cache_creation_tokens_5m) || 0) +
      (Number(other.cache_creation_tokens_1h) || 0) ||
    Number(other.cache_creation_tokens) ||
    0;
  const ratio =
    other.user_group_ratio !== undefined &&
    Number(other.user_group_ratio) !== -1
      ? other.user_group_ratio
      : other.group_ratio;

  const identityItem = isAdminUser ? (
    <DetailItem
      label={t('用户')}
      value={
        record.username
          ? `${record.username}${record.user_id ? ` · ID ${record.user_id}` : ''}`
          : record.user_id
      }
    />
  ) : (
    <DetailItem label={t('令牌名称')} value={record.token_name} />
  );

  const requestIdItem = (
    <DetailItem
      label={t('Request ID')}
      value={record.request_id}
      mono
      onCopy={(event) => copyText(event, record.request_id)}
    />
  );

  const routeSection = isAdminUser ? (
    <DetailSection title={t(isError ? '路由诊断' : '渠道路由')}>
      <DetailItem
        label={t('最终渠道')}
        value={
          record.channel
            ? `#${record.channel} ${record.channel_name || ''}`.trim()
            : record.channel_name
        }
      />
      <DetailItem
        label={t('路由状态')}
        value={
          Array.isArray(adminInfo.use_channel) &&
          adminInfo.use_channel.length > 1
            ? adminInfo.use_channel.map((id) => `#${id}`).join(' → ')
            : t('直连')
        }
        mono
      />
      <DetailItem
        label={t('上游 Request ID')}
        value={record.upstream_request_id}
        mono
      />
      {!isRefund && (
        <DetailItem
          label={t('流状态')}
          value={
            other.stream_status?.status
              ? `${other.stream_status.status === 'ok' ? t('正常') : t('异常')} · ${other.stream_status.end_reason || '-'}`
              : record.is_stream
                ? t('流式')
                : t('非流式')
          }
        />
      )}
    </DetailSection>
  ) : null;

  let panels;
  let panelClass = '';

  if (isConsumption) {
    panels = (
      <>
        <DetailSection title={isAdminUser ? t('用户与请求') : t('请求信息')}>
          {identityItem}
          {requestIdItem}
          <DetailItem label={t('客户端 IP')} value={record.ip} mono />
          <DetailItem label={t('请求路径')} value={other.request_path} mono />
          {!isAdminUser && (
            <DetailItem label={t('模型')} value={record.model_name} />
          )}
        </DetailSection>
        {routeSection}
        <DetailSection title={t('Token 与计费')}>
          <DetailItem
            label={t('输入 / 输出')}
            value={`${Number(record.prompt_tokens || 0).toLocaleString()} / ${Number(record.completion_tokens || 0).toLocaleString()}`}
            mono
          />
          <DetailItem
            label={t('缓存')}
            value={`${t('读')} ${cacheRead.toLocaleString()} · ${t('写')} ${cacheCreate.toLocaleString()}`}
            mono
          />
          <DetailItem
            label={t('计费分组')}
            value={`${record.group || other.group || '-'}${ratio !== undefined && ratio !== null ? ` · ${ratio}x` : ''}`}
          />
          <DetailItem
            label={t('实际扣费')}
            value={renderQuota(record.quota, 6)}
            mono
            strong
          />
        </DetailSection>
      </>
    );
  } else if (isError) {
    panels = (
      <>
        <DetailSection title={t('错误概览')} tone='error'>
          <DetailItem
            label={t('错误状态')}
            value={
              other.stream_status?.end_reason ||
              other.stream_status?.status ||
              t('请求失败')
            }
          />
          <DetailItem
            label={t('失败耗时')}
            value={record.use_time ? `${record.use_time}s` : null}
            mono
          />
          <DetailItem
            label={t('失败原因')}
            value={
              record.content ||
              other.stream_status?.end_error ||
              detailValue('失败原因')
            }
            fullWidth
          />
        </DetailSection>
        <DetailSection title={t('请求信息')}>
          {identityItem}
          <DetailItem label={t('模型')} value={record.model_name} />
          {requestIdItem}
          <DetailItem label={t('请求路径')} value={other.request_path} mono />
          <DetailItem label={t('客户端 IP')} value={record.ip} mono />
        </DetailSection>
        {routeSection}
      </>
    );
  } else if (isRefund) {
    panels = (
      <>
        <DetailSection title={t('退款信息')} tone='refund'>
          <DetailItem
            label={t('退款金额')}
            value={renderQuota(record.quota, 6)}
            mono
            strong
          />
          {identityItem}
          <DetailItem label={t('退款说明')} value={record.content} />
          <DetailItem label={t('退款分组')} value={record.group || other.group} />
        </DetailSection>
        <DetailSection title={t('关联任务')}>
          <DetailItem
            label={t('任务ID')}
            value={other.task_id || detailValue('任务ID')}
            mono
          />
          <DetailItem label={t('关联模型')} value={record.model_name} />
          {requestIdItem}
          <DetailItem
            label={t('失败原因')}
            value={other.reason || detailValue('失败原因')}
          />
        </DetailSection>
        {routeSection}
      </>
    );
  } else if (isTopup) {
    panelClass = ' usage-log-detail__panels--two';
    panels = (
      <>
        <DetailSection title={t('充值信息')} tone='topup'>
          <DetailItem
            label={t('到账金额')}
            value={`+${renderQuota(record.quota, 6)}`}
            mono
            strong
          />
          {identityItem}
          <DetailItem label={t('充值说明')} value={record.content} />
          {requestIdItem}
        </DetailSection>
        {isAdminUser && (
          <DetailSection title={t('支付审计')}>
            <DetailItem
              label={t('订单支付方式')}
              value={
                adminInfo.payment_method || detailValue('订单支付方式')
              }
            />
            <DetailItem
              label={t('回调支付方式')}
              value={detailValue('回调支付方式')}
            />
            <DetailItem
              label={t('回调调用者IP')}
              value={detailValue('回调调用者IP')}
              mono
            />
            <DetailItem
              label={t('服务器IP')}
              value={detailValue('服务器IP')}
              mono
            />
            <DetailItem
              label={t('节点名称')}
              value={adminInfo.node_name || detailValue('节点名称')}
            />
            <DetailItem label={t('系统版本')} value={detailValue('系统版本')} />
            <DetailItem label={t('审计信息')} value={detailValue('审计信息')} />
          </DetailSection>
        )}
      </>
    );
  } else if (isOperation) {
    panelClass = ' usage-log-detail__panels--two';
    panels = (
      <>
        <DetailSection title={t(type === 3 ? '管理操作' : '系统事件')}>
          {identityItem}
          <DetailItem label={t('操作内容')} value={record.content} />
          <DetailItem
            label={t('操作管理员')}
            value={detailValue('操作管理员')}
          />
        </DetailSection>
        <DetailSection title={t('技术信息')}>
          {requestIdItem}
          <DetailItem label={t('客户端 IP')} value={record.ip} mono />
          <DetailItem label={t('请求路径')} value={other.request_path} mono />
        </DetailSection>
      </>
    );
  } else {
    panelClass = ' usage-log-detail__panels--two';
    panels = (
      <DetailSection title={t('日志详情')}>
        {identityItem}
        <DetailItem label={t('内容')} value={record.content} />
        {requestIdItem}
      </DetailSection>
    );
  }

  return (
    <div
      className={`usage-log-detail${isAdminUser ? ' usage-log-detail--admin' : ' usage-log-detail--user'}`}
    >
      <div className={`usage-log-detail__panels${panelClass}`}>{panels}</div>

      {extraItems.length > 0 && (
        <div className='usage-log-detail__billing'>
          <Button
            theme='borderless'
            size='small'
            icon={extraOpen ? <IconChevronUp /> : <IconChevronDown />}
            onClick={() => setExtraOpen((open) => !open)}
          >
            {t('更多技术信息')}
          </Button>
          <Collapsible isOpen={extraOpen} keepDOM>
            <section className='usage-log-detail__section usage-log-detail__section--extra'>
              <div className='usage-log-detail__grid'>
                {extraItems.map((item, index) => (
                  <DetailItem
                    key={`${item.key}-${index}`}
                    label={item.key}
                    value={item.value}
                  />
                ))}
              </div>
            </section>
          </Collapsible>
        </div>
      )}

      {billingItems.length > 0 && (
        <div className='usage-log-detail__billing'>
          <Button
            theme='borderless'
            size='small'
            icon={billingOpen ? <IconChevronUp /> : <IconChevronDown />}
            onClick={() => setBillingOpen((open) => !open)}
          >
            {t('计费依据')}
          </Button>
          <Collapsible isOpen={billingOpen} keepDOM>
            <div className='usage-log-detail__billing-content'>
              {billingItems.map((item, index) => (
                <Typography.Paragraph key={index}>
                  {item.value}
                </Typography.Paragraph>
              ))}
            </div>
          </Collapsible>
        </div>
      )}
    </div>
  );
};

export default UsageLogInlineDetail;
