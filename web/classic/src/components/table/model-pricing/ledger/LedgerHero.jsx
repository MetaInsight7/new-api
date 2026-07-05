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

import React, { useContext } from 'react';
import { useNavigate } from 'react-router-dom';
import { UserContext } from '../../../../context/User';

const LedgerHero = ({ t }) => {
  const navigate = useNavigate();
  const [userState] = useContext(UserContext);
  const loggedIn = !!userState?.user;

  return (
    <div className='hero-grid'>
      <div className='hero-copy'>
        <span className='kicker'>
          <i />
          {t('价格表 · Pricing · 实时同步')}
        </span>
        <h1>
          {t('所有大模型，')}
          <em>{t('一张价目表看全')}</em>
        </h1>
        <p className='lead'>
          {t(
            '统一接入 40+ 供应商，按量计费，价格随令牌分组透明公开，注册即用。',
          )}
        </p>
        <div className='herobar'>
          <button
            type='button'
            className='cta-hero'
            onClick={() => navigate(loggedIn ? '/console/token' : '/register')}
          >
            {loggedIn ? t('去创建令牌') : t('免费注册领额度')} <i>›</i>
          </button>
        </div>
      </div>

      <aside className='hero-mascot'>
        <img src='/token-dazi-logo.svg' alt={t('Token 搭子 吉祥物')} />
      </aside>
    </div>
  );
};

export default LedgerHero;
