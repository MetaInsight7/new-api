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

import React, { useState, useRef, useEffect } from 'react';
import { Modal, Form, Col, Row } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';
import { Typography } from '@douyinfe/semi-ui';
import { IconLink } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { useRequestLifecycle } from '../../../../hooks/common/useRequestLifecycle';

const EditVendorModal = ({ visible, handleClose, refresh, editingVendor }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const formApiRef = useRef(null);
  const { beginRequest, isCurrentRequest } = useRequestLifecycle();

  const isMobile = useIsMobile();
  const isEdit = editingVendor && editingVendor.id !== undefined;

  const getInitValues = () => ({
    name: '',
    description: '',
    icon: '',
    status: true,
  });

  const handleCancel = () => {
    handleClose();
    formApiRef.current?.reset();
  };

  const loadVendor = async () => {
    if (!isEdit || !editingVendor.id) return;

    const requestId = beginRequest('vendor');
    setLoading(true);
    try {
      const res = await API.get(`/api/vendors/${editingVendor.id}`);
      const { success, message, data } = res.data;
      if (isCurrentRequest('vendor', requestId) && success) {
        // 将数字状态转为布尔值
        data.status = data.status === 1;
        if (isCurrentRequest('vendor', requestId) && formApiRef.current) {
          formApiRef.current.setValues({ ...getInitValues(), ...data });
        }
      } else if (isCurrentRequest('vendor', requestId)) {
        showError(message);
      }
    } catch (error) {
      if (isCurrentRequest('vendor', requestId)) {
        showError(t('加载供应商信息失败'));
      }
    }
    if (isCurrentRequest('vendor', requestId)) setLoading(false);
  };

  useEffect(() => {
    if (visible) {
      if (isEdit) {
        loadVendor();
      } else {
        formApiRef.current?.setValues(getInitValues());
      }
    } else {
      beginRequest('vendor');
      beginRequest('submit');
      setLoading(false);
      formApiRef.current?.reset();
    }
  }, [visible, editingVendor?.id]);

  const submit = async (values) => {
    const requestId = beginRequest('submit');
    setLoading(true);
    try {
      // 转换 status 为数字
      const submitData = {
        ...values,
        status: values.status ? 1 : 0,
      };

      if (isEdit) {
        submitData.id = editingVendor.id;
        const res = await API.put('/api/vendors/', submitData);
        const { success, message } = res.data;
        if (isCurrentRequest('submit', requestId) && success) {
          showSuccess(t('供应商更新成功！'));
          refresh();
          handleClose();
        } else if (isCurrentRequest('submit', requestId)) {
          showError(t(message));
        }
      } else {
        const res = await API.post('/api/vendors/', submitData);
        const { success, message } = res.data;
        if (isCurrentRequest('submit', requestId) && success) {
          showSuccess(t('供应商创建成功！'));
          refresh();
          handleClose();
        } else if (isCurrentRequest('submit', requestId)) {
          showError(t(message));
        }
      }
    } catch (error) {
      if (isCurrentRequest('submit', requestId)) {
        showError(error.response?.data?.message || t('操作失败'));
      }
    }
    if (isCurrentRequest('submit', requestId)) setLoading(false);
  };

  return (
    <Modal
      title={isEdit ? t('编辑供应商') : t('新增供应商')}
      visible={visible}
      onOk={() => formApiRef.current?.submitForm()}
      onCancel={handleCancel}
      confirmLoading={loading}
      okButtonProps={{ theme: 'solid' }}
      cancelButtonProps={{ theme: 'borderless' }}
      size={isMobile ? 'full-width' : 'small'}
    >
      <Form
        initValues={getInitValues()}
        getFormApi={(api) => (formApiRef.current = api)}
        onSubmit={submit}
      >
        <Row gutter={12}>
          <Col span={24}>
            <Form.Input
              field='name'
              label={t('供应商名称')}
              placeholder={t('请输入供应商名称，如：OpenAI')}
              rules={[{ required: true, message: t('请输入供应商名称') }]}
              showClear
            />
          </Col>
          <Col span={24}>
            <Form.TextArea
              field='description'
              label={t('描述')}
              placeholder={t('请输入供应商描述')}
              rows={3}
              showClear
            />
          </Col>
          <Col span={24}>
            <Form.Input
              field='icon'
              label={t('供应商图标')}
              placeholder={t('请输入图标名称')}
              extraText={
                <span>
                  {t(
                    "图标使用@lobehub/icons库，如：OpenAI、Claude.Color，支持链式参数：OpenAI.Avatar.type={'platform'}、OpenRouter.Avatar.shape={'square'}，查询所有可用图标请 ",
                  )}
                  <Typography.Text
                    link={{
                      href: 'https://icons.lobehub.com/components/lobe-hub',
                      target: '_blank',
                    }}
                    icon={<IconLink />}
                    underline
                  >
                    {t('请点击我')}
                  </Typography.Text>
                </span>
              }
              showClear
            />
          </Col>
          <Col span={24}>
            <Form.Switch field='status' label={t('状态')} initValue={true} />
          </Col>
        </Row>
      </Form>
    </Modal>
  );
};

export default EditVendorModal;
