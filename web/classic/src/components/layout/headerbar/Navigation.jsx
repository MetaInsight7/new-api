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
import { Link, useLocation } from 'react-router-dom';

const Navigation = ({
  mainNavLinks,
  isMobile,
  isLoading,
  userState,
  pricingRequireAuth,
  isConsoleRoute,
}) => {
  const location = useLocation();
  const isActive = (to) => {
    if (!to) return false;
    if (to === '/') return location.pathname === '/';
    return location.pathname === to || location.pathname.startsWith(`${to}/`);
  };

  const renderNavLinks = () => {
    return mainNavLinks.map((link) => {
      if (link.isExternal) {
        return (
          <a
            key={link.itemKey}
            href={link.externalLink}
            target='_blank'
            rel='noopener noreferrer'
            className='tn-link'
          >
            {link.text}
            <span className='tn-ext'>↗</span>
          </a>
        );
      }

      let targetPath = link.to;
      if (link.itemKey === 'console' && !userState.user) {
        targetPath = '/login';
      }
      if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
        targetPath = '/login';
      }

      return (
        <Link
          key={link.itemKey}
          to={targetPath}
          aria-current={isActive(link.to) ? 'page' : undefined}
          className={`tn-link${isActive(link.to) ? ' tn-on' : ''}`}
        >
          <span>{link.text}</span>
        </Link>
      );
    });
  };

  return (
    <nav
      aria-label='主导航'
      aria-busy={isLoading ? 'true' : undefined}
      className='flex flex-1 items-center gap-1 lg:gap-2 mx-2 md:mx-4 overflow-x-auto whitespace-nowrap scrollbar-hide lg:absolute lg:left-1/2 lg:top-1/2 lg:-translate-x-1/2 lg:-translate-y-1/2 lg:flex-none lg:mx-0'
    >
      {renderNavLinks()}
    </nav>
  );
};

export default Navigation;
