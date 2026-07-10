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
import { Inbox } from 'lucide-react';

// DMIT 风空状态:柔和方块图标 + 标题(+ 可选副标题 / 引导操作)
// 取代各表格页的 Semi 卡通插画,统一为扁平、克制的内联空状态
const TableEmpty = ({ title, description, icon: Icon = Inbox, action }) => (
  <div className='flex flex-col items-center justify-center text-center px-6 py-14'>
    <span
      className='inline-flex items-center justify-center w-14 h-14 rounded-2xl mb-4'
      style={{ background: 'var(--semi-color-fill-0)', color: 'var(--semi-color-text-2)' }}
    >
      <Icon size={26} strokeWidth={1.75} />
    </span>
    <div
      className='text-[15px] font-semibold'
      style={{ color: 'var(--semi-color-text-2)' }}
    >
      {title}
    </div>
    {description && (
      <div
        className='text-[13px] mt-1 max-w-[280px]'
        style={{ color: 'var(--semi-color-text-2)' }}
      >
        {description}
      </div>
    )}
    {action && <div className='mt-4'>{action}</div>}
  </div>
);

export default TableEmpty;
