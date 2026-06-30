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

// DMIT 扁平统计小卡：彩色方块图标 + 弱化标签 + 粗值
// tone: 'blue' | 'violet' | 'emerald' | 'amber'
const StatTile = ({ tone = 'blue', icon: Icon, label, value }) => (
  <div className={`dmit-stat-tile is-${tone}`}>
    <div className='dmit-stat-tile__head'>
      <span className='dmit-stat-tile__icon'>
        {Icon ? <Icon size={15} /> : null}
      </span>
      <span className='dmit-stat-tile__label'>{label}</span>
    </div>
    <div className='dmit-stat-tile__value'>{value}</div>
  </div>
);

export default StatTile;
