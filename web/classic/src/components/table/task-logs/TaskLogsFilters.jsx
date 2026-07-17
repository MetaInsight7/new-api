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

// 全部搜索条件集中在一个弹窗内(时间 + 任务 ID + 渠道 ID)。
// Form 常驻挂载(弹窗 keepDOM),保证二级导航上的快捷区间能通过 formApi 写入 dateRange。
const TaskLogsFilters = ({
  formInitValues,
  setFormApi,
  refresh,
  setShowColumnSelector,
  formApi,
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
    setActiveTimeRange('today');
    setTimeout(() => refresh(), 100);
  };

  const handleSearch = () => {
    setActiveTimeRange(null);
    setShowFilterModal(false);
    setTimeout(() => refresh(), 0);
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
        width={480}
        footer={
          <div className='flex justify-end gap-2'>
            <Button type='tertiary' theme='light' size='small' onClick={handleReset}>
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
              field='task_id'
              label={t('任务 ID')}
              prefix={<IconSearch />}
              placeholder={t('任务 ID')}
              showClear
              size='small'
            />
            {isAdminUser && (
              <Form.Input
                field='channel_id'
                label={t('渠道 ID')}
                prefix={<IconSearch />}
                placeholder={t('渠道 ID')}
                showClear
                size='small'
              />
            )}
          </div>
        </div>
      </Modal>
    </Form>
  );
};

export default TaskLogsFilters;
