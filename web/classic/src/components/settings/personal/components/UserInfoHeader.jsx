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
import { isRoot, isAdmin } from '../../../../helpers';

// 个人中心顶部：紧凑身份行（头像 + 用户名 + 角色/ID/分组胶囊）
const UserInfoHeader = ({ t, userState }) => {
  const user = userState?.user || {};
  const username = user.username || 'null';
  const avatarText =
    username && username.length > 0 ? username.slice(0, 2).toUpperCase() : 'NA';
  const roleLabel = isRoot()
    ? t('超级管理员')
    : isAdmin()
      ? t('管理员')
      : t('普通用户');

  const pill = {
    display: 'inline-flex',
    alignItems: 'center',
    height: 22,
    padding: '0 10px',
    borderRadius: 999,
    fontSize: 12,
    fontWeight: 600,
    background: '#eef0f3',
    color: '#4a5364',
  };

  return (
    <div className='flex items-center gap-4'>
      <div
        className='flex items-center justify-center flex-shrink-0'
        style={{
          width: 50,
          height: 50,
          borderRadius: 14,
          background: '#2563eb',
          color: '#fff',
          fontSize: 18,
          fontWeight: 700,
        }}
      >
        {avatarText}
      </div>
      <div className='min-w-0'>
        <div className='text-xl font-extrabold truncate leading-tight text-[var(--semi-color-text-0)]'>
          {username}
        </div>
        <div className='flex flex-wrap items-center gap-1.5 mt-1.5'>
          <span style={pill}>{roleLabel}</span>
          <span style={pill}>ID {user.id}</span>
          <span style={pill}>{user.group || t('默认')}</span>
        </div>
      </div>
    </div>
  );
};

export default UserInfoHeader;
