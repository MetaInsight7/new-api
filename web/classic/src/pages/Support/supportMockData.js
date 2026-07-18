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

export const SUPPORT_STORAGE_KEY = 'classic_support_tickets_v1';

export const supportDepartments = [
  { value: 'technical', label: '技术支持' },
  { value: 'billing', label: '计费与充值' },
  { value: 'account', label: '账户与安全' },
  { value: 'other', label: '其他问题' },
];

export const supportPriorities = [
  { value: 'high', label: '高' },
  { value: 'medium', label: '中' },
  { value: 'low', label: '低' },
];

export const supportServices = [
  { value: 'none', label: '无' },
  { value: 'api', label: 'API 调用服务' },
  { value: 'balance', label: '账户余额与充值' },
  { value: 'token', label: 'API 密钥' },
];

export const initialSupportTickets = [
  {
    id: 'TK-20260715001',
    subject: 'Claude Code 调用频繁出现 429',
    department: 'technical',
    service: 'api',
    priority: 'high',
    status: 'waiting_support',
    createdAt: '2026-07-15 09:18',
    updatedAt: '2026-07-15 10:42',
    unread: true,
    messages: [
      {
        id: 'm-1001',
        sender: 'user',
        author: '我',
        createdAt: '2026-07-15 09:18',
        content:
          '今天上午 Claude Code 连续出现 429，使用的是默认分组。请帮忙确认是上游限流还是 API 密钥配置问题。',
      },
      {
        id: 'm-1002',
        sender: 'support',
        author: '技术支持',
        createdAt: '2026-07-15 10:42',
        content:
          '您好，我们已经定位到对应请求记录，正在检查渠道限流情况。为避免重复计费，建议暂时不要连续重试，我们会尽快同步处理结果。',
      },
    ],
  },
  {
    id: 'TK-20260714006',
    subject: '充值完成后余额没有更新',
    department: 'billing',
    service: 'balance',
    priority: 'medium',
    status: 'waiting_user',
    createdAt: '2026-07-14 20:06',
    updatedAt: '2026-07-14 20:31',
    unread: false,
    messages: [
      {
        id: 'm-2001',
        sender: 'user',
        author: '我',
        createdAt: '2026-07-14 20:06',
        content: '已完成在线充值，但控制台余额暂时没有变化。',
      },
      {
        id: 'm-2002',
        sender: 'support',
        author: '客户支持',
        createdAt: '2026-07-14 20:31',
        content:
          '款项已经到账，我们需要您补充支付订单号后四位，以便核对对应充值记录。',
      },
    ],
  },
  {
    id: 'TK-20260712003',
    subject: '如何限制 API 密钥只能访问指定模型',
    department: 'technical',
    service: 'token',
    priority: 'low',
    status: 'resolved',
    createdAt: '2026-07-12 14:22',
    updatedAt: '2026-07-12 15:08',
    unread: false,
    messages: [
      {
        id: 'm-3001',
        sender: 'user',
        author: '我',
        createdAt: '2026-07-12 14:22',
        content: '想给课程演示用的 API 密钥只开放 gpt-5-mini，应该在哪里设置？',
      },
      {
        id: 'm-3002',
        sender: 'support',
        author: '技术支持',
        createdAt: '2026-07-12 14:48',
        content:
          '请进入 API 密钥页面编辑对应 API 密钥，打开“模型限制”后选择 gpt-5-mini 并保存即可。',
      },
      {
        id: 'm-3003',
        sender: 'user',
        author: '我',
        createdAt: '2026-07-12 15:08',
        content: '已经设置成功，谢谢。',
      },
    ],
  },
  {
    id: 'TK-20260710002',
    subject: '请求日志中的首字耗时是什么意思',
    department: 'other',
    service: 'api',
    priority: 'low',
    status: 'closed',
    createdAt: '2026-07-10 11:36',
    updatedAt: '2026-07-10 12:04',
    unread: false,
    messages: [
      {
        id: 'm-4001',
        sender: 'user',
        author: '我',
        createdAt: '2026-07-10 11:36',
        content: '日志里的总耗时和首字耗时分别代表什么？',
      },
      {
        id: 'm-4002',
        sender: 'support',
        author: '客户支持',
        createdAt: '2026-07-10 12:04',
        content:
          '首字耗时是流式请求从发起到收到第一个内容片段的时间，总耗时是完整请求结束所需时间。',
      },
    ],
  },
];

export const supportStatusMeta = {
  waiting_support: { label: '等待支持', tone: 'orange' },
  waiting_user: { label: '等待回复', tone: 'blue' },
  resolved: { label: '已解决', tone: 'green' },
  closed: { label: '已关闭', tone: 'grey' },
};

export const getSupportLabel = (options, value) =>
  options.find((item) => item.value === value)?.label || value || '-';
