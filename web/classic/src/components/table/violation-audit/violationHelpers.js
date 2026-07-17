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

// 违规审计前端共享工具：分类/分级/动作 → Semi Tag 颜色 + 标签，时间格式化。
// 颜色走 Semi 语义色（受控制台主题统一控制），不自造。

export const VIOLATION_CATEGORIES = [
  { value: 'politics', label: '涉政', color: 'red' },
  { value: 'porn', label: '涉黄', color: 'violet' },
  { value: 'abuse', label: '辱骂', color: 'amber' },
  { value: 'other', label: '其他', color: 'grey' },
];

export const VIOLATION_SEVERITIES = [
  { value: 'high', label: '高危', color: 'red' },
  { value: 'low', label: '低危', color: 'amber' },
];

export const VIOLATION_ACTIONS = [
  { value: 'blocked', label: '已拦截', color: 'red' },
  { value: 'allowed', label: '已放行', color: 'green' },
];

const cat = (v) => VIOLATION_CATEGORIES.find((c) => c.value === v);

export function categoryLabel(t, v) {
  const m = cat(v);
  return m ? t(m.label) : v || '-';
}
export function categoryColor(v) {
  const m = cat(v);
  return m ? m.color : 'grey';
}
export function severityTag(t, v) {
  const m = VIOLATION_SEVERITIES.find((s) => s.value === v);
  return { label: m ? t(m.label) : v, color: m ? m.color : 'grey' };
}
export function actionTag(t, v) {
  const m = VIOLATION_ACTIONS.find((a) => a.value === v);
  return { label: m ? t(m.label) : v, color: m ? m.color : 'grey' };
}

export function fmtTime(unixSec) {
  if (!unixSec) return '-';
  const d = new Date(unixSec * 1000);
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

export function fmtTimeSplit(unixSec) {
  if (!unixSec) return { time: '-', date: '' };
  const d = new Date(unixSec * 1000);
  const p = (n) => String(n).padStart(2, '0');
  return {
    time: `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`,
    date: `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`,
  };
}
