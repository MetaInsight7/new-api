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
import ArrowRight from 'lucide-react/dist/esm/icons/arrow-right';
import MailCheck from 'lucide-react/dist/esm/icons/mail-check';
import ShieldCheck from 'lucide-react/dist/esm/icons/shield-check';
import Sparkles from 'lucide-react/dist/esm/icons/sparkles';
import UserPlus from 'lucide-react/dist/esm/icons/user-plus';
import Zap from 'lucide-react/dist/esm/icons/zap';
import { useTranslation } from 'react-i18next';

const REGISTER_STEPS = [
  {
    key: 'account',
    label: '创建入口',
    Icon: UserPlus,
  },
  {
    key: 'verify',
    label: '连接模型',
    Icon: ShieldCheck,
  },
  {
    key: 'ready',
    label: '开始创作',
    Icon: Sparkles,
  },
];

export const AuthFormHeader = ({ title, subtitle, logo }) => {
  return (
    <div className='auth-form-header'>
      <span className='auth-brand-mark' aria-hidden='true'>
        {logo ? (
          <img src={logo} alt='' className='auth-brand-logo' />
        ) : (
          <Zap size={18} strokeWidth={2.4} />
        )}
      </span>
      <div className='auth-heading-copy'>
        <h2 className='auth-title'>{title}</h2>
        {subtitle && <p className='auth-subtitle'>{subtitle}</p>}
      </div>
    </div>
  );
};

export const AuthRegisterSteps = () => {
  const { t } = useTranslation();

  return (
    <div className='auth-register-steps' aria-hidden='true'>
      {REGISTER_STEPS.map(({ key, label, Icon }, index) => (
        <React.Fragment key={key}>
          <div className='auth-register-step'>
            <span className={`auth-register-step-icon auth-step-${key}`}>
              <Icon size={16} strokeWidth={2.2} />
            </span>
            <span className='auth-register-step-label'>{t(label)}</span>
          </div>
          {index < REGISTER_STEPS.length - 1 && (
            <span className='auth-register-step-line' />
          )}
        </React.Fragment>
      ))}
    </div>
  );
};

export const AuthButtonContent = ({ children, icon = 'arrow' }) => {
  const Icon = icon === 'mail' ? MailCheck : ArrowRight;

  return (
    <span className='auth-button-content'>
      <span>{children}</span>
      <span className='auth-button-icon' aria-hidden='true'>
        <Icon size={17} strokeWidth={2.4} />
      </span>
    </span>
  );
};
