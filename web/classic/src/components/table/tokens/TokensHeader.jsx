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

import React, { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import { Button } from '@douyinfe/semi-ui';
import { Plus } from 'lucide-react';

// API 密钥管理表头工具栏(原型 .tmp/token-proto5.html)
const TokensHeader = ({ setEditingToken, setShowEdit, t }) => {
  const [actionsHost, setActionsHost] = useState(null);

  useEffect(() => {
    setActionsHost(document.getElementById('token-page-actions'));
  }, []);

  return (
    <>
      {actionsHost &&
        createPortal(
          <>
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
              {t('添加 API 密钥')}
            </Button>
          </>,
          actionsHost,
        )}
    </>
  );
};

export default TokensHeader;
