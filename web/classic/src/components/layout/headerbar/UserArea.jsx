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

import React, { useRef } from 'react';
import { Link } from 'react-router-dom';
import { Avatar, Button, Dropdown } from '@douyinfe/semi-ui';
import { ChevronDown } from 'lucide-react';
import {
  IconExit,
  IconUserSetting,
  IconCreditCard,
  IconKey,
} from '@douyinfe/semi-icons';
import SkeletonWrapper from '../components/SkeletonWrapper';

const AVATAR_COLORS = [
  'amber',
  'blue',
  'cyan',
  'green',
  'grey',
  'indigo',
  'light-blue',
  'lime',
  'orange',
  'pink',
  'purple',
  'red',
  'teal',
  'violet',
  'yellow',
];

const stringToColor = (value = '') => {
  let sum = 0;
  for (let index = 0; index < value.length; index += 1) {
    sum += value.charCodeAt(index);
  }

  return AVATAR_COLORS[sum % AVATAR_COLORS.length];
};

const UserArea = ({
  userState,
  isLoading,
  isMobile,
  isSelfUseMode,
  logout,
  navigate,
  t,
}) => {
  const dropdownRef = useRef(null);
  if (isLoading) {
    return <span className='tn-av-skeleton' aria-hidden='true' />;
  }

  if (userState.user) {
    const uname = userState.user.username || '';
    const initial = (uname[0] || '?').toUpperCase();
    const avColor = stringToColor(uname);
    const role = userState.user.role || 0;
    const roleLabel =
      role >= 100 ? t('站长') : role >= 10 ? t('管理员') : t('用户');

    return (
      <div className='relative tn-userwrap' ref={dropdownRef}>
        <Dropdown
          position='bottomRight'
          getPopupContainer={() => dropdownRef.current}
          render={
            <Dropdown.Menu className='tn-userpop'>
              <div className='tn-userpop-head'>
                <span className='tn-userpop-av' style={{ background: avColor }}>
                  {initial}
                </span>
                <div className='tn-userpop-meta'>
                  <div className='tn-userpop-name'>{uname}</div>
                  <div className='tn-userpop-sub'>
                    {userState.user.email || roleLabel}
                  </div>
                </div>
                <span className='tn-userpop-role'>{roleLabel}</span>
              </div>

              <Dropdown.Item
                className='tn-userpop-item'
                onClick={() => navigate('/console/personal')}
              >
                <IconUserSetting size='small' />
                <span>{t('个人设置')}</span>
              </Dropdown.Item>
              <Dropdown.Item
                className='tn-userpop-item'
                onClick={() => navigate('/console/token')}
              >
                <IconKey size='small' />
                <span>{t('令牌管理')}</span>
              </Dropdown.Item>
              <Dropdown.Item
                className='tn-userpop-item'
                onClick={() => navigate('/console/topup')}
              >
                <IconCreditCard size='small' />
                <span>{t('钱包管理')}</span>
              </Dropdown.Item>

              <Dropdown.Divider />

              <Dropdown.Item
                className='tn-userpop-item tn-logout'
                onClick={logout}
              >
                <IconExit size='small' />
                <span>{t('退出')}</span>
              </Dropdown.Item>
            </Dropdown.Menu>
          }
        >
          <Button
            theme='borderless'
            type='tertiary'
            className='tn-avatar'
            aria-label={uname}
          >
            <span className='tn-av' style={{ background: avColor }}>
              {initial}
            </span>
          </Button>
        </Dropdown>
      </div>
    );
  } else {
    const showRegisterButton = !isSelfUseMode;

    return (
      <div className='flex items-center gap-1'>
        <Link to='/login' className='flex'>
          <Button theme='borderless' type='tertiary' className='tn-login'>
            <span>{t('登录')}</span>
          </Button>
        </Link>
        {showRegisterButton && (
          <div className='hidden md:block'>
            <Link to='/register' className='flex'>
              <Button theme='solid' type='primary' className='tn-register'>
                <span>{t('注册')}</span>
              </Button>
            </Link>
          </div>
        )}
      </div>
    );
  }
};

export default UserArea;
