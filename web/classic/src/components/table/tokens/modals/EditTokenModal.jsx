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

import React, { useEffect, useState, useContext, useRef } from 'react';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
  renderGroupOption,
  getCurrencyConfig,
  getModelCategories,
  selectFilter,
} from '../../../../helpers';
import {
  quotaToDisplayAmount,
  displayAmountToQuota,
} from '../../../../helpers/quota';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import {
  Button,
  Modal,
  Space,
  Spin,
  Typography,
  Tag,
  Form,
  Col,
  Row,
  InputNumber,
} from '@douyinfe/semi-ui';
import {
  IconSave,
  IconClose,
  IconKey,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../../../context/Status';
import { renderSafeDatePickerTrigger } from '../../../common/ui/SafeDatePickerTrigger';
import { useRequestLifecycle } from '../../../../hooks/common/useRequestLifecycle';

const { Title } = Typography;

const EditTokenModal = (props) => {
  const { t } = useTranslation();
  const [statusState, statusDispatch] = useContext(StatusContext);
  const [loading, setLoading] = useState(false);
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [models, setModels] = useState([]);
  const [groups, setGroups] = useState([]);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const { beginRequest, isCurrentRequest } = useRequestLifecycle();
  const isEdit = props.editingToken.id !== undefined;

  const getInitValues = () => ({
    name: '',
    remain_quota: 0,
    remain_amount: 0,
    expired_time: -1,
    unlimited_quota: true,
    model_limits_enabled: false,
    model_limits: [],
    allow_ips: '',
    group: '',
    cross_group_retry: false,
    tokenCount: 1,
  });

  const handleCancel = () => {
    props.handleClose();
  };

  const setExpiredTime = (month, day, hour, minute) => {
    let now = new Date();
    let timestamp = now.getTime() / 1000;
    let seconds = month * 30 * 24 * 60 * 60;
    seconds += day * 24 * 60 * 60;
    seconds += hour * 60 * 60;
    seconds += minute * 60;
    if (!formApiRef.current) return;
    if (seconds !== 0) {
      timestamp += seconds;
      formApiRef.current.setValue('expired_time', timestamp2string(timestamp));
    } else {
      formApiRef.current.setValue('expired_time', -1);
    }
  };

  const loadModels = async () => {
    const requestId = beginRequest('models');
    let res;
    try {
      res = await API.get(`/api/user/models`);
    } catch (error) {
      if (isCurrentRequest('models', requestId)) showError(t('加载模型列表失败'));
      return;
    }
    const { success, message, data } = res.data;
    if (isCurrentRequest('models', requestId) && success) {
      const categories = getModelCategories(t);
      let localModelOptions = (data || []).map((model) => {
        let icon = null;
        for (const [key, category] of Object.entries(categories)) {
          if (key !== 'all' && category.filter({ model_name: model })) {
            icon = category.icon;
            break;
          }
        }
        return {
          label: (
            <span className='flex items-center gap-1'>
              {icon}
              {model}
            </span>
          ),
          value: model,
        };
      });
      setModels(localModelOptions);
    } else if (isCurrentRequest('models', requestId)) {
      showError(t(message));
    }
  };

  const loadGroups = async () => {
    const requestId = beginRequest('groups');
    let res;
    try {
      res = await API.get(`/api/user/self/groups`);
    } catch (error) {
      if (isCurrentRequest('groups', requestId)) showError(t('加载分组失败'));
      return;
    }
    const { success, message, data } = res.data;
    if (isCurrentRequest('groups', requestId) && success) {
      let localGroupOptions = Object.entries(data).map(([group, info]) => ({
        label: info.desc,
        value: group,
        ratio: info.ratio,
      }));
      if (statusState?.status?.default_use_auto_group) {
        if (localGroupOptions.some((group) => group.value === 'auto')) {
          localGroupOptions.sort((a, b) => (a.value === 'auto' ? -1 : 1));
        }
      }
      setGroups(localGroupOptions);
      // if (statusState?.status?.default_use_auto_group && formApiRef.current) {
      //   formApiRef.current.setValue('group', 'auto');
      // }
    } else if (isCurrentRequest('groups', requestId)) {
      showError(t(message));
    }
  };

  const loadToken = async () => {
    const requestId = beginRequest('token');
    setLoading(true);
    try {
      let res = await API.get(`/api/token/${props.editingToken.id}`);
      const { success, message, data } = res.data;
      if (isCurrentRequest('token', requestId) && success) {
        if (data.expired_time !== -1) {
          data.expired_time = timestamp2string(data.expired_time);
        }
        if (data.model_limits !== '') {
          data.model_limits = data.model_limits.split(',');
        } else {
          data.model_limits = [];
        }
        data.remain_amount = Number(
          quotaToDisplayAmount(data.remain_quota || 0).toFixed(6),
        );
        if (formApiRef.current) {
          formApiRef.current.setValues({ ...getInitValues(), ...data });
        }
      } else if (isCurrentRequest('token', requestId)) {
        showError(message);
      }
    } catch (error) {
      if (isCurrentRequest('token', requestId)) showError(t('加载令牌信息失败'));
    } finally {
      if (isCurrentRequest('token', requestId)) setLoading(false);
    }
  };

  useEffect(() => {
    if (formApiRef.current) {
      if (!isEdit) {
        formApiRef.current.setValues(getInitValues());
      }
    }
    loadModels();
    loadGroups();
  }, [props.editingToken.id]);

  useEffect(() => {
    if (props.visiable) {
      setAdvancedOpen(false);
      if (isEdit) {
        loadToken();
      } else {
        formApiRef.current?.setValues(getInitValues());
      }
    } else {
      beginRequest('models');
      beginRequest('groups');
      beginRequest('token');
      beginRequest('submit');
      setLoading(false);
      formApiRef.current?.reset();
    }
  }, [props.visiable, props.editingToken.id]);

  const generateRandomSuffix = () => {
    const characters =
      'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
    let result = '';
    for (let i = 0; i < 6; i++) {
      result += characters.charAt(
        Math.floor(Math.random() * characters.length),
      );
    }
    return result;
  };

  const submit = async (values) => {
    const requestId = beginRequest('submit');
    setLoading(true);
    try {
      if (isEdit) {
      let { tokenCount: _tc, ...localInputs } = values;
      localInputs.remain_quota = localInputs.unlimited_quota
        ? 0
        : displayAmountToQuota(localInputs.remain_amount);
      if (!localInputs.unlimited_quota && localInputs.remain_quota <= 0) {
        showError(t('请输入金额'));
        return;
      }
      if (localInputs.expired_time !== -1) {
        let time = Date.parse(localInputs.expired_time);
        if (isNaN(time)) {
          showError(t('过期时间格式错误！'));
          return;
        }
        localInputs.expired_time = Math.ceil(time / 1000);
      }
      localInputs.model_limits = localInputs.model_limits.join(',');
      localInputs.model_limits_enabled = localInputs.model_limits.length > 0;
      let res = await API.put(`/api/token/`, {
        ...localInputs,
        id: parseInt(props.editingToken.id),
      });
      const { success, message } = res.data;
      if (isCurrentRequest('submit', requestId) && success) {
        showSuccess(t('令牌更新成功！'));
        props.refresh();
        props.handleClose();
      } else if (isCurrentRequest('submit', requestId)) {
        showError(t(message));
      }
    } else {
      const count = parseInt(values.tokenCount, 10) || 1;
      let successCount = 0;
      for (let i = 0; i < count; i++) {
        let { tokenCount: _tc, ...localInputs } = values;
        const baseName =
          values.name.trim() === '' ? 'default' : values.name.trim();
        if (i !== 0 || values.name.trim() === '') {
          localInputs.name = `${baseName}-${generateRandomSuffix()}`;
        } else {
          localInputs.name = baseName;
        }
        localInputs.remain_quota = localInputs.unlimited_quota
          ? 0
          : displayAmountToQuota(localInputs.remain_amount);
        if (!localInputs.unlimited_quota && localInputs.remain_quota <= 0) {
          showError(t('请输入金额'));
          break;
        }

        if (localInputs.expired_time !== -1) {
          let time = Date.parse(localInputs.expired_time);
          if (isNaN(time)) {
            showError(t('过期时间格式错误！'));
            break;
          }
          localInputs.expired_time = Math.ceil(time / 1000);
        }
        localInputs.model_limits = localInputs.model_limits.join(',');
        localInputs.model_limits_enabled = localInputs.model_limits.length > 0;
        let res = await API.post(`/api/token/`, localInputs);
        const { success, message } = res.data;
        if (isCurrentRequest('submit', requestId) && success) {
          successCount++;
        } else if (isCurrentRequest('submit', requestId)) {
          showError(t(message));
          break;
        }
      }
      if (isCurrentRequest('submit', requestId) && successCount > 0) {
        showSuccess(t('令牌创建成功，请在列表页面点击复制获取令牌！'));
        props.refresh();
        props.handleClose();
      }
      }
    } catch (error) {
      if (isCurrentRequest('submit', requestId)) {
        showError(error.response?.data?.message || t('操作失败'));
      }
    } finally {
      if (isCurrentRequest('submit', requestId)) {
        setLoading(false);
        formApiRef.current?.setValues(getInitValues());
      }
    }
  };

  return (
    <Modal
      className='token-edit-modal'
      title={
        <div className='flex items-center gap-3'>
          <span
            className='inline-flex items-center justify-center w-9 h-9 rounded-lg flex-shrink-0'
            style={{ background: 'rgba(37, 99, 235,0.10)', color: '#2563eb' }}
          >
            <IconKey size={18} />
          </span>
          <div className='min-w-0'>
            <div className='flex items-center gap-2'>
              <Title heading={5} className='m-0'>
                {isEdit ? t('更新令牌信息') : t('创建新的令牌')}
              </Title>
              <Tag color={isEdit ? 'grey' : 'green'} shape='circle' size='small'>
                {isEdit ? t('更新') : t('新建')}
              </Tag>
            </div>
            <div className='text-xs text-gray-500'>
              {t('配置基本信息、额度与访问限制')}
            </div>
          </div>
        </div>
      }
      visible={props.visiable}
      onCancel={() => handleCancel()}
      maskClosable={false}
      width={isMobile ? '100%' : 640}
      centered={!isMobile}
      bodyStyle={{
        padding: '0',
        background: '#ffffff',
        maxHeight: isMobile ? 'calc(100dvh - 152px)' : '68vh',
        overflowY: 'auto',
      }}
      style={{
        '--semi-color-primary': '#2563eb',
        '--semi-color-primary-hover': '#1d4ed8',
        '--semi-color-primary-active': '#1e40af',
        ...(isMobile
          ? { top: 0, margin: 0, maxWidth: '100vw', paddingBottom: 0 }
          : {}),
      }}
      footer={
        <div className='flex justify-end'>
          <Space>
            <Button
              theme='light'
              className='!rounded-lg'
              type='tertiary'
              onClick={handleCancel}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
            <Button
              theme='solid'
              type='primary'
              className='!rounded-lg'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconSave />}
              loading={loading}
            >
              {t('提交')}
            </Button>
          </Space>
        </div>
      }
    >
      <style>{`
        .token-edit-modal .semi-switch-checked { background-color: #2563eb !important; }
        .token-edit-modal .semi-switch-checked:hover { background-color: #1d4ed8 !important; }
      `}</style>
      <Spin spinning={loading}>
        <Form
          key={isEdit ? 'edit' : 'new'}
          initValues={getInitValues()}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={submit}
        >
          {({ values }) => (
            <div style={{ padding: '20px 22px 10px' }}>
              {/* 常用字段：一屏直出，统一两列栅格 */}
              <Row gutter={{ xs: 0, sm: 0, md: 16 }}>
                <Col xs={24} md={12}>
                  <Form.Input
                    field='name'
                    label={t('名称')}
                    placeholder={t('请输入名称')}
                    rules={[{ required: true, message: t('请输入名称') }]}
                    showClear
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} md={12}>
                  {groups.length > 0 ? (
                    <Form.Select
                      field='group'
                      label={t('令牌分组')}
                      placeholder={t('默认为你的分组')}
                      optionList={groups}
                      renderOptionItem={renderGroupOption}
                      filter={(input, option) => {
                        const q = input.toLowerCase();
                        return (
                          option.value?.toLowerCase().includes(q) ||
                          (typeof option.label === 'string' &&
                            option.label.toLowerCase().includes(q))
                        );
                      }}
                      showClear
                      style={{ width: '100%' }}
                    />
                  ) : (
                    <Form.Select
                      placeholder={t('管理员未设置用户可选分组')}
                      disabled
                      label={t('令牌分组')}
                      style={{ width: '100%' }}
                    />
                  )}
                </Col>

                <Col xs={24} md={16}>
                  <Form.InputNumber
                    field='remain_amount'
                    label={t('额度')}
                    prefix={getCurrencyConfig().symbol}
                    placeholder={values.unlimited_quota ? t('无限额度') : t('输入金额')}
                    precision={6}
                    disabled={values.unlimited_quota}
                    min={0}
                    step={0.000001}
                    onChange={(val) => {
                      const amount = val === '' || val == null ? 0 : val;
                      formApiRef.current?.setValue('remain_amount', amount);
                      formApiRef.current?.setValue(
                        'remain_quota',
                        displayAmountToQuota(amount),
                      );
                    }}
                    style={{ width: '100%' }}
                    showClear
                  />
                </Col>
                <Col xs={24} md={8}>
                  <Form.Switch field='unlimited_quota' label={t('无限额度')} />
                </Col>

                <Col span={24}>
                  <Form.DatePicker
                    field='expired_time'
                    label={t('过期时间')}
                    type='dateTime'
                    triggerRender={renderSafeDatePickerTrigger}
                    placeholder={t('请选择过期时间')}
                    rules={[
                      { required: true, message: t('请选择过期时间') },
                      {
                        validator: (rule, value) => {
                          if (value === -1 || !value) return Promise.resolve();
                          const time = Date.parse(value);
                          if (isNaN(time)) {
                            return Promise.reject(t('过期时间格式错误！'));
                          }
                          if (time <= Date.now()) {
                            return Promise.reject(t('过期时间不能早于当前时间！'));
                          }
                          return Promise.resolve();
                        },
                      },
                    ]}
                    showClear
                    style={{ width: '100%' }}
                  />
                  <div
                    style={{
                      display: 'flex',
                      flexWrap: 'wrap',
                      gap: 8,
                      marginTop: 8,
                    }}
                  >
                    {[
                      [t('永不过期'), () => setExpiredTime(0, 0, 0, 0)],
                      [t('一个月'), () => setExpiredTime(1, 0, 0, 0)],
                      [t('一天'), () => setExpiredTime(0, 1, 0, 0)],
                      [t('一小时'), () => setExpiredTime(0, 0, 1, 0)],
                    ].map(([label, onClick], i) => (
                      <Button
                        key={i}
                        size='small'
                        theme='light'
                        type={i === 0 ? 'primary' : 'tertiary'}
                        className='!rounded-lg'
                        onClick={onClick}
                      >
                        {label}
                      </Button>
                    ))}
                  </div>
                </Col>
              </Row>

              {/* 高级设置：折叠，收纳低频项 */}
              <div style={{ borderTop: '1px solid #eef1f6', marginTop: 18 }}>
                <button
                  type='button'
                  onClick={() => setAdvancedOpen((v) => !v)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 6,
                    width: '100%',
                    padding: '14px 0 2px',
                    background: 'none',
                    border: 0,
                    cursor: 'pointer',
                    color: '#374151',
                    fontSize: 13,
                    fontWeight: 600,
                  }}
                >
                  {t('高级设置')}
                  <span style={{ marginLeft: 'auto', color: '#9aa4b2', fontWeight: 500 }}>
                    {advancedOpen ? `${t('收起')} ▴` : `${t('展开')} ▾`}
                  </span>
                </button>

                <div style={{ display: advancedOpen ? 'block' : 'none', paddingTop: 12 }}>
                  <Row gutter={{ xs: 0, sm: 0, md: 16 }}>
                    {!isEdit && (
                      <Col xs={24} md={12}>
                        <Form.InputNumber
                          field='tokenCount'
                          label={t('新建数量')}
                          min={1}
                          extraText={t('批量创建时名称后自动加随机后缀')}
                          rules={[
                            { required: true, message: t('请输入新建数量') },
                          ]}
                          style={{ width: '100%' }}
                        />
                      </Col>
                    )}
                    <Col
                      xs={24}
                      md={12}
                      style={{
                        display: values.group === 'auto' ? 'block' : 'none',
                      }}
                    >
                      <Form.Switch
                        field='cross_group_retry'
                        label={t('跨分组重试')}
                        extraText={t('当前分组失败时按序尝试下一分组')}
                      />
                    </Col>
                    <Col span={24}>
                      <Form.Select
                        field='model_limits'
                        label={t('模型限制列表')}
                        placeholder={t('留空支持所有模型')}
                        multiple
                        optionList={models}
                        filter={selectFilter}
                        autoClearSearchValue={false}
                        searchPosition='dropdown'
                        showClear
                        style={{ width: '100%' }}
                      />
                    </Col>
                    <Col span={24}>
                      <Form.TextArea
                        field='allow_ips'
                        label={t('IP 白名单（支持 CIDR）')}
                        placeholder={t('允许的 IP，一行一个，留空不限制')}
                        autosize
                        rows={1}
                        showClear
                        style={{ width: '100%' }}
                      />
                    </Col>
                    <Col span={24}>
                      <Form.InputNumber
                        field='remain_quota'
                        label={t('原生额度')}
                        placeholder={t('按 quota 直接输入')}
                        disabled={values.unlimited_quota}
                        min={0}
                        step={500000}
                        onChange={(val) => {
                          const quota = val === '' || val == null ? 0 : val;
                          formApiRef.current?.setValue('remain_quota', quota);
                          formApiRef.current?.setValue(
                            'remain_amount',
                            Number(quotaToDisplayAmount(quota).toFixed(6)),
                          );
                        }}
                        style={{ width: '100%' }}
                        showClear
                      />
                    </Col>
                  </Row>
                </div>
              </div>
            </div>
          )}
        </Form>
      </Spin>
    </Modal>
  );
};

export default EditTokenModal;
