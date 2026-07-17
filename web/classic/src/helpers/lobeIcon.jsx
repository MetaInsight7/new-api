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
import { Avatar } from '@douyinfe/semi-ui';
import * as LobeIcons from '@lobehub/icons';

function parseValue(raw) {
  if (raw == null) return true;
  let v = String(raw).trim();
  if (v.startsWith('{') && v.endsWith('}')) v = v.slice(1, -1).trim();
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) return v.slice(1, -1);
  if (v === 'true') return true;
  if (v === 'false') return false;
  if (/^-?\d+(?:\.\d+)?$/.test(v)) return Number(v);
  return v;
}

export function getLobeIcon(iconName, size = 20) {
  if (!iconName || typeof iconName !== 'string' || !iconName.trim()) {
    return (
      <div className='flex items-center justify-center rounded-full text-xs font-medium'
        style={{ width: size, height: size, backgroundColor: 'var(--semi-color-fill-0)', color: 'var(--semi-color-text-2)' }}>
        {iconName ? iconName.charAt(0).toUpperCase() : '?'}
      </div>
    );
  }

  const segments = iconName.trim().split('.');
  const baseKey = segments[0];
  const BaseIcon = LobeIcons[baseKey];

  let IconComponent;
  let propStartIndex;

  if (BaseIcon && segments.length > 1 && BaseIcon[segments[1]]) {
    IconComponent = BaseIcon[segments[1]];
    propStartIndex = 2;
  } else {
    IconComponent = LobeIcons[baseKey];
    propStartIndex = segments.length > 1 && /^[A-Z]/.test(segments[1]) ? 2 : 1;
  }

  if (!IconComponent || (typeof IconComponent !== 'function' && typeof IconComponent !== 'object')) {
    return (
      <div className='flex items-center justify-center rounded-full text-xs font-medium'
        style={{ width: size, height: size, backgroundColor: 'var(--semi-color-fill-0)', color: 'var(--semi-color-text-2)' }}>
        {baseKey.charAt(0).toUpperCase()}
      </div>
    );
  }

  const props = {};
  for (let i = propStartIndex; i < segments.length; i++) {
    const seg = segments[i];
    if (!seg) continue;
    const eqIdx = seg.indexOf('=');
    if (eqIdx === -1) { props[seg.trim()] = true; continue; }
    props[seg.slice(0, eqIdx).trim()] = parseValue(seg.slice(eqIdx + 1).trim());
  }
  if (props.size == null) props.size = size;

  return <IconComponent {...props} />;
}

export function getLobeHubIcon(iconName, size = 14) {
  const normalizedIconName = String(iconName || '').trim();
  if (!normalizedIconName) {
    return <Avatar size='extra-extra-small'>?</Avatar>;
  }

  const segments = normalizedIconName.split('.');
  const baseKey = segments[0];
  const BaseIcon = LobeIcons[baseKey];
  let IconComponent;
  let propStartIndex = 1;

  if (BaseIcon && segments.length > 1 && BaseIcon[segments[1]]) {
    IconComponent = BaseIcon[segments[1]];
    propStartIndex = 2;
  } else {
    IconComponent = BaseIcon;
  }

  if (
    !IconComponent ||
    (typeof IconComponent !== 'function' && typeof IconComponent !== 'object')
  ) {
    return (
      <Avatar size='extra-extra-small'>
        {normalizedIconName.charAt(0).toUpperCase()}
      </Avatar>
    );
  }

  const props = {};
  for (let index = propStartIndex; index < segments.length; index += 1) {
    const segment = segments[index];
    if (!segment) continue;
    const equalIndex = segment.indexOf('=');
    if (equalIndex === -1) {
      props[segment.trim()] = true;
      continue;
    }
    props[segment.slice(0, equalIndex).trim()] = parseValue(
      segment.slice(equalIndex + 1).trim(),
    );
  }

  if (props.size == null && size != null) props.size = size;
  return <IconComponent {...props} />;
}
