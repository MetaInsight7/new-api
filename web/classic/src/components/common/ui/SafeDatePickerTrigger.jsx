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
import { Input } from '@douyinfe/semi-ui';

/**
 * Semi 2.99 forwards rangeSeparatorNode to its native input under React 19.
 * Keep the DatePicker trigger controlled while explicitly forwarding only
 * supported Input props.
 */
export const renderSafeDatePickerTrigger = (props = {}) => {
  const {
    inputValue = '',
    placeholder,
    value: _value,
    clearIcon,
    disabled,
    inputReadOnly,
    insetLabel,
    insetLabelId,
    prefix,
    size,
    autofocus,
    showClear,
    onEnterPress,
    onChange,
    onClear,
    onBlur,
    onFocus,
    inputStyle,
    validateStatus,
  } = props;

  return (
    <Input
      value={inputValue}
      placeholder={Array.isArray(placeholder) ? placeholder.join(' ~ ') : placeholder}
      clearIcon={clearIcon}
      disabled={disabled}
      readonly={inputReadOnly}
      insetLabel={insetLabel}
      insetLabelId={insetLabelId}
      prefix={prefix}
      size={size}
      autoFocus={autofocus}
      showClear={showClear}
      onEnterPress={onEnterPress}
      onChange={onChange}
      onClear={onClear}
      onBlur={onBlur}
      onFocus={onFocus}
      inputStyle={inputStyle}
      validateStatus={validateStatus}
    />
  );
};

export default renderSafeDatePickerTrigger;
