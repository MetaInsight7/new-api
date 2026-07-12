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

const DetailItem = ({ label, value, mono = false, onCopy, strong = false }) => {
  if (value === undefined || value === null || value === '') return null;
  return (
    <div className='usage-log-detail__item'>
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
  const billingItems = useMemo(
    () => detailData.filter((item) => item?.key === t('计费过程')),
    [detailData, t],
  );
  const extraItems = useMemo(
    () =>
      detailData.filter(
        (item) =>
          item?.key !== t('计费过程') &&
          ![
            t('Request ID'),
            t('渠道信息'),
            t('请求路径'),
            t('缓存 Tokens'),
            t('缓存创建 Tokens'),
          ].includes(item?.key),
      ),
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

  return (
    <div
      className={`usage-log-detail${isAdminUser ? ' usage-log-detail--admin' : ' usage-log-detail--user'}`}
    >
      <div className='usage-log-detail__panels'>
        <section className='usage-log-detail__section'>
          <h4>{isAdminUser ? t('用户与请求') : t('请求信息')}</h4>
          <div className='usage-log-detail__grid'>
            {isAdminUser && (
              <DetailItem
                label={t('用户')}
                value={
                  record.username
                    ? `${record.username}${record.user_id ? ` · ID ${record.user_id}` : ''}`
                    : record.user_id
                }
              />
            )}
            {!isAdminUser && (
              <DetailItem label={t('令牌名称')} value={record.token_name} />
            )}
            <DetailItem
              label={t('Request ID')}
              value={record.request_id}
              mono
              onCopy={(event) => copyText(event, record.request_id)}
            />
            {isAdminUser && (
              <DetailItem label={t('客户端 IP')} value={record.ip} mono />
            )}
            <DetailItem label={t('请求路径')} value={other.request_path} mono />
            {!isAdminUser && (
              <DetailItem label={t('模型')} value={record.model_name} />
            )}
          </div>
        </section>

        {isAdminUser && (
          <section className='usage-log-detail__section'>
            <h4>{t('渠道路由')}</h4>
            <div className='usage-log-detail__grid'>
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
            </div>
          </section>
        )}

        <section className='usage-log-detail__section'>
          <h4>{t('Token 与计费')}</h4>
          <div className='usage-log-detail__grid'>
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
          </div>
        </section>
      </div>

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
