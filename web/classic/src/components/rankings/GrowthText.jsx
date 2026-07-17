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

export default function GrowthText({ value, className = '' }) {
  if (!Number.isFinite(value) || value === 0) {
    return <span className={`font-mono text-xs ${className}`} style={{ color: 'var(--semi-color-text-2)' }}>0%</span>;
  }
  const isUp = value > 0;
  const color = isUp ? 'var(--semi-color-success)' : 'var(--semi-color-danger)';
  return (
    <span className={`font-mono text-xs ${className}`} style={{ color }}>
      {isUp ? '↑' : '↓'}
      {Math.abs(value).toFixed(Math.abs(value) >= 100 ? 0 : 1)}%
    </span>
  );
}
