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
import { Button, Form, Modal } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';

import { DATE_RANGE_PRESETS } from '../../../constants/console.constants';

// 全部搜索条件集中在一个弹窗内(时间 + 模型/令牌/分组/RequestID/类型/渠道/用户)。
// Form 常驻挂载(弹窗 keepDOM),保证二级导航上的快捷区间能通过 formApi 写入 dateRange。
const LogsFilters = ({
  formInitValues,
  setFormApi,
  refresh,
  setShowColumnSelector,
  formApi,
  setLogType,
  loading,
  isAdminUser,
  showFilterModal,
  setShowFilterModal,
  setActiveTimeRange,
  t,
}) => {
  const handleReset = () => {
    if (!formApi) return;
    formApi.reset();
    setLogType(0);
    setActiveTimeRange('today');
    setTimeout(() => refresh(), 100);
  };

  const handleSearch = async () => {
    if (formApi) await formApi.validate();
    setActiveTimeRange(null);
    setShowFilterModal(false);
    await refresh();
  };

  return (
    <Form
      initValues={formInitValues}
      getFormApi={(api) => setFormApi(api)}
      onSubmit={refresh}
      allowEmpty={true}
      autoComplete='off'
      layout='vertical'
      trigger='change'
      stopValidateWithError={false}
    >
      <Modal
        title={t('筛选')}
        visible={showFilterModal}
        onCancel={() => setShowFilterModal(false)}
        keepDOM
        lazyRender={false}
        width={560}
        footer={
          <div className='flex justify-end gap-2'>
            <Button
              type='tertiary'
              theme='light'
              size='small'
              onClick={handleReset}
            >
              {t('重置')}
            </Button>
            <Button
              type='tertiary'
              theme='light'
              size='small'
              onClick={() => setShowColumnSelector(true)}
            >
              {t('列设置')}
            </Button>
            <Button
              type='primary'
              theme='solid'
              size='small'
              icon={<IconSearch />}
              loading={loading}
              onClick={handleSearch}
            >
              {t('查询')}
            </Button>
          </div>
        }
      >
        <div className='flex flex-col gap-3'>
          <Form.DatePicker
            field='dateRange'
            label={t('时间范围')}
            className='w-full'
            type='dateTimeRange'
            placeholder={[t('开始时间'), t('结束时间')]}
            rangeSeparator=''
            showClear
            size='small'
            presets={DATE_RANGE_PRESETS.map((preset) => ({
              text: t(preset.text),
              start: preset.start(),
              end: preset.end(),
            }))}
          />

          <div className='grid grid-cols-2 gap-x-3 gap-y-1'>
            <Form.Input
              field='model_name'
              label={t('模型名称')}
              prefix={<IconSearch />}
              placeholder={t('模型名称')}
              showClear
              size='small'
            />
            <Form.Input
              field='token_name'
              label={t('令牌名称')}
              prefix={<IconSearch />}
              placeholder={t('令牌名称')}
              showClear
              size='small'
            />
            <Form.Input
              field='group'
              label={t('分组')}
              prefix={<IconSearch />}
              placeholder={t('分组')}
              showClear
              size='small'
            />
            <Form.Input
              field='request_id'
              label={t('Request ID')}
              prefix={<IconSearch />}
              placeholder={t('Request ID')}
              showClear
              size='small'
            />
            <Form.Select
              field='logType'
              label={t('日志类型')}
              placeholder={t('日志类型')}
              className='w-full'
              showClear
              size='small'
            >
              <Form.Select.Option value='0'>{t('全部')}</Form.Select.Option>
              <Form.Select.Option value='1'>{t('充值')}</Form.Select.Option>
              <Form.Select.Option value='2'>{t('消费')}</Form.Select.Option>
              <Form.Select.Option value='3'>{t('管理')}</Form.Select.Option>
              <Form.Select.Option value='4'>{t('系统')}</Form.Select.Option>
              <Form.Select.Option value='5'>{t('错误')}</Form.Select.Option>
              <Form.Select.Option value='6'>{t('退款')}</Form.Select.Option>
            </Form.Select>
            {isAdminUser && (
              <>
                <Form.Input
                  field='channel'
                  label={t('渠道 ID')}
                  prefix={<IconSearch />}
                  placeholder={t('渠道 ID')}
                  showClear
                  size='small'
                />
                <Form.Input
                  field='username'
                  label={t('用户名称')}
                  prefix={<IconSearch />}
                  placeholder={t('用户名称')}
                  showClear
                  size='small'
                />
              </>
            )}
          </div>
        </div>
      </Modal>
    </Form>
  );
};

export default LogsFilters;
