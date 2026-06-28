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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Card, Dropdown, Skeleton, Tooltip } from '@douyinfe/semi-ui';
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
import { API, copy, showError, showSuccess } from '../../helpers';
import { fetchTokenKey } from '../../helpers/token';

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
  const localServerAddress = localStorage.getItem('server_address');

  if (status?.server_address) {
    return status.server_address;
  }

  if (localServerAddress) {
    return localServerAddress;
  }

  try {
    const localStatus = JSON.parse(localStorage.getItem('status') || '{}');
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

const WorkspacePanel = ({ user, status, t }) => {
  const navigate = useNavigate();
  const [tokens, setTokens] = useState([]);
  const [selectedTokenId, setSelectedTokenId] = useState();
  const [selectedEndpointId, setSelectedEndpointId] = useState();
  const [visibleKeys, setVisibleKeys] = useState({});
  const [fullKeys, setFullKeys] = useState({});
  const [loading, setLoading] = useState(false);
  const [keyLoading, setKeyLoading] = useState(false);
  const [copyKeyLoading, setCopyKeyLoading] = useState(false);

  const docsLink = status?.docs_link || localStorage.getItem('docs_link') || '';
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
    setLoading(true);
    try {
      const res = await API.get('/api/token/?p=1&size=100');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('获取令牌失败'));
        return;
      }

      const activeTokens = getTokenItems(data).filter(
        (token) => token.status === 1,
      );
      setTokens(activeTokens);
      setSelectedTokenId((currentId) => {
        if (activeTokens.some((token) => token.id === currentId)) {
          return currentId;
        }
        return activeTokens[0]?.id;
      });
    } catch (error) {
      showError(error.message || t('获取令牌失败'));
    } finally {
      setLoading(false);
    }
  }, [t]);

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

  return (
    <Card
      className='dashboard-api-access-card !rounded-2xl'
      bodyStyle={{ padding: 0 }}
    >
      <div className='dashboard-api-access'>
        {/* 1. 概要汇总:可用标识 + 线路/令牌数量 + 当前线路/令牌 */}
        <div className='dashboard-api-access__summary'>
          <div className='dashboard-api-access__summary-top'>
            <span
              className={`dashboard-api-access__status is-${
                statusReady ? 'ready' : 'pending'
              }`}
            >
              <span className='dashboard-api-access__status-dot' />
              {statusReady ? t('已就绪') : t('待配置')}
            </span>
            <span className='dashboard-api-access__summary-meta'>
              {switchMeta}
            </span>
          </div>
          <div className='dashboard-api-access__summary-current'>
            <Dropdown
              trigger='click'
              position='bottomLeft'
              render={
                <Dropdown.Menu className='dashboard-api-access__menu'>
                  {endpointOptions.map((option) => (
                    <Dropdown.Item
                      key={option.value}
                      active={option.value === selectedEndpointId}
                      className={
                        option.value === selectedEndpointId
                          ? 'dashboard-api-access__menu-li is-active'
                          : 'dashboard-api-access__menu-li'
                      }
                      onClick={() => setSelectedEndpointId(option.value)}
                    >
                      <div className='dashboard-api-access__menu-item'>
                        <div className='dashboard-api-access__menu-main'>
                          <strong>{option.label}</strong>
                          {option.description && (
                            <small>{option.description}</small>
                          )}
                        </div>
                      </div>
                    </Dropdown.Item>
                  ))}
                </Dropdown.Menu>
              }
            >
              <button
                type='button'
                className='dashboard-api-access__switch is-line'
                aria-label={t('切换线路')}
              >
                <span className='dashboard-api-access__switch-label'>
                  <Globe2 size={12} />
                  {t('当前线路')}
                </span>
                <span className='dashboard-api-access__switch-value'>
                  <strong>{currentLineLabel}</strong>
                  <ChevronDown
                    size={15}
                    className='dashboard-api-access__switch-caret'
                  />
                </span>
              </button>
            </Dropdown>

            <Dropdown
              trigger='click'
              position='bottomLeft'
              render={
                <Dropdown.Menu className='dashboard-api-access__menu'>
                  {tokenOptions.length === 0 ? (
                    <Dropdown.Item disabled>
                      {t('暂无可用令牌')}
                    </Dropdown.Item>
                  ) : (
                    tokenOptions.map((option) => (
                      <Dropdown.Item
                        key={option.value}
                        active={option.value === selectedTokenId}
                        className={
                          option.value === selectedTokenId
                            ? 'dashboard-api-access__menu-li is-active'
                            : 'dashboard-api-access__menu-li'
                        }
                        onClick={() => setSelectedTokenId(option.value)}
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
                className='dashboard-api-access__switch is-token'
                aria-label={t('切换令牌')}
              >
                <span className='dashboard-api-access__switch-label'>
                  <KeyRound size={12} />
                  {t('当前令牌')}
                </span>
                <span className='dashboard-api-access__switch-value'>
                  <strong>{currentTokenLabel}</strong>
                  <ChevronDown
                    size={15}
                    className='dashboard-api-access__switch-caret'
                  />
                </span>
              </button>
            </Dropdown>
          </div>
        </div>

        {/* 2. 凭据子格子:Base URL / API Key 的值放入灰色单行代码块 */}
        <div className='dashboard-api-access__body'>
          <Skeleton
            active
            loading={loading && tokens.length === 0}
            placeholder={<Skeleton.Title style={{ width: '100%' }} />}
          >
            {statusReady ? (
              <div className='dashboard-api-access__creds'>
                <div className='dashboard-api-access__cred'>
                  <span className='dashboard-api-access__cred-label'>
                    {t('Base URL')}
                  </span>
                  <div className='dashboard-api-access__code'>
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

                <div className='dashboard-api-access__cred'>
                  <span className='dashboard-api-access__cred-label'>
                    {t('API Key')}
                  </span>
                  <div className='dashboard-api-access__code'>
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
                        {isKeyVisible ? (
                          <EyeOff size={15} />
                        ) : (
                          <Eye size={15} />
                        )}
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
                  </div>
                </div>
              </div>
            ) : (
              <div className='dashboard-api-access__empty'>
                <KeyRound size={18} />
                <span>{t('新建令牌后即可在此处选择并复制 API Key')}</span>
              </div>
            )}
          </Skeleton>
        </div>

        {/* 3. 操作按钮 */}
        <div className='dashboard-api-access__actions'>
          <Button
            theme='solid'
            type='primary'
            icon={<KeyRound size={15} />}
            onClick={() => navigate('/console/token')}
          >
            {t('管理令牌')}
          </Button>
          <Button
            theme='light'
            type='tertiary'
            icon={<BookOpen size={15} />}
            onClick={handleOpenDocs}
          >
            {t('使用文档')}
          </Button>
        </div>
      </div>
    </Card>
  );
};

export default WorkspacePanel;

