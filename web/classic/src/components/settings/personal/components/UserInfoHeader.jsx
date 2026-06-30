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
import { Avatar, Card, Tag } from '@douyinfe/semi-ui';
import {
  isRoot,
  isAdmin,
  renderQuota,
  stringToColor,
} from '../../../../helpers';
import { Wallet, Coins, BarChart2, Users } from 'lucide-react';
import StatTile from '../../../common/ui/StatTile';

const UserInfoHeader = ({ t, userState }) => {
  const getUsername = () => {
    if (userState.user) {
      return userState.user.username;
    } else {
      return 'null';
    }
  };

  const getAvatarText = () => {
    const username = getUsername();
    if (username && username.length > 0) {
      return username.slice(0, 2).toUpperCase();
    }
    return 'NA';
  };

  const roleTag = isRoot() ? (
    <Tag color='red' shape='circle'>
      {t('超级管理员')}
    </Tag>
  ) : isAdmin() ? (
    <Tag color='blue' shape='circle'>
      {t('管理员')}
    </Tag>
  ) : (
    <Tag color='grey' shape='circle'>
      {t('普通用户')}
    </Tag>
  );

  return (
    <Card className='!rounded-2xl'>
      {/* 头部：头像 + 用户名 + 角色胶囊 */}
      <div className='flex items-center gap-4 mb-5'>
        <Avatar size='large' color={stringToColor(getUsername())}>
          {getAvatarText()}
        </Avatar>
        <div className='min-w-0 flex-1'>
          <div className='text-2xl font-bold truncate text-[var(--semi-color-text-0)]'>
            {getUsername()}
          </div>
          <div className='flex flex-wrap items-center gap-2 mt-1.5'>
            {roleTag}
            <Tag color='white' shape='circle'>
              ID: {userState?.user?.id}
            </Tag>
          </div>
        </div>
      </div>

      {/* 统计小卡：余额 / 历史消耗 / 请求次数 / 用户分组 */}
      <div className='grid grid-cols-2 sm:grid-cols-4 gap-3'>
        <StatTile
          tone='blue'
          icon={Wallet}
          label={t('当前余额')}
          value={renderQuota(userState?.user?.quota)}
        />
        <StatTile
          tone='violet'
          icon={Coins}
          label={t('历史消耗')}
          value={renderQuota(userState?.user?.used_quota)}
        />
        <StatTile
          tone='emerald'
          icon={BarChart2}
          label={t('请求次数')}
          value={userState?.user?.request_count || 0}
        />
        <StatTile
          tone='amber'
          icon={Users}
          label={t('用户分组')}
          value={userState?.user?.group || t('默认')}
        />
      </div>
    </Card>
  );
};

export default UserInfoHeader;
