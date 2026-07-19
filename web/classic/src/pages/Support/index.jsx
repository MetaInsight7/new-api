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

import React, { useEffect, useRef, useState } from 'react';
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
import { API, isAdmin, showError, showSuccess } from '../../helpers';
import {
  getSupportLabel,
  supportDepartments,
  supportPriorities,
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
  priority: 'medium',
  content: '',
});

const formatTime = (ts) => {
  if (!ts) return '-';
  const d = new Date(ts * 1000);
  const pad = (v) => String(v).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

const SupportStatus = ({ status }) => {
  const meta = supportStatusMeta[status] || supportStatusMeta.closed;
  return (
    <Tag color={meta.tone} className='support-status-tag'>
      {meta.label}
    </Tag>
  );
};

const TicketImage = ({ imageId, filename }) => {
  const [src, setSrc] = useState(null);
  useEffect(() => {
    let revoked = false;
    API.get(`/api/ticket/image/${imageId}`, { responseType: 'blob' })
      .then((res) => {
        if (!revoked) {
          setSrc(URL.createObjectURL(res.data));
        }
      })
      .catch(() => {});
    return () => {
      revoked = true;
      if (src) URL.revokeObjectURL(src);
    };
  }, [imageId]);
  if (!src) return null;
  return <img src={src} alt={filename} className='support-message-img' />;
};

const Support = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const [tickets, setTickets] = useState([]);
  const [loading, setLoading] = useState(false);
  const [query, setQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [createVisible, setCreateVisible] = useState(false);
  const [form, setForm] = useState(createInitialForm);
  const [reply, setReply] = useState('');
  const [formError, setFormError] = useState('');
  const [selectedTicket, setSelectedTicket] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [createFiles, setCreateFiles] = useState([]); // Files for create modal
  const [replyFiles, setReplyFiles] = useState([]); // Files for reply panel
  const [failedBatches, setFailedBatches] = useState([]); // [{file, messageId, ticketId, expired, noRetry}]
  const [uploading, setUploading] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const [total, setTotal] = useState(0);
  const pageSize = 20;
  const conversationRef = useRef(null);
  const searchTimer = useRef(null);
  const admin = isAdmin();

  const selectedId = searchParams.get('ticket');

  const loadTickets = async (page = 1, status = statusFilter, kw = query) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        p: String(page),
        page_size: String(pageSize),
      });
      if (status && status !== 'all') params.set('status', status);
      if (kw.trim()) params.set('keyword', kw.trim());
      const base = admin ? '/api/ticket/admin/' : '/api/ticket/';
      const res = await API.get(`${base}?${params.toString()}`);
      const { success, data, message } = res.data;
      if (success) {
        setTickets(data?.items || data || []);
        setTotal(data?.total || 0);
        setCurrentPage(page);
      } else {
        showError(message || '加载工单失败');
      }
    } catch (e) {
      showError(e.message || '加载工单失败');
    } finally {
      setLoading(false);
    }
  };

  const loadTicketDetail = async (id) => {
    try {
      const endpoint = admin ? `/api/ticket/admin/${id}` : `/api/ticket/${id}`;
      const res = await API.get(endpoint);
      const { success, data, message } = res.data;
      if (success) {
        setSelectedTicket(data);
      } else {
        showError(message || '加载工单详情失败');
      }
    } catch (e) {
      showError(e.message || '加载工单详情失败');
    }
  };

  useEffect(() => {
    loadTickets();
  }, []);

  useEffect(() => {
    if (selectedId) {
      loadTicketDetail(selectedId);
    } else {
      setSelectedTicket(null);
    }
  }, [selectedId]);

  useEffect(() => {
    if (!selectedTicket || !conversationRef.current) return;
    conversationRef.current.scrollTop = conversationRef.current.scrollHeight;
  }, [selectedTicket?.messages?.length]);

  const visibleTickets = tickets;

  const updateForm = (key, value) => {
    setForm((current) => ({ ...current, [key]: value }));
    if (formError) setFormError('');
  };

  const openTicket = (ticket) => {
    setSearchParams({ ticket: String(ticket.id) });
  };

  const closeDetail = () => {
    setReply('');
    setReplyFiles([]);
    setSelectedTicket(null);
    setSearchParams({});
    loadTickets();
  };

  const allowedMimes = ['image/jpeg', 'image/png', 'image/webp'];

  const uploadOneImage = async (ticketId, messageId, file) => {
    const formData = new FormData();
    formData.append('file', file);
    const res = await API.post(
      `/api/ticket/${ticketId}/messages/${messageId}/images`,
      formData,
      { headers: { 'Content-Type': 'multipart/form-data' } },
    );
    return res.data;
  };

  const uploadFilesSerial = async (ticketId, messageId, files) => {
    let failed = 0;
    for (const file of files) {
      try {
        const res = await uploadOneImage(ticketId, messageId, file);
        if (!res.success) {
          failed++;
          const expired = res.message?.includes('过期');
          setFailedBatches((prev) => [
            ...prev,
            { file, messageId, ticketId, expired },
          ]);
          if (res.message) showError(res.message);
        }
      } catch {
        failed++;
        setFailedBatches((prev) => [
          ...prev,
          { file, messageId, ticketId, expired: false },
        ]);
      }
    }
    return failed;
  };

  const validateFiles = (files, currentCount) => {
    const valid = [];
    for (const f of files) {
      if (f.size > 5 * 1024 * 1024) {
        showError(`${f.name} 超过 5MB`);
        continue;
      }
      if (!allowedMimes.includes(f.type)) {
        showError(`${f.name} 格式不支持，仅 JPG/PNG/WebP`);
        continue;
      }
      valid.push(f);
    }
    if (currentCount + valid.length > 3) {
      showError('最多选择 3 张图片');
      return [];
    }
    return valid;
  };

  const handleCreateFileSelect = (e) => {
    const files = Array.from(e.target.files || []);
    const valid = validateFiles(files, createFiles.length);
    if (valid.length > 0) setCreateFiles((prev) => [...prev, ...valid]);
    e.target.value = '';
  };

  const handleReplyFileSelect = (e) => {
    const files = Array.from(e.target.files || []);
    const valid = validateFiles(files, replyFiles.length);
    if (valid.length > 0) setReplyFiles((prev) => [...prev, ...valid]);
    e.target.value = '';
  };

  const currentTicketFailed = failedBatches.filter(
    (b) =>
      selectedTicket &&
      b.ticketId === selectedTicket.id &&
      !b.expired &&
      !b.noRetry,
  );
  const currentTicketNoRetry = failedBatches.filter(
    (b) =>
      selectedTicket &&
      b.ticketId === selectedTicket.id &&
      (b.expired || b.noRetry),
  );

  const retryFailedFiles = async () => {
    if (currentTicketFailed.length === 0 || uploading) return;
    setUploading(true);
    const toRetry = [...currentTicketFailed];
    // Remove these from failedBatches
    setFailedBatches((prev) =>
      prev.filter(
        (b) =>
          !(
            selectedTicket &&
            b.ticketId === selectedTicket.id &&
            !b.expired &&
            !b.noRetry
          ),
      ),
    );
    for (const item of toRetry) {
      try {
        const res = await uploadOneImage(
          item.ticketId,
          item.messageId,
          item.file,
        );
        if (!res.success) {
          const expired = res.message?.includes('过期');
          setFailedBatches((prev) => [...prev, { ...item, expired }]);
          if (res.message) showError(res.message);
        }
      } catch {
        setFailedBatches((prev) => [...prev, item]);
      }
    }
    setUploading(false);
    if (selectedTicket) await loadTicketDetail(selectedTicket.id);
  };

  const discardFailed = () => {
    setFailedBatches((prev) =>
      prev.filter((b) => !(selectedTicket && b.ticketId === selectedTicket.id)),
    );
  };

  const submitTicket = async () => {
    if (!form.subject.trim() || !form.content.trim()) {
      setFormError('请填写工单主题和问题描述');
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post('/api/ticket/', {
        subject: form.subject.trim(),
        department: form.department,
        priority: form.priority,
        content: form.content.trim(),
      });
      const { success, data, message } = res.data;
      if (!success) {
        setFormError(message || '创建失败');
        return;
      }
      const firstMsgId = data.messages?.[0]?.id;
      const filesToUpload = [...createFiles];
      setForm(createInitialForm());
      setCreateFiles([]);
      setCreateVisible(false);
      if (filesToUpload.length > 0) {
        if (!firstMsgId) {
          showSuccess('工单已创建，但无法获取消息 ID，附件未上传');
          for (const file of filesToUpload) {
            setFailedBatches((prev) => [
              ...prev,
              { file, messageId: 0, ticketId: data.id, noRetry: true },
            ]);
          }
        } else {
          const failed = await uploadFilesSerial(
            data.id,
            firstMsgId,
            filesToUpload,
          );
          if (failed > 0) {
            showSuccess(`工单已创建，${failed} 张图片上传失败`);
          } else {
            showSuccess('工单已创建');
          }
        }
      } else {
        showSuccess('工单已创建');
      }
      await loadTickets();
      setSearchParams({ ticket: String(data.id) });
    } catch (e) {
      setFormError(e.message || '创建失败');
    } finally {
      setSubmitting(false);
    }
  };

  const sendReply = async () => {
    if (!selectedTicket || !reply.trim()) return;
    setSubmitting(true);
    try {
      const endpoint = admin
        ? `/api/ticket/admin/${selectedTicket.id}/reply`
        : `/api/ticket/${selectedTicket.id}/reply`;
      const res = await API.post(endpoint, { content: reply.trim() });
      const { success, data, message } = res.data;
      if (!success) {
        showError(message || '回复失败');
        return;
      }
      const msgId = data?.id;
      const filesToUpload = [...replyFiles];
      setReply('');
      setReplyFiles([]);
      if (filesToUpload.length > 0) {
        if (!msgId) {
          showSuccess('回复已发送，但无法获取消息 ID，附件未上传');
          for (const file of filesToUpload) {
            setFailedBatches((prev) => [
              ...prev,
              {
                file,
                messageId: 0,
                ticketId: selectedTicket.id,
                noRetry: true,
              },
            ]);
          }
        } else {
          const failed = await uploadFilesSerial(
            selectedTicket.id,
            msgId,
            filesToUpload,
          );
          if (failed > 0) {
            showSuccess(`回复已发送，${failed} 张图片上传失败`);
          } else {
            showSuccess('回复已发送');
          }
        }
      } else {
        showSuccess('回复已发送');
      }
      await loadTicketDetail(selectedTicket.id);
      await loadTickets();
    } catch (e) {
      showError(e.message || '回复失败');
    } finally {
      setSubmitting(false);
    }
  };

  const closeTicket = async () => {
    if (!selectedTicket) return;
    try {
      const endpoint = admin
        ? `/api/ticket/admin/${selectedTicket.id}/status`
        : `/api/ticket/${selectedTicket.id}/close`;
      const res = admin
        ? await API.put(endpoint, { status: 'closed' })
        : await API.put(endpoint);
      const { success, message } = res.data;
      if (success) {
        showSuccess('工单已关闭');
        await loadTicketDetail(selectedTicket.id);
        await loadTickets();
      } else {
        showError(message || '关闭失败');
      }
    } catch (e) {
      showError(e.message || '关闭失败');
    }
  };

  const columns = [
    {
      title: '状态',
      dataIndex: 'status',
      width: 126,
      render: (status) => <SupportStatus status={status} />,
    },
    ...(admin
      ? [
          {
            title: '用户',
            dataIndex: 'username',
            width: 120,
            render: (username) => (
              <span className='support-table-secondary'>{username}</span>
            ),
          },
        ]
      : []),
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
          <span className='support-ticket-id'>{record.ticket_no}</span>
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
      dataIndex: 'updated_at',
      width: 168,
      render: (ts) => (
        <span className='support-table-time'>{formatTime(ts)}</span>
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
                <span>工单编号 {selectedTicket.ticket_no}</span>
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
                <span>创建于 {formatTime(selectedTicket.created_at)}</span>
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
            {(selectedTicket.messages || []).map((message) => (
              <article
                key={message.id}
                className={`support-message support-message--${message.sender}`}
              >
                <div className='support-message-meta'>
                  <span>
                    {message.sender === (admin ? 'support' : 'user')
                      ? '我'
                      : admin
                        ? message.author
                        : '技术支持'}
                  </span>
                  <span>{formatTime(message.created_at)}</span>
                </div>
                <div className='support-message-bubble'>
                  {message.content}
                  {message.images && message.images.length > 0 && (
                    <div className='support-message-images'>
                      {message.images.map((img) => (
                        <TicketImage
                          key={img.id}
                          imageId={img.id}
                          filename={img.filename}
                        />
                      ))}
                    </div>
                  )}
                </div>
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
                  <label
                    className='semi-button semi-button-borderless'
                    style={{
                      cursor: 'pointer',
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 4,
                    }}
                  >
                    <input
                      type='file'
                      accept='image/jpeg,image/png,image/webp'
                      multiple
                      style={{ display: 'none' }}
                      onChange={handleReplyFileSelect}
                      disabled={replyFiles.length >= 3 || submitting}
                    />
                    <Paperclip size={16} />
                    添加图片
                    {replyFiles.length > 0 ? ` (${replyFiles.length}/3)` : ''}
                  </label>
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
                  loading={submitting}
                  onClick={sendReply}
                >
                  发送回复
                </Button>
              </div>
              {replyFiles.length > 0 && (
                <div
                  className='support-upload-preview'
                  style={{ marginTop: 8 }}
                >
                  {replyFiles.map((file, i) => (
                    <span key={i} className='support-upload-file'>
                      {file.name}
                      <button
                        type='button'
                        onClick={() =>
                          setReplyFiles((prev) =>
                            prev.filter((_, idx) => idx !== i),
                          )
                        }
                      >
                        ×
                      </button>
                    </span>
                  ))}
                </div>
              )}
              {currentTicketFailed.length > 0 && (
                <div style={{ marginTop: 8, display: 'flex', gap: 8 }}>
                  <Button
                    size='small'
                    type='warning'
                    loading={uploading}
                    onClick={retryFailedFiles}
                  >
                    重试失败附件（{currentTicketFailed.length} 张）
                  </Button>
                  <Button
                    size='small'
                    theme='borderless'
                    onClick={discardFailed}
                  >
                    丢弃
                  </Button>
                </div>
              )}
              {currentTicketNoRetry.length > 0 && (
                <div
                  style={{
                    marginTop: 4,
                    fontSize: 12,
                    color: 'var(--semi-color-text-2)',
                  }}
                >
                  {currentTicketNoRetry.length}{' '}
                  张附件无法上传（窗口过期或响应异常），请通过新回复重新上传
                  <Button
                    size='small'
                    theme='borderless'
                    onClick={discardFailed}
                    style={{ marginLeft: 8 }}
                  >
                    清除
                  </Button>
                </div>
              )}
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
                onClick={() => {
                  setStatusFilter(option.value);
                  loadTickets(1, option.value);
                }}
              >
                {option.label}
                {option.value === 'all' && <span>{total}</span>}
              </button>
            ))}
          </div>
          <Input
            className='support-search'
            prefix={<Search size={16} />}
            value={query}
            onChange={(val) => {
              setQuery(val);
              if (searchTimer.current) clearTimeout(searchTimer.current);
              searchTimer.current = setTimeout(() => {
                loadTickets(1, statusFilter, val);
              }, 400);
            }}
            showClear
            placeholder='搜索主题或工单编号'
          />
        </div>

        <Table
          className='support-table dmit-flat-table'
          columns={columns}
          dataSource={visibleTickets}
          rowKey='id'
          loading={loading}
          pagination={
            total > pageSize
              ? {
                  currentPage,
                  pageSize,
                  total,
                  onPageChange: (page) => {
                    setCurrentPage(page);
                    loadTickets(page);
                  },
                }
              : false
          }
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
                    {formatTime(ticket.updated_at)}
                  </span>
                </span>
                <strong className='support-mobile-ticket-subject'>
                  {ticket.unread && <span className='support-unread-dot' />}
                  <span>{ticket.subject}</span>
                </strong>
                <span className='support-mobile-ticket-id'>
                  {ticket.ticket_no}
                </span>
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
          <span>共 {total} 个工单</span>
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
        confirmLoading={submitting}
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
            <label
              className='support-upload-placeholder'
              style={
                createFiles.length >= 3
                  ? { opacity: 0.5, pointerEvents: 'none' }
                  : {}
              }
            >
              <input
                type='file'
                accept='image/jpeg,image/png,image/webp'
                multiple
                style={{ display: 'none' }}
                onChange={handleCreateFileSelect}
              />
              <ImagePlus size={20} />
              {createFiles.length > 0
                ? `已选择 ${createFiles.length} 张`
                : '添加图片'}
            </label>
            {createFiles.length > 0 && (
              <div className='support-upload-preview'>
                {createFiles.map((file, i) => (
                  <span key={i} className='support-upload-file'>
                    {file.name}
                    <button
                      type='button'
                      onClick={() =>
                        setCreateFiles((prev) =>
                          prev.filter((_, idx) => idx !== i),
                        )
                      }
                    >
                      ×
                    </button>
                  </span>
                ))}
              </div>
            )}
          </div>
          {formError && <div className='support-form-error'>{formError}</div>}
        </div>
      </Modal>
    </div>
  );
};

export default Support;
