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

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { Button, Card, Skeleton, Tooltip } from '@douyinfe/semi-ui';
import {
  BookOpen,
  ChevronRight,
  Copy,
  Eye,
  EyeOff,
  Globe2,
  KeyRound,
  ShieldCheck,
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
  const [configOpen, setConfigOpen] = useState(false);
  const [visibleKeys, setVisibleKeys] = useState({});
  const [fullKeys, setFullKeys] = useState({});
  const [loading, setLoading] = useState(false);
  const [keyLoading, setKeyLoading] = useState(false);
  const [copyKeyLoading, setCopyKeyLoading] = useState(false);
  const configShellRef = useRef(null);

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
        badge: token.group || t('令牌'),
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
  const renderAccessChoices = useCallback(
    (options, value, onChange, emptyText) => {
      if (options.length === 0) {
        return (
          <div className='dashboard-api-access__selector-empty'>
            {emptyText}
          </div>
        );
      }

      return options.map((option) => {
        const isSelected = option.value === value;

        return (
          <button
            type='button'
            key={option.value}
            className={`dashboard-api-access__selector-option${
              isSelected ? ' is-selected' : ''
            }`}
            aria-pressed={isSelected}
            onClick={() => onChange(option.value)}
          >
            <span className='dashboard-api-access__selector-option-main'>
              <strong>{option.title || option.label}</strong>
              {option.description && <small>{option.description}</small>}
            </span>
            {option.badge && (
              <span className='dashboard-api-access__selector-option-badge'>
                {option.badge}
              </span>
            )}
          </button>
        );
      });
    },
    [],
  );
  const selectedConfigTitle = `${selectedEndpoint?.route || t('默认线路')} · ${
    selectedToken?.name || t('未选择令牌')
  }`;
  const selectedConfigMeta = t(
    '{{lineCount}} 条线路 · {{tokenCount}} 个令牌可切换',
    {
      lineCount: apiEndpoints.length,
      tokenCount: tokens.length,
    },
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

  useEffect(() => {
    if (!configOpen) return undefined;

    const handlePointerDown = (event) => {
      if (!configShellRef.current?.contains(event.target)) {
        setConfigOpen(false);
      }
    };
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') {
        setConfigOpen(false);
      }
    };

    document.addEventListener('pointerdown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('pointerdown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [configOpen]);

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
  const configContent = (
    <div className='dashboard-api-access__selector-panel'>
      <div className='dashboard-api-access__selector-section'>
        <div className='dashboard-api-access__selector-label'>
          <Globe2 size={14} />
          {t('线路')}
        </div>
        <div className='dashboard-api-access__selector-list'>
          {renderAccessChoices(
            endpointOptions,
            selectedEndpoint?.id,
            setSelectedEndpointId,
            t('暂无数据'),
          )}
        </div>
      </div>
      <div className='dashboard-api-access__selector-section'>
        <div className='dashboard-api-access__selector-label'>
          <KeyRound size={14} />
          {t('令牌')}
        </div>
        <div className='dashboard-api-access__selector-list'>
          {loading && tokenOptions.length === 0 ? (
            <div className='dashboard-api-access__selector-empty'>
              {t('加载中')}
            </div>
          ) : (
            renderAccessChoices(
              tokenOptions,
              selectedTokenId,
              setSelectedTokenId,
              t('暂无数据'),
            )
          )}
        </div>
      </div>
    </div>
  );

  return (
    <Card
      className='dashboard-api-access-card !rounded-2xl'
      bodyStyle={{ padding: 0 }}
    >
      <div className='dashboard-api-access'>
        <div className='dashboard-api-access__header'>
          <div>
            <div className='dashboard-api-access__title'>{t('API 接入')}</div>
            <div className='dashboard-api-access__note'>
              {t('当前可用接入信息')}
            </div>
          </div>
          <div className='dashboard-api-access__ready'>
            <span />
            {tokens.length > 0 ? t('已就绪') : t('待配置')}
          </div>
        </div>

        <div
          className='dashboard-api-access__config-shell'
          ref={configShellRef}
        >
          <button
            type='button'
            className='dashboard-api-access__config'
            aria-expanded={configOpen}
            onClick={() => setConfigOpen((open) => !open)}
          >
            <span className='dashboard-api-access__config-icon'>
              <Globe2 size={16} />
            </span>
            <span className='dashboard-api-access__config-copy'>
              <span>{t('当前接入')}</span>
              <strong>{selectedConfigTitle}</strong>
              <small>{selectedConfigMeta}</small>
            </span>
            <ChevronRight size={17} className='dashboard-api-access__chevron' />
          </button>
          {configOpen && (
            <div className='dashboard-api-access__selector-popover'>
              {configContent}
            </div>
          )}
        </div>

        <Skeleton
          active
          loading={loading && tokens.length === 0}
          placeholder={<Skeleton.Title style={{ width: '100%' }} />}
        >
          {tokens.length > 0 ? (
            <>
              <div className='dashboard-api-access__credentials'>
                <div className='dashboard-api-access__credential'>
                  <div className='dashboard-api-access__credential-head'>
                    <span>{t('Base URL')}</span>
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
                  <Tooltip content={apiBaseUrl}>
                    <code>{apiBaseUrl}</code>
                  </Tooltip>
                </div>

                <div className='dashboard-api-access__credential'>
                  <div className='dashboard-api-access__credential-head'>
                    <span>{t('API Key')}</span>
                    <div className='dashboard-api-access__credential-actions'>
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
                  <Tooltip content={selectedTokenDisplayKey}>
                    <code>{selectedTokenDisplayKey}</code>
                  </Tooltip>
                </div>
              </div>

              <div className='dashboard-api-access__meta-line'>
                {t('OpenAI 兼容 · Bearer Token · 脱敏展示不影响复制完整令牌')}
              </div>
            </>
          ) : (
            <div className='dashboard-api-access__empty'>
              <KeyRound size={24} />
              <strong>{t('暂无可用令牌')}</strong>
              <span>{t('新建令牌后即可在此处选择并复制 API Key')}</span>
            </div>
          )}
        </Skeleton>

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

        {user?.username && (
          <div className='dashboard-api-access__account'>
            <ShieldCheck size={13} />
            {t('当前账号')} · {user.username}
          </div>
        )}
      </div>
    </Card>
  );
};

export default WorkspacePanel;

