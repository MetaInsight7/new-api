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
import { Button } from '@douyinfe/semi-ui';
import { IconLock, IconDelete } from '@douyinfe/semi-icons';
import { ShieldCheck } from 'lucide-react';
import TwoFASetting from '../components/TwoFASetting';

const ICO_BLUE = { background: 'rgba(37,99,235,0.10)', color: '#2563eb' };
const ICO_RED = { background: 'rgba(213,52,59,0.10)', color: '#d5343b' };

const iconChip = (tint) => ({
  width: 34,
  height: 34,
  borderRadius: 9,
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  flex: '0 0 auto',
  ...tint,
});

const Pill = ({ ok, children }) => (
  <span
    style={{
      display: 'inline-flex',
      alignItems: 'center',
      height: 20,
      padding: '0 8px',
      borderRadius: 999,
      fontSize: 11.5,
      fontWeight: 600,
      background: ok ? 'rgba(15,157,110,0.10)' : '#eef0f3',
      color: ok ? '#0f9d6e' : '#6b7686',
    }}
  >
    {children}
  </span>
);

// 通用一行：图标 + 标题(+状态) + 说明 + 右侧操作
const Row = ({ tint = ICO_BLUE, icon, title, pill, desc, action, divider }) => (
  <div
    style={{
      display: 'flex',
      alignItems: 'center',
      gap: 12,
      padding: '13px 0',
      borderBottom: divider ? '1px solid #eef1f6' : 'none',
    }}
  >
    <span style={iconChip(tint)}>{icon}</span>
    <div style={{ flex: 1, minWidth: 0 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <span style={{ fontSize: 13.5, fontWeight: 600, color: '#141a1f' }}>
          {title}
        </span>
        {pill}
      </div>
      <div style={{ fontSize: 12, color: '#6b7686', marginTop: 1 }}>{desc}</div>
    </div>
    <div style={{ flex: '0 0 auto' }}>{action}</div>
  </div>
);

const SecuritySettings = ({
  t,
  setShowChangePasswordModal,
  setShowAccountDeleteModal,
}) => {
  return (
    <div
      style={{
        border: '1px solid #eef1f6',
        borderRadius: 14,
        padding: '4px 18px 6px',
        background: '#fff',
      }}
    >
      {/* 卡头 */}
      <div
        style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '14px 0 6px' }}
      >
        <span
          style={{
            width: 30,
            height: 30,
            borderRadius: 9,
            background: 'rgba(37,99,235,0.08)',
            color: '#2563eb',
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <ShieldCheck size={16} />
        </span>
        <span style={{ fontSize: 14, fontWeight: 600, letterSpacing: '-0.01em', color: '#141a1f' }}>
          {t('安全设置')}
        </span>
      </div>

      {/* 登录密码 */}
      <Row
        divider
        icon={<IconLock />}
        title={t('登录密码')}
        pill={<Pill ok>{t('已设置')}</Pill>}
        desc={t('定期更改密码可提高账户安全性')}
        action={
          <Button
            theme='solid'
            type='primary'
            size='small'
            className='!rounded-lg'
            onClick={() => setShowChangePasswordModal(true)}
          >
            {t('修改')}
          </Button>
        }
      />

      {/* 两步验证（组件内部自带状态与弹窗，已压缩为紧凑行） */}
      <TwoFASetting t={t} />

      {/* 危险区：删除账户 */}
      <div style={{ marginTop: 4, paddingTop: 12, borderTop: '1px dashed #f0d9da' }}>
        <Row
          tint={ICO_RED}
          icon={<IconDelete />}
          title={<span style={{ color: '#d5343b' }}>{t('删除账户')}</span>}
          desc={t('此操作不可逆，所有数据将被永久删除')}
          action={
            <Button
              theme='solid'
              type='danger'
              size='small'
              className='!rounded-lg'
              onClick={() => setShowAccountDeleteModal(true)}
            >
              {t('删除')}
            </Button>
          }
        />
      </div>
    </div>
  );
};

export default SecuritySettings;
