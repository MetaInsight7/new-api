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
import { getLogo, getSystemName } from '../../helpers';

const SiderBar = ({ onNavigate = () => {} }) => {
  const { categories, activeCategory, categoryTarget } = useConsoleNav();

  const logo = getLogo();
  const systemName = getSystemName();

  return (
    <div className='dmit-sider'>
      <Link to='/' className='dmit-sider__logo' onClick={onNavigate}>
        {logo && <img src={logo} alt='logo' className='dmit-sider__logo-img' />}
        <span className='dmit-sider__logo-text'>{systemName}</span>
      </Link>

      <nav className='dmit-sider__menu'>
        {categories.map((cat) => {
          const Icon = cat.icon;
          const active = activeCategory && activeCategory.key === cat.key;
          return (
            <Link
              key={cat.key}
              to={categoryTarget(cat)}
              onClick={onNavigate}
              className={`dmit-menu-item ${active ? 'dmit-menu-item--active' : ''}`}
            >
              <span className='dmit-menu-item__icon'>
                <Icon size={34} strokeWidth={1.75} />
              </span>
              <span className='dmit-menu-item__title'>{cat.label}</span>
            </Link>
          );
        })}
      </nav>
    </div>
  );
};

export default SiderBar;
