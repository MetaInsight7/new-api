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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  Button,
  Input,
  Modal,
  Select,
  Table,
  Tag,
  TextArea,
} from '@douyinfe/semi-ui';
import {
  ArrowLeft,
  ArrowUpRight,
  BookOpen,
  CircleCheck,
  Clock3,
  FileQuestion,
  ImagePlus,
  LifeBuoy,
  MessageSquareText,
  Paperclip,
  Plus,
  Search,
  Send,
  XCircle,
} from 'lucide-react';
import { getStoredValue, setStoredValue, showSuccess } from '../../helpers';
import {
  SUPPORT_STORAGE_KEY,
  getSupportLabel,
  initialSupportTickets,
  supportDepartments,
  supportPriorities,
  supportServices,
  supportStatusMeta,
} from './supportMockData';
import './support.css';

const filterOptions = [
  { value: 'all', label: '全部' },
  { value: 'waiting_support', label: '等待支持' },
  { value: 'waiting_user', label: '等待回复' },
  { value: 'resolved', label: '已解决' },
  { value: 'closed', label: '已关闭' },
];

const createInitialForm = () => ({
  subject: '',
  department: 'technical',
  service: 'none',
  priority: 'medium',
  content: '',
});

const loadTickets = () => {
  try {
    const saved = getStoredValue(SUPPORT_STORAGE_KEY, '');
    if (saved) {
      const parsed = JSON.parse(saved);
      if (Array.isArray(parsed) && parsed.length > 0) return parsed;
    }
  } catch {
    // Mock 数据损坏时自动回退到初始数据。
  }
  return initialSupportTickets;
};

const SupportStatus = ({ status }) => {
  const meta = supportStatusMeta[status] || supportStatusMeta.closed;
  return (
    <Tag color={meta.tone} className='support-status-tag'>
      {meta.label}
    </Tag>
  );
};

const Support = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const [tickets, setTickets] = useState(loadTickets);
  const [query, setQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [createVisible, setCreateVisible] = useState(false);
  const [form, setForm] = useState(createInitialForm);
  const [reply, setReply] = useState('');
  const [formError, setFormError] = useState('');
  const conversationRef = useRef(null);

  const selectedId = searchParams.get('ticket');
  const selectedTicket = tickets.find((ticket) => ticket.id === selectedId);

  useEffect(() => {
    setStoredValue(SUPPORT_STORAGE_KEY, JSON.stringify(tickets));
  }, [tickets]);

  useEffect(() => {
    if (!selectedTicket || !conversationRef.current) return;
    conversationRef.current.scrollTop = conversationRef.current.scrollHeight;
  }, [selectedId, selectedTicket?.messages.length]);

  const visibleTickets = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return tickets.filter((ticket) => {
      const matchesStatus =
        statusFilter === 'all' || ticket.status === statusFilter;
      const matchesQuery =
        !normalized ||
        ticket.subject.toLowerCase().includes(normalized) ||
        ticket.id.toLowerCase().includes(normalized) ||
        getSupportLabel(supportDepartments, ticket.department)
          .toLowerCase()
          .includes(normalized);
      return matchesStatus && matchesQuery;
    });
  }, [query, statusFilter, tickets]);

  const updateForm = (key, value) => {
    setForm((current) => ({ ...current, [key]: value }));
    if (formError) setFormError('');
  };

  const openTicket = (ticket) => {
    setTickets((current) =>
      current.map((item) =>
        item.id === ticket.id ? { ...item, unread: false } : item,
      ),
    );
    setSearchParams({ ticket: ticket.id });
  };

  const closeDetail = () => {
    setReply('');
    setSearchParams({});
  };

  const submitTicket = () => {
    if (!form.subject.trim() || !form.content.trim()) {
      setFormError('请填写工单主题和问题描述');
      return;
    }

    const now = new Date();
    const pad = (value) => String(value).padStart(2, '0');
    const timestamp = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(
      now.getDate(),
    )} ${pad(now.getHours())}:${pad(now.getMinutes())}`;
    const id = `TK-${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(
      now.getDate(),
    )}${String(tickets.length + 1).padStart(3, '0')}`;
    const ticket = {
      id,
      subject: form.subject.trim(),
      department: form.department,
      service: form.service,
      priority: form.priority,
      status: 'waiting_support',
      createdAt: timestamp,
      updatedAt: timestamp,
      unread: false,
      messages: [
        {
          id: `m-${Date.now()}`,
          sender: 'user',
          author: '我',
          createdAt: timestamp,
          content: form.content.trim(),
        },
      ],
    };

    setTickets((current) => [ticket, ...current]);
    setForm(createInitialForm());
    setCreateVisible(false);
    showSuccess('工单已创建');
    setSearchParams({ ticket: ticket.id });
  };

  const sendReply = () => {
    if (!selectedTicket || !reply.trim()) return;
    const now = new Date();
    const timestamp = now
      .toLocaleString('zh-CN', {
        hour12: false,
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      })
      .replaceAll('/', '-');

    setTickets((current) =>
      current.map((ticket) =>
        ticket.id === selectedTicket.id
          ? {
              ...ticket,
              status: 'waiting_support',
              updatedAt: timestamp,
              messages: [
                ...ticket.messages,
                {
                  id: `m-${Date.now()}`,
                  sender: 'user',
                  author: '我',
                  createdAt: timestamp,
                  content: reply.trim(),
                },
              ],
            }
          : ticket,
      ),
    );
    setReply('');
    showSuccess('回复已发送');
  };

  const closeTicket = () => {
    if (!selectedTicket) return;
    setTickets((current) =>
      current.map((ticket) =>
        ticket.id === selectedTicket.id
          ? { ...ticket, status: 'closed', unread: false }
          : ticket,
      ),
    );
    showSuccess('工单已关闭');
  };

  const columns = [
    {
      title: '状态',
      dataIndex: 'status',
      width: 126,
      render: (status) => <SupportStatus status={status} />,
    },
    {
      title: '工单主题',
      dataIndex: 'subject',
      render: (subject, record) => (
        <button
          type='button'
          className='support-subject-button'
          onClick={() => openTicket(record)}
        >
          <span className='support-subject-line'>
            {record.unread && <span className='support-unread-dot' />}
            <span>{subject}</span>
          </span>
          <span className='support-ticket-id'>{record.id}</span>
        </button>
      ),
    },
    {
      title: '部门',
      dataIndex: 'department',
      width: 150,
      render: (department) => (
        <span className='support-table-secondary'>
          {getSupportLabel(supportDepartments, department)}
        </span>
      ),
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      width: 112,
      render: (priority) => (
        <span className={`support-priority support-priority--${priority}`}>
          {getSupportLabel(supportPriorities, priority)}
        </span>
      ),
    },
    {
      title: '最后更新',
      dataIndex: 'updatedAt',
      width: 168,
      render: (updatedAt) => (
        <span className='support-table-time'>{updatedAt}</span>
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 90,
      render: (_, record) => (
        <Button theme='borderless' onClick={() => openTicket(record)}>
          查看
        </Button>
      ),
    },
  ];

  if (selectedTicket) {
    const isClosed = ['closed', 'resolved'].includes(selectedTicket.status);
    return (
      <div className='support-page support-page--detail'>
        <button type='button' className='support-back' onClick={closeDetail}>
          <ArrowLeft size={17} />
          返回工单列表
        </button>

        <section className='support-detail-shell'>
          <header className='support-detail-header'>
            <div className='support-detail-heading'>
              <div className='support-detail-title-row'>
                <h1>{selectedTicket.subject}</h1>
                <SupportStatus status={selectedTicket.status} />
              </div>
              <div className='support-detail-meta'>
                <span>工单编号 {selectedTicket.id}</span>
                <span>
                  部门{' '}
                  {getSupportLabel(
                    supportDepartments,
                    selectedTicket.department,
                  )}
                </span>
                <span>
                  优先级{' '}
                  {getSupportLabel(supportPriorities, selectedTicket.priority)}
                </span>
                <span>创建于 {selectedTicket.createdAt}</span>
              </div>
            </div>
          </header>

          {isClosed && (
            <div className='support-closed-notice'>
              <CircleCheck size={18} />
              此工单已{selectedTicket.status === 'resolved' ? '解决' : '关闭'}
              。如有新问题，请另外创建工单。
            </div>
          )}

          <div className='support-conversation' ref={conversationRef}>
            {selectedTicket.messages.map((message) => (
              <article
                key={message.id}
                className={`support-message support-message--${message.sender}`}
              >
                <div className='support-message-meta'>
                  <span>{message.author}</span>
                  <span>{message.createdAt}</span>
                </div>
                <div className='support-message-bubble'>{message.content}</div>
              </article>
            ))}
          </div>

          {!isClosed && (
            <footer className='support-reply-panel'>
              <div className='support-reply-header'>
                <div>
                  <MessageSquareText size={17} />
                  <strong>回复工单</strong>
                </div>
                <span>回复后，工单将进入等待支持状态</span>
              </div>
              <div className='support-reply-field'>
                <TextArea
                  value={reply}
                  onChange={setReply}
                  autosize={{ minRows: 3, maxRows: 6 }}
                  maxCount={5000}
                  placeholder='补充问题信息或回复支持人员…'
                />
              </div>
              <div className='support-reply-actions'>
                <div className='support-reply-actions-left'>
                  <Button icon={<Paperclip size={16} />} theme='borderless'>
                    添加附件
                  </Button>
                  <Button
                    icon={<XCircle size={16} />}
                    theme='borderless'
                    type='danger'
                    onClick={closeTicket}
                  >
                    关闭工单
                  </Button>
                </div>
                <Button
                  type='primary'
                  icon={<Send size={16} />}
                  disabled={!reply.trim()}
                  onClick={sendReply}
                >
                  发送回复
                </Button>
              </div>
            </footer>
          )}
        </section>
      </div>
    );
  }

  return (
    <div className='support-page'>
      <header className='support-page-header'>
        <div>
          <h1>技术支持</h1>
          <p>建议先查阅帮助内容；仍未解决时，再提交工单联系我们。</p>
          <div className='support-page-service'>
            <LifeBuoy size={14} />
            <span>服务时间：工作日 09:00–22:00 · 紧急问题优先处理</span>
          </div>
        </div>
        <Button
          type='primary'
          icon={<Plus size={17} />}
          onClick={() => setCreateVisible(true)}
        >
          新建工单
        </Button>
      </header>

      <section className='support-resource-row' aria-label='帮助资源'>
        <a
          className='support-resource-card'
          href='https://docs.newapi.pro'
          target='_blank'
          rel='noreferrer'
        >
          <span className='support-resource-icon'>
            <FileQuestion size={26} />
          </span>
          <span className='support-resource-copy'>
            <strong>常见问题</strong>
            <small>先查看调用、API 密钥和计费相关解答</small>
          </span>
          <span className='support-resource-action' aria-hidden='true'>
            <ArrowUpRight size={17} />
          </span>
        </a>

        <a
          className='support-resource-card'
          href='https://docs.newapi.pro'
          target='_blank'
          rel='noreferrer'
        >
          <span className='support-resource-icon'>
            <BookOpen size={26} />
          </span>
          <span className='support-resource-copy'>
            <strong>帮助文档</strong>
            <small>查看接口说明、接入指南与示例</small>
          </span>
          <span className='support-resource-action' aria-hidden='true'>
            <ArrowUpRight size={17} />
          </span>
        </a>
      </section>

      <Button
        className='support-mobile-create'
        type='primary'
        icon={<Plus size={17} />}
        onClick={() => setCreateVisible(true)}
      >
        新建工单
      </Button>

      <section className='support-list-card'>
        <div className='support-list-toolbar'>
          <div className='support-filter-tabs'>
            {filterOptions.map((option) => (
              <button
                type='button'
                key={option.value}
                className={statusFilter === option.value ? 'is-active' : ''}
                onClick={() => setStatusFilter(option.value)}
              >
                {option.label}
                {option.value === 'all' && <span>{tickets.length}</span>}
              </button>
            ))}
          </div>
          <Input
            className='support-search'
            prefix={<Search size={16} />}
            value={query}
            onChange={setQuery}
            showClear
            placeholder='搜索主题或工单编号'
          />
        </div>

        <Table
          className='support-table dmit-flat-table'
          columns={columns}
          dataSource={visibleTickets}
          rowKey='id'
          pagination={false}
          empty={
            <div className='support-empty'>
              <MessageSquareText size={28} />
              <strong>没有匹配的工单</strong>
              <span>调整筛选条件，或创建一个新工单。</span>
            </div>
          }
        />

        <div className='support-mobile-list'>
          {visibleTickets.length > 0 ? (
            visibleTickets.map((ticket) => (
              <button
                type='button'
                className='support-mobile-ticket'
                key={ticket.id}
                onClick={() => openTicket(ticket)}
              >
                <span className='support-mobile-ticket-top'>
                  <SupportStatus status={ticket.status} />
                  <span className='support-mobile-ticket-time'>
                    {ticket.updatedAt}
                  </span>
                </span>
                <strong className='support-mobile-ticket-subject'>
                  {ticket.unread && <span className='support-unread-dot' />}
                  <span>{ticket.subject}</span>
                </strong>
                <span className='support-mobile-ticket-id'>{ticket.id}</span>
                <span className='support-mobile-ticket-meta'>
                  <span>
                    {getSupportLabel(supportDepartments, ticket.department)}
                  </span>
                  <span>
                    优先级 {getSupportLabel(supportPriorities, ticket.priority)}
                  </span>
                </span>
              </button>
            ))
          ) : (
            <div className='support-empty'>
              <MessageSquareText size={28} />
              <strong>没有匹配的工单</strong>
              <span>调整筛选条件，或创建一个新工单。</span>
            </div>
          )}
        </div>

        <div className='support-list-footer'>
          <span>共 {visibleTickets.length} 个工单</span>
          <span>
            <Clock3 size={14} />
            工单进度以最后更新时间为准
          </span>
        </div>
      </section>

      <Modal
        className='support-create-modal'
        title='新建工单'
        visible={createVisible}
        onCancel={() => {
          setCreateVisible(false);
          setFormError('');
        }}
        onOk={submitTicket}
        okText='提交工单'
        cancelText='取消'
        width={720}
        centered
      >
        <div className='support-create-intro'>
          <LifeBuoy size={18} />
          <span>
            请尽量提供请求时间、模型名称和报错信息，我们会更快定位问题。
          </span>
        </div>
        <div className='support-form'>
          <label className='support-form-field support-form-field--full'>
            <span>工单主题</span>
            <Input
              value={form.subject}
              onChange={(value) => updateForm('subject', value)}
              maxLength={120}
              placeholder='用一句话描述遇到的问题'
            />
          </label>
          <label className='support-form-field'>
            <span>问题部门</span>
            <Select
              value={form.department}
              optionList={supportDepartments}
              onChange={(value) => updateForm('department', value)}
            />
          </label>
          <label className='support-form-field'>
            <span>关联服务</span>
            <Select
              value={form.service}
              optionList={supportServices}
              onChange={(value) => updateForm('service', value)}
            />
          </label>
          <label className='support-form-field'>
            <span>优先级</span>
            <Select
              value={form.priority}
              optionList={supportPriorities}
              onChange={(value) => updateForm('priority', value)}
            />
          </label>
          <div className='support-form-hint'>
            高优先级仅用于服务不可用、持续报错等紧急问题。
          </div>
          <label className='support-form-field support-form-field--full'>
            <span>问题描述</span>
            <TextArea
              value={form.content}
              onChange={(value) => updateForm('content', value)}
              autosize={{ minRows: 5, maxRows: 9 }}
              maxCount={5000}
              placeholder='请描述问题现象、发生时间、复现步骤，以及相关模型或 API 密钥名称…'
            />
          </label>
          <div className='support-form-field support-form-field--full'>
            <span>附件（可选，最多 3 张）</span>
            <button type='button' className='support-upload-placeholder'>
              <ImagePlus size={20} />
              添加图片
            </button>
          </div>
          {formError && <div className='support-form-error'>{formError}</div>}
        </div>
      </Modal>
    </div>
  );
};

export default Support;
