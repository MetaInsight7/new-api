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

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button, Card, Dropdown, Tooltip } from '@douyinfe/semi-ui';
import {
  BookOpen,
  ChevronDown,
  Copy,
  Eye,
  EyeOff,
  Globe2,
  KeyRound,
} from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import {
  API,
  copy,
  getStoredValue,
  showError,
  showSuccess,
} from '../../helpers';
import { fetchTokenKey } from '../../helpers/token';
import { useRequestLifecycle } from '../../hooks/common/useRequestLifecycle';

const normalizeApiKey = (key) => {
  if (!key) return '';
  return key.startsWith('sk-') ? key : `sk-${key}`;
};

const getTokenItems = (data) => {
  if (Array.isArray(data)) return data;
  if (Array.isArray(data?.items)) return data.items;
  return [];
};

const getServerAddress = (status) => {
  const localServerAddress = getStoredValue('server_address', '');

  if (status?.server_address) {
    return status.server_address;
  }

  if (localServerAddress) {
    return localServerAddress;
  }

  try {
    const localStatus = JSON.parse(getStoredValue('status', '{}'));
    if (localStatus?.server_address) {
      return localStatus.server_address;
    }
  } catch (e) {
    console.error('Failed to parse status from localStorage:', e);
  }

  return window.location.origin;
};

const getFallbackEndpoint = (status) => {
  const serverAddress = getServerAddress(status);
  return `${String(serverAddress).replace(/\/$/, '')}/v1`;
};

const getApiEndpoints = (status) => {
  const fallbackEndpoint = getFallbackEndpoint(status);
  const apiInfo = Array.isArray(status?.api_info) ? status.api_info : [];
  const endpoints = apiInfo
    .filter((item) => item?.url)
    .map((item, index) => ({
      id: item.id ?? `api-${index}`,
      url: item.url,
      route: item.route || item.description || `API #${index + 1}`,
      description: item.description || '',
    }));

  if (endpoints.length > 0) {
    return endpoints;
  }

  return [
    {
      id: 'default',
      url: fallbackEndpoint,
      route: '',
      description: '',
    },
  ];
};

const WorkspacePanel = ({ user, status, t, onStateChange }) => {
  const navigate = useNavigate();
  const [tokens, setTokens] = useState([]);
  const [selectedTokenId, setSelectedTokenId] = useState();
  const [selectedEndpointId, setSelectedEndpointId] = useState();
  const [visibleKeys, setVisibleKeys] = useState({});
  const [fullKeys, setFullKeys] = useState({});
  const [loading, setLoading] = useState(false);
  const [keyLoading, setKeyLoading] = useState(false);
  const [copyKeyLoading, setCopyKeyLoading] = useState(false);
  const [lineMenuOpen, setLineMenuOpen] = useState(false);
  const [tokenMenuOpen, setTokenMenuOpen] = useState(false);
  const linePickRef = useRef(null);
  const tokenPickRef = useRef(null);
  const fieldsRef = useRef(null);
  const [menuWidth, setMenuWidth] = useState(undefined);
  const { beginRequest, isCurrentRequest } = useRequestLifecycle();

  useEffect(() => {
    const el = fieldsRef.current;
    if (!el || typeof ResizeObserver === 'undefined') return undefined;
    // 菜单锚定在触发器(表头右侧,左右各 13px 内边距)的右缘。
    // 宽度取字段卡宽减去两侧 13px,则菜单左右各内缩 13px,居中对齐且不溢出。
    const update = () =>
      setMenuWidth(Math.max(0, el.getBoundingClientRect().width - 26));
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const menuStyle = menuWidth ? { width: `${menuWidth}px` } : undefined;

  const docsLink = status?.docs_link || getStoredValue('docs_link', '');
  const apiEndpoints = useMemo(() => getApiEndpoints(status), [status]);
  const selectedEndpoint = useMemo(
    () => apiEndpoints.find((endpoint) => endpoint.id === selectedEndpointId),
    [apiEndpoints, selectedEndpointId],
  );
  const apiBaseUrl = selectedEndpoint?.url || getFallbackEndpoint(status);
  const selectedToken = useMemo(
    () => tokens.find((token) => token.id === selectedTokenId),
    [selectedTokenId, tokens],
  );
  const selectedTokenKey = selectedToken ? selectedToken.key || '' : '';
  const selectedTokenMaskedKey = selectedTokenKey
    ? normalizeApiKey(selectedTokenKey)
    : '';
  const selectedTokenDisplayKey = selectedToken
    ? visibleKeys[selectedToken.id] || selectedTokenMaskedKey
    : '';
  const tokenOptions = useMemo(
    () =>
      tokens.map((token) => ({
        badge: token.group || t('默认分组'),
        description: normalizeApiKey(token.key || ''),
        label: token.name || `${t('令牌')} #${token.id}`,
        title: token.name || `${t('令牌')} #${token.id}`,
        value: token.id,
      })),
    [tokens, t],
  );
  const endpointOptions = useMemo(
    () =>
      apiEndpoints.map((endpoint) => ({
        badge: t('线路'),
        description: endpoint.url,
        label: endpoint.route || t('默认线路'),
        title: endpoint.route || t('默认线路'),
        value: endpoint.id,
      })),
    [apiEndpoints, t],
  );

  const loadTokens = useCallback(async () => {
    const requestId = beginRequest('tokens');
    setLoading(true);
    try {
      const res = await API.get('/api/token/?p=1&size=100');
      const { success, message, data } = res.data || {};
      if (!success) {
        if (isCurrentRequest('tokens', requestId)) {
          showError(message || t('获取令牌失败'));
        }
        return;
      }

      const activeTokens = getTokenItems(data).filter(
        (token) => token.status === 1,
      );
      if (!isCurrentRequest('tokens', requestId)) return;
      setTokens(activeTokens);
      setSelectedTokenId((currentId) => {
        if (activeTokens.some((token) => token.id === currentId)) {
          return currentId;
        }
        return activeTokens[0]?.id;
      });
    } catch (error) {
      if (isCurrentRequest('tokens', requestId)) {
        showError(error.message || t('获取令牌失败'));
      }
    } finally {
      if (isCurrentRequest('tokens', requestId)) setLoading(false);
    }
  }, [beginRequest, isCurrentRequest, t]);

  useEffect(() => {
    loadTokens();
  }, [loadTokens]);

  useEffect(() => {
    setSelectedEndpointId((currentId) => {
      if (apiEndpoints.some((endpoint) => endpoint.id === currentId)) {
        return currentId;
      }
      return apiEndpoints[0]?.id;
    });
  }, [apiEndpoints]);

  const handleCopy = async (text) => {
    if (!text) return;
    if (await copy(text)) {
      showSuccess(t('复制成功'));
    }
  };

  const getSelectedFullKey = async () => {
    if (!selectedToken) {
      showError(t('请先选择令牌'));
      return '';
    }

    if (fullKeys[selectedToken.id]) {
      return fullKeys[selectedToken.id];
    }

    const fullKey = normalizeApiKey(await fetchTokenKey(selectedToken.id));
    setFullKeys((keys) => ({ ...keys, [selectedToken.id]: fullKey }));
    return fullKey;
  };

  const handleShowKey = async () => {
    if (!selectedToken) {
      showError(t('请先选择令牌'));
      return;
    }

    if (visibleKeys[selectedToken.id]) {
      setVisibleKeys((keys) => {
        const nextKeys = { ...keys };
        delete nextKeys[selectedToken.id];
        return nextKeys;
      });
      return;
    }

    setKeyLoading(true);
    try {
      const fullKey = await getSelectedFullKey();
      if (!fullKey) return;
      setVisibleKeys((keys) => ({ ...keys, [selectedToken.id]: fullKey }));
    } catch (error) {
      showError(error.message || t('获取 API Key 失败'));
    } finally {
      setKeyLoading(false);
    }
  };

  const handleCopySelectedKey = async () => {
    setCopyKeyLoading(true);
    try {
      const fullKey = await getSelectedFullKey();
      if (!fullKey) return;
      await handleCopy(fullKey);
    } catch (error) {
      showError(error.message || t('获取 API Key 失败'));
    } finally {
      setCopyKeyLoading(false);
    }
  };

  const handleOpenDocs = () => {
    if (docsLink) {
      window.open(docsLink, '_blank', 'noopener,noreferrer');
      return;
    }
    navigate('/about');
  };

  const isKeyVisible = Boolean(selectedToken && visibleKeys[selectedToken.id]);
  const statusReady = tokens.length > 0;
  const currentLineLabel = selectedEndpoint?.route || t('默认线路');
  const currentTokenLabel = selectedToken
    ? selectedToken.name || `${t('令牌')} #${selectedToken.id}`
    : t('未选择令牌');
  const switchMeta = t('{{lineCount}} 条线路 · {{tokenCount}} 个令牌可切换', {
    lineCount: apiEndpoints.length,
    tokenCount: tokens.length,
  });

  useEffect(() => {
    onStateChange?.({
      ready: statusReady,
      lineCount: apiEndpoints.length,
      tokenCount: tokens.length,
    });
  }, [onStateChange, statusReady, apiEndpoints.length, tokens.length]);

  return (
    <Card
      className='dashboard-api-access-card !rounded-2xl'
      bodyStyle={{ padding: 0 }}
    >
      <div className='dashboard-api-access'>
        <div className='dashboard-api-access__fields' ref={fieldsRef}>
          {/* 线路字段:上排标签+可切换,发丝线,下排 Base URL + 复制 */}
          <div className='dashboard-api-access__field'>
            <div className='dashboard-api-access__field-head'>
              <span className='dashboard-api-access__field-label'>
                <Globe2 size={13} />
                {t('当前线路')}
              </span>
              <Dropdown
                trigger='custom'
                position='bottomRight'
                visible={lineMenuOpen}
                onClickOutSide={(e) => {
                  if (!linePickRef.current?.contains(e?.target)) {
                    setLineMenuOpen(false);
                  }
                }}
                render={
                  <Dropdown.Menu
                    className='dashboard-api-access__menu'
                    style={menuStyle}
                  >
                    {endpointOptions.map((option) => (
                      <Dropdown.Item
                        key={option.value}
                        className={
                          option.value === selectedEndpointId
                            ? 'dashboard-api-access__menu-li is-active'
                            : 'dashboard-api-access__menu-li'
                        }
                        onClick={() => {
                          setSelectedEndpointId(option.value);
                          setLineMenuOpen(false);
                        }}
                      >
                        <div className='dashboard-api-access__menu-item'>
                          <div className='dashboard-api-access__menu-main'>
                            <strong>{option.label}</strong>
                            {option.description && (
                              <small>{option.description}</small>
                            )}
                          </div>
                          {option.badge && (
                            <span className='dashboard-api-access__menu-badge'>
                              {option.badge}
                            </span>
                          )}
                        </div>
                      </Dropdown.Item>
                    ))}
                  </Dropdown.Menu>
                }
              >
                <button
                  type='button'
                  ref={linePickRef}
                  className='dashboard-api-access__field-pick'
                  aria-label={t('切换线路')}
                  onClick={() => setLineMenuOpen((open) => !open)}
                >
                  <strong>{currentLineLabel}</strong>
                  <ChevronDown size={15} />
                </button>
              </Dropdown>
            </div>
            <div className='dashboard-api-access__field-hair' />
            <div className='dashboard-api-access__field-value'>
              <Tooltip content={apiBaseUrl}>
                <code>{apiBaseUrl}</code>
              </Tooltip>
              <Tooltip content={t('复制 Base URL')}>
                <button
                  type='button'
                  className='dashboard-api-access__icon-btn'
                  aria-label={t('复制 Base URL')}
                  onClick={() => handleCopy(apiBaseUrl)}
                >
                  <Copy size={15} />
                </button>
              </Tooltip>
            </div>
          </div>

          {/* 令牌字段:上排标签+可切换,发丝线,下排 API Key + 显示/复制 */}
          <div className='dashboard-api-access__field'>
            <div className='dashboard-api-access__field-head'>
              <span className='dashboard-api-access__field-label'>
                <KeyRound size={13} />
                {t('当前令牌')}
              </span>
              <Dropdown
                trigger='custom'
                position='bottomRight'
                visible={tokenMenuOpen}
                onClickOutSide={(e) => {
                  if (!tokenPickRef.current?.contains(e?.target)) {
                    setTokenMenuOpen(false);
                  }
                }}
                render={
                  <Dropdown.Menu
                    className='dashboard-api-access__menu'
                    style={menuStyle}
                  >
                    {tokenOptions.length === 0 ? (
                      <Dropdown.Item disabled>
                        {t('暂无可用令牌')}
                      </Dropdown.Item>
                    ) : (
                      tokenOptions.map((option) => (
                        <Dropdown.Item
                          key={option.value}
                          className={
                            option.value === selectedTokenId
                              ? 'dashboard-api-access__menu-li is-active'
                              : 'dashboard-api-access__menu-li'
                          }
                          onClick={() => {
                            setSelectedTokenId(option.value);
                            setTokenMenuOpen(false);
                          }}
                        >
                          <div className='dashboard-api-access__menu-item'>
                            <div className='dashboard-api-access__menu-main'>
                              <strong>{option.label}</strong>
                              {option.description && (
                                <small>{option.description}</small>
                              )}
                            </div>
                            {option.badge && (
                              <span className='dashboard-api-access__menu-badge'>
                                {option.badge}
                              </span>
                            )}
                          </div>
                        </Dropdown.Item>
                      ))
                    )}
                  </Dropdown.Menu>
                }
              >
                <button
                  type='button'
                  ref={tokenPickRef}
                  className={`dashboard-api-access__field-pick${
                    statusReady ? '' : ' is-muted'
                  }`}
                  aria-label={t('切换令牌')}
                  onClick={() => setTokenMenuOpen((open) => !open)}
                >
                  <strong>{currentTokenLabel}</strong>
                  <ChevronDown size={15} />
                </button>
              </Dropdown>
            </div>
            <div className='dashboard-api-access__field-hair' />
            <div className='dashboard-api-access__field-value'>
              {statusReady ? (
                <>
                  <Tooltip content={selectedTokenDisplayKey}>
                    <code>{selectedTokenDisplayKey}</code>
                  </Tooltip>
                  <Tooltip
                    content={
                      isKeyVisible ? t('隐藏 API Key') : t('显示 API Key')
                    }
                  >
                    <button
                      type='button'
                      className='dashboard-api-access__icon-btn'
                      aria-label={
                        isKeyVisible ? t('隐藏 API Key') : t('显示 API Key')
                      }
                      disabled={keyLoading}
                      onClick={handleShowKey}
                    >
                      {isKeyVisible ? <EyeOff size={15} /> : <Eye size={15} />}
                    </button>
                  </Tooltip>
                  <Tooltip content={t('复制完整 API Key')}>
                    <button
                      type='button'
                      className='dashboard-api-access__icon-btn'
                      aria-label={t('复制完整 API Key')}
                      disabled={copyKeyLoading || !selectedToken}
                      onClick={handleCopySelectedKey}
                    >
                      <Copy size={15} />
                    </button>
                  </Tooltip>
                </>
              ) : (
                <code className='is-muted'>
                  {t('新建令牌后可在此复制 API Key')}
                </code>
              )}
            </div>
          </div>
        </div>

        {/* 底部双按钮:令牌(主蓝实心) / 文档(白底蓝描边) */}
        <div className='dashboard-api-access__actions'>
          <Button
            theme='solid'
            type='primary'
            icon={<KeyRound size={15} />}
            onClick={() => navigate('/console/token')}
          >
            {t('令牌')}
          </Button>
          <Button
            theme='outline'
            type='primary'
            icon={<BookOpen size={15} />}
            onClick={handleOpenDocs}
          >
            {t('文档')}
          </Button>
        </div>
      </div>
    </Card>
  );
};

export default WorkspacePanel;
