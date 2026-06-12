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

export const formatDashboardTokenMetric = (value) => {
  const number = Number(value || 0);
  if (!Number.isFinite(number)) return '0';

  const absNumber = Math.abs(number);
  if (absNumber >= 1_000_000_000) {
    return `${(number / 1_000_000_000).toFixed(2)}B`;
  }

  if (absNumber >= 1_000_000) {
    return `${(number / 1_000_000).toFixed(2)}M`;
  }

  return number.toLocaleString();
};

