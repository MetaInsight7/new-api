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

import React, { useState } from 'react';
import { Form, Button, Dropdown } from '@douyinfe/semi-ui';
import { Search, Plus, ChevronDown, Copy, Trash2 } from 'lucide-react';
import { showError } from '../../../helpers';
import CopyTokensModal from './modals/CopyTokensModal';
import DeleteTokensModal from './modals/DeleteTokensModal';

// 令牌管理表头工具栏(原型 .tmp/token-proto5.html)
const TokensHeader = ({
  tokenCount,
  formInitValues,
  setFormApi,
  searchTokens,
  loading,
  searching,
  selectedKeys,
  batchCopyTokens,
  batchDeleteTokens,
  setEditingToken,
  setShowEdit,
  t,
}) => {
  const [showCopyModal, setShowCopyModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);

  const requireSelection = (fn) => () => {
    if (!selectedKeys || selectedKeys.length === 0) {
      showError(t('请至少选择一个令牌！'));
      return;
    }
    fn();
  };

  return (
    <Form
      initValues={formInitValues}
      getFormApi={(api) => setFormApi(api)}
      onSubmit={() => searchTokens(1)}
      allowEmpty={true}
      autoComplete='off'
      layout='horizontal'
      trigger='change'
      stopValidateWithError={false}
      className='w-full'
    >
      <div className='token-toolbar'>
        <div className='token-toolbar__title'>
          {t('令牌管理')}
          <span className='token-toolbar__count'>
            {tokenCount} {t('个令牌')}
          </span>
        </div>

        <div className='token-toolbar__actions'>
          <div className='token-toolbar__search'>
            <Form.Input
              field='searchKeyword'
              prefix={<Search size={15} />}
              placeholder={t('搜索名称或密钥')}
              showClear
              pure
              size='small'
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  searchTokens(1);
                }
              }}
            />
          </div>

          <Dropdown
            trigger='click'
            position='bottomRight'
            clickToHide
            menu={[
              {
                node: 'item',
                name: t('复制所选令牌'),
                icon: <Copy size={14} />,
                onClick: requireSelection(() => setShowCopyModal(true)),
              },
              {
                node: 'item',
                name: t('删除所选令牌'),
                type: 'danger',
                icon: <Trash2 size={14} />,
                onClick: requireSelection(() => setShowDeleteModal(true)),
              },
            ]}
          >
            <Button
              type='tertiary'
              theme='light'
              size='small'
              iconPosition='right'
              icon={<ChevronDown size={15} />}
            >
              {t('批量操作')}
            </Button>
          </Dropdown>

          <Button
            type='primary'
            theme='solid'
            size='small'
            icon={<Plus size={15} />}
            onClick={() => {
              setEditingToken({ id: undefined });
              setShowEdit(true);
            }}
          >
            {t('添加令牌')}
          </Button>
        </div>
      </div>

      <CopyTokensModal
        visible={showCopyModal}
        onCancel={() => setShowCopyModal(false)}
        batchCopyTokens={batchCopyTokens}
        t={t}
      />
      <DeleteTokensModal
        visible={showDeleteModal}
        onCancel={() => setShowDeleteModal(false)}
        onConfirm={() => {
          batchDeleteTokens();
          setShowDeleteModal(false);
        }}
        selectedKeys={selectedKeys}
        t={t}
      />
    </Form>
  );
};

export default TokensHeader;
