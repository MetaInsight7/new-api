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
import { Link } from 'react-router-dom';
import { useConsoleNav } from '../../hooks/common/useConsoleNav';
import { getLucideIcon } from '../../helpers/render';

/**
 * 二级导航（DMIT 风格 pill 条）：
 * 渲染当前大组的子页为横向 pill，置于内容区顶部。
 * 仅当当前组存在 ≥2 个子页时显示（看板等单页组不显示）。
 */
const ConsoleSubNav = () => {
  const { activeCategory, selectedKey } = useConsoleNav();

  const children = activeCategory?.children || [];
  if (children.length < 2) return null;

  return (
    <div className='dmit-subnav'>
      {children.map((ch) => {
        const active = ch.key === selectedKey;
        return (
          <Link
            key={ch.key}
            to={ch.to}
            className={`dmit-subnav__item ${
              active ? 'dmit-subnav__item--active' : ''
            }`}
          >
            <span className='dmit-subnav__icon'>{getLucideIcon(ch.key)}</span>
            <span>{ch.label}</span>
          </Link>
        );
      })}
      {/* 右侧动作槽:各页面通过 portal 注入(如日志的时间区间 + 筛选) */}
      <div className='dmit-subnav__actions' id='console-subnav-actions' />
    </div>
  );
};

export default ConsoleSubNav;
