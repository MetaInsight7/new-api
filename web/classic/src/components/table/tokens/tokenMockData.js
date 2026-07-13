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

const now = Math.floor(Date.now() / 1000);

export const tokenMockData = [
  {
    id: 980001,
    name: 'Claude Code 主力令牌',
    key: 'clau**********9k2m',
    status: 1,
    created_time: now - 86400 * 32,
    accessed_time: now - 92,
    expired_time: -1,
    remain_quota: 0,
    used_quota: 428000,
    unlimited_quota: true,
    group: 'auto',
    cross_group_retry: true,
    model_limits_enabled: true,
    model_limits: 'claude-sonnet-4-5,claude-opus-4-1,gpt-5',
    allow_ips: '114.93.10.21\n10.0.0.8',
  },
  {
    id: 980002,
    name: 'Cursor 日常开发',
    key: 'curs**********7f4p',
    status: 1,
    created_time: now - 86400 * 18,
    accessed_time: now - 738,
    expired_time: now + 86400 * 46,
    remain_quota: 760000,
    used_quota: 240000,
    unlimited_quota: false,
    group: 'pro',
    cross_group_retry: false,
    model_limits_enabled: true,
    model_limits: 'gpt-5,gpt-4.1',
    allow_ips: '',
  },
  {
    id: 980003,
    name: '课程实验 Key',
    key: 'stud**********2x8q',
    status: 2,
    created_time: now - 86400 * 12,
    accessed_time: now - 86400 * 3,
    expired_time: now + 86400 * 8,
    remain_quota: 185000,
    used_quota: 315000,
    unlimited_quota: false,
    group: 'default',
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '120.230.8.16',
  },
  {
    id: 980004,
    name: '旧版测试令牌',
    key: 'oldk**********6d1n',
    status: 3,
    created_time: now - 86400 * 90,
    accessed_time: now - 86400 * 35,
    expired_time: now - 86400,
    remain_quota: 80000,
    used_quota: 420000,
    unlimited_quota: false,
    group: 'default',
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
  },
  {
    id: 980005,
    name: '一次性批处理',
    key: 'batc**********3m5r',
    status: 4,
    created_time: now - 86400 * 5,
    accessed_time: 0,
    expired_time: -1,
    remain_quota: 0,
    used_quota: 200000,
    unlimited_quota: false,
    group: 'pro',
    cross_group_retry: false,
    model_limits_enabled: true,
    model_limits: 'gpt-5-mini',
    allow_ips: '',
  },
];
