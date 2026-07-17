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

import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { API, isAdmin, showError } from '../../helpers';
import {
  getDashboardComparisonRange,
  getDashboardTimeRange,
  getDefaultTime,
  summarizeDashboardQuotaData,
} from '../../helpers/dashboard';
import {
  TIME_OPTIONS,
  TIME_RANGE_OPTIONS,
} from '../../constants/dashboard.constants';
import { useIsMobile } from '../common/useIsMobile';
import { useMinimumLoadingTime } from '../common/useMinimumLoadingTime';
import { setStoredValue } from '../../helpers/siteStorage';

export const useDashboardData = (userState, userDispatch, statusState) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const isMobile = useIsMobile();
  const initialized = useRef(false);
  const quotaRequestRef = useRef(0);
  const recentTokensRequestRef = useRef(0);
  const uptimeRequestRef = useRef(0);
  const userQuotaRequestRef = useRef(0);
  const selectedAdminUserRequestRef = useRef(0);
  const mountedRef = useRef(true);

  // ========== 基础状态 ==========
  const [loading, setLoading] = useState(false);
  const [greetingVisible, setGreetingVisible] = useState(false);
  const [searchModalVisible, setSearchModalVisible] = useState(false);
  const showLoading = useMinimumLoadingTime(loading);
  const initialTimeRange = getDashboardTimeRange('today');
  const [activeTimeRange, setActiveTimeRange] = useState(initialTimeRange.key);

  // ========== 输入状态 ==========
  const [inputs, setInputs] = useState({
    username: '',
    token_name: '',
    model_name: '',
    start_timestamp: initialTimeRange.start_timestamp,
    end_timestamp: initialTimeRange.end_timestamp,
    channel: '',
    data_export_default_time: '',
  });

  const [dataExportDefaultTime, setDataExportDefaultTime] = useState(
    initialTimeRange.defaultTime || getDefaultTime(),
  );

  // ========== 数据状态 ==========
  const [quotaData, setQuotaData] = useState([]);
  const [consumeQuota, setConsumeQuota] = useState(0);
  const [consumeTokens, setConsumeTokens] = useState(0);
  const [times, setTimes] = useState(0);
  const [recentTokens, setRecentTokens] = useState(0);
  const [comparisonSummary, setComparisonSummary] = useState(null);
  const [adminUsageSummary, setAdminUsageSummary] = useState({
    activeUsers: 0,
  });
  const [selectedAdminUser, setSelectedAdminUser] = useState(null);
  const [pieData, setPieData] = useState([{ type: 'null', value: '0' }]);
  const [lineData, setLineData] = useState([]);
  const [modelColors, setModelColors] = useState({});

  // ========== 图表状态 ==========
  const [activeChartTab, setActiveChartTab] = useState('1');

  // ========== 趋势数据 ==========
  const [trendData, setTrendData] = useState({
    balance: [],
    usedQuota: [],
    requestCount: [],
    times: [],
    consumeQuota: [],
    tokens: [],
    rpm: [],
    tpm: [],
  });

  // ========== Uptime 数据 ==========
  const [uptimeData, setUptimeData] = useState([]);
  const [uptimeLoading, setUptimeLoading] = useState(false);
  const [activeUptimeTab, setActiveUptimeTab] = useState('');

  // ========== 常量 ==========
  const isAdminUser = isAdmin();

  // ========== Panel enable flags ==========
  const apiInfoEnabled = statusState?.status?.api_info_enabled ?? true;
  const announcementsEnabled =
    statusState?.status?.announcements_enabled ?? true;
  const faqEnabled = statusState?.status?.faq_enabled ?? true;
  const uptimeEnabled = statusState?.status?.uptime_kuma_enabled ?? true;

  const hasApiInfoPanel = apiInfoEnabled;
  const hasInfoPanels = announcementsEnabled || faqEnabled || uptimeEnabled;

  // ========== Memoized Values ==========
  const timeOptions = useMemo(
    () =>
      TIME_OPTIONS.map((option) => ({
        ...option,
        label: t(option.label),
      })),
    [t],
  );

  const timeRangeOptions = useMemo(
    () =>
      TIME_RANGE_OPTIONS.map((option) => ({
        ...option,
        label: t(option.label),
      })),
    [t],
  );

  const performanceMetrics = useMemo(() => {
    const { start_timestamp, end_timestamp } = inputs;
    const timeDiff =
      (Date.parse(end_timestamp) - Date.parse(start_timestamp)) / 60000;
    const avgRPM = isNaN(times / timeDiff)
      ? '0'
      : (times / timeDiff).toFixed(3);
    const avgTPM = isNaN(consumeTokens / timeDiff)
      ? '0'
      : (consumeTokens / timeDiff).toFixed(3);

    return { avgRPM, avgTPM, timeDiff };
  }, [times, consumeTokens, inputs.start_timestamp, inputs.end_timestamp]);

  const getGreeting = useMemo(() => {
    const hours = new Date().getHours();
    let greeting = '';

    if (hours >= 5 && hours < 12) {
      greeting = t('早上好');
    } else if (hours >= 12 && hours < 14) {
      greeting = t('中午好');
    } else if (hours >= 14 && hours < 18) {
      greeting = t('下午好');
    } else {
      greeting = t('晚上好');
    }

    const username = userState?.user?.username || '';
    return `👋${greeting}，${username}`;
  }, [t, userState?.user?.username]);

  // ========== 回调函数 ==========
  const handleInputChange = useCallback((value, name) => {
    if (name === 'data_export_default_time') {
      setDataExportDefaultTime(value);
      setStoredValue('data_export_default_time', value);
      return;
    }
    if (name === 'start_timestamp' || name === 'end_timestamp') {
      setActiveTimeRange('custom');
    }
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  }, []);

  const handleTimeRangeChange = useCallback((rangeKey) => {
    const range = getDashboardTimeRange(rangeKey);
    setActiveTimeRange(range.key);
    setDataExportDefaultTime(range.defaultTime);
    setInputs((inputs) => ({
      ...inputs,
      start_timestamp: range.start_timestamp,
      end_timestamp: range.end_timestamp,
    }));
    return range;
  }, []);

  const showSearchModal = useCallback(() => {
    setSearchModalVisible(true);
  }, []);

  const handleCloseModal = useCallback(() => {
    setSearchModalVisible(false);
  }, []);

  // ========== API 调用函数 ==========
  const loadQuotaData = useCallback(
    async (inputOverrides = {}, loadOptions = {}) => {
      const requestId = quotaRequestRef.current + 1;
      quotaRequestRef.current = requestId;
      const isLatestRequest = () => quotaRequestRef.current === requestId;

      setLoading(true);
      try {
        let url = '';
        const effectiveInputs = { ...inputs, ...inputOverrides };
        const { username } = effectiveInputs;
        let { start_timestamp, end_timestamp } = effectiveInputs;
        const effectiveTimeRange =
          loadOptions.rangeKey || activeTimeRange || 'today';
        const effectiveDefaultTime =
          loadOptions.defaultTime || dataExportDefaultTime;

        if (effectiveTimeRange !== 'custom') {
          const range = getDashboardTimeRange(effectiveTimeRange);
          start_timestamp = range.start_timestamp;
          end_timestamp = range.end_timestamp;
          setInputs((currentInputs) => ({
            ...currentInputs,
            start_timestamp,
            end_timestamp,
            ...(Object.prototype.hasOwnProperty.call(inputOverrides, 'username')
              ? { username: inputOverrides.username }
              : {}),
          }));
        }

        let localStartTimestamp = Date.parse(start_timestamp) / 1000;
        let localEndTimestamp = Date.parse(end_timestamp) / 1000;

        const buildQuotaUrl = (rangeStart, rangeEnd) => {
          if (isAdminUser) {
            return `/api/data/?username=${encodeURIComponent(username || '')}&start_timestamp=${rangeStart}&end_timestamp=${rangeEnd}&default_time=${effectiveDefaultTime}`;
          }

          return `/api/data/self/?start_timestamp=${rangeStart}&end_timestamp=${rangeEnd}&default_time=${effectiveDefaultTime}`;
        };

        const normalizeRows = (rows) => {
          const normalizedData = [...(rows || [])];
          normalizedData.sort((a, b) => a.created_at - b.created_at);
          return normalizedData;
        };

        url = buildQuotaUrl(localStartTimestamp, localEndTimestamp);

        const comparisonRange = getDashboardComparisonRange(
          effectiveTimeRange,
          localStartTimestamp,
          localEndTimestamp,
        );
        const comparisonRequest = comparisonRange
          ? API.get(
              buildQuotaUrl(
                comparisonRange.startTimestamp,
                comparisonRange.endTimestamp,
              ),
            ).catch((err) => {
              console.error(err);
              return null;
            })
          : Promise.resolve(null);

        const [res, comparisonRes] = await Promise.all([
          API.get(url),
          comparisonRequest,
        ]);
        if (!mountedRef.current || !isLatestRequest()) {
          return null;
        }

        const { success, message, data } = res.data;
        if (success) {
          const normalizedData = normalizeRows(data);

          const comparisonSuccess = comparisonRes?.data?.success;
          const comparisonRows = comparisonRange
            ? normalizeRows(comparisonSuccess ? comparisonRes.data.data : [])
            : [];

          setQuotaData(normalizedData);
          setComparisonSummary(summarizeDashboardQuotaData(comparisonRows));
          if (normalizedData.length === 0) {
            normalizedData.push({
              count: 0,
              model_name: '无数据',
              quota: 0,
              created_at: Date.now() / 1000,
            });
          }
          return normalizedData;
        } else {
          showError(message);
          setComparisonSummary(null);
          return [];
        }
      } catch (err) {
        console.error(err);
        if (!mountedRef.current || !isLatestRequest()) {
          return null;
        }
        showError(err?.message || t('请求失败'));
        setComparisonSummary(null);
        return [];
      } finally {
        if (mountedRef.current && isLatestRequest()) {
          setLoading(false);
        }
      }
    },
    [inputs, activeTimeRange, dataExportDefaultTime, isAdminUser],
  );

  const clearAdminUserFilter = useCallback(async () => {
    if (!isAdminUser) return [];
    setInputs((inputs) => ({ ...inputs, username: '' }));
    selectedAdminUserRequestRef.current += 1;
    setSelectedAdminUser(null);
    const data = await loadQuotaData({ username: '' });
    if (data === null) {
      return null;
    }
    setSearchModalVisible(false);
    return data;
  }, [isAdminUser, loadQuotaData]);

  const loadRecentTokenUsage = useCallback(async () => {
    const requestId = recentTokensRequestRef.current + 1;
    recentTokensRequestRef.current = requestId;
    const isLatestRequest = () => recentTokensRequestRef.current === requestId;

    if (isAdminUser) {
      if (mountedRef.current && isLatestRequest()) {
        setRecentTokens(0);
      }
      return 0;
    }

    try {
      const nowTimestamp = Math.floor(Date.now() / 1000);
      const startTimestamp = nowTimestamp - 86400 * 30;
      const url = `/api/data/self/?start_timestamp=${startTimestamp}&end_timestamp=${nowTimestamp}&default_time=day`;
      const res = await API.get(url);
      if (!mountedRef.current || !isLatestRequest()) {
        return null;
      }

      const { success, data } = res.data || {};
      if (!success) {
        setRecentTokens(0);
        return 0;
      }

      const totalTokens = (data || []).reduce(
        (total, item) => total + Number(item?.token_used || 0),
        0,
      );
      setRecentTokens(totalTokens);
      return totalTokens;
    } catch (err) {
      console.error(err);
      if (mountedRef.current && isLatestRequest()) {
        setRecentTokens(0);
      }
      return 0;
    }
  }, [isAdminUser]);

  const loadUptimeData = useCallback(async () => {
    const requestId = uptimeRequestRef.current + 1;
    uptimeRequestRef.current = requestId;
    const isLatestRequest = () => uptimeRequestRef.current === requestId;

    setUptimeLoading(true);
    try {
      const res = await API.get('/api/uptime/status');
      if (!mountedRef.current || !isLatestRequest()) {
        return null;
      }

      const { success, message, data } = res.data;
      if (success) {
        setUptimeData(data || []);
        if (data && data.length > 0 && !activeUptimeTab) {
          setActiveUptimeTab(data[0].categoryName);
        }
      } else {
        showError(message);
      }
    } catch (err) {
      console.error(err);
    } finally {
      if (mountedRef.current && isLatestRequest()) {
        setUptimeLoading(false);
      }
    }
  }, [activeUptimeTab]);

  const loadUserQuotaData = useCallback(
    async (loadOptions = {}) => {
      if (!isAdminUser) return [];
      const requestId = userQuotaRequestRef.current + 1;
      userQuotaRequestRef.current = requestId;
      const isLatestRequest = () => userQuotaRequestRef.current === requestId;

      try {
        let { start_timestamp, end_timestamp } = inputs;
        const effectiveTimeRange =
          loadOptions.rangeKey || activeTimeRange || 'today';
        if (effectiveTimeRange !== 'custom') {
          const range = getDashboardTimeRange(effectiveTimeRange);
          start_timestamp = range.start_timestamp;
          end_timestamp = range.end_timestamp;
        }
        const localStartTimestamp = Date.parse(start_timestamp) / 1000;
        const localEndTimestamp = Date.parse(end_timestamp) / 1000;
        const url = `/api/data/users?start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}`;
        const res = await API.get(url);
        if (!mountedRef.current || !isLatestRequest()) {
          return null;
        }

        const { success, message, data } = res.data;
        if (success) {
          const rows = data || [];
          const activeUsers = new Set(
            rows
              .filter(
                (item) =>
                  Number(item?.count || 0) > 0 ||
                  Number(item?.quota || 0) > 0 ||
                  Number(item?.token_used || 0) > 0,
              )
              .map((item) => item?.username)
              .filter(Boolean),
          ).size;
          setAdminUsageSummary({ activeUsers });
          return rows;
        } else {
          showError(message);
          setAdminUsageSummary({ activeUsers: 0 });
          return [];
        }
      } catch (err) {
        console.error(err);
        if (mountedRef.current && isLatestRequest()) {
          setAdminUsageSummary({ activeUsers: 0 });
        }
        return [];
      }
    },
    [inputs, activeTimeRange, isAdminUser],
  );

  const loadSelectedAdminUser = useCallback(
    async (username) => {
      if (!isAdminUser) return null;

      const trimmedUsername = String(username || '').trim();
      const requestId = selectedAdminUserRequestRef.current + 1;
      selectedAdminUserRequestRef.current = requestId;
      const isLatestRequest = () =>
        selectedAdminUserRequestRef.current === requestId;

      if (!trimmedUsername) {
        setSelectedAdminUser(null);
        return null;
      }

      try {
        const res = await API.get(
          `/api/user/search?keyword=${encodeURIComponent(trimmedUsername)}&group=&p=1&page_size=10`,
        );
        if (!mountedRef.current || !isLatestRequest()) {
          return null;
        }

        const { success, data } = res.data || {};
        if (!success) {
          setSelectedAdminUser(null);
          return null;
        }

        const users = data?.items || [];
        const exactUser = users.find(
          (item) => item?.username === trimmedUsername,
        );
        setSelectedAdminUser(exactUser || null);
        return exactUser || null;
      } catch (err) {
        console.error(err);
        if (mountedRef.current && isLatestRequest()) {
          setSelectedAdminUser(null);
        }
        return null;
      }
    },
    [isAdminUser],
  );

  const getUserData = useCallback(async () => {
    let res = await API.get(`/api/user/self`);
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  }, [userDispatch]);

  const refresh = useCallback(
    async (loadOptions = {}) => {
      const data = await loadQuotaData(
        loadOptions.inputOverrides || {},
        loadOptions,
      );
      if (data === null) {
        return null;
      }
      await loadRecentTokenUsage();
      await loadUptimeData();
      await loadSelectedAdminUser(
        loadOptions.inputOverrides?.username ?? inputs.username,
      );
      return data;
    },
    [
      inputs.username,
      loadQuotaData,
      loadRecentTokenUsage,
      loadUptimeData,
      loadSelectedAdminUser,
    ],
  );

  const handleSearchConfirm = useCallback(
    async (updateChartDataCallback) => {
      const data = await refresh();
      if (data === null) {
        return;
      }
      if (data && data.length > 0 && updateChartDataCallback) {
        updateChartDataCallback(data);
      }
      setSearchModalVisible(false);
    },
    [refresh],
  );

  // ========== Effects ==========
  useEffect(() => {
    mountedRef.current = true;
    const timer = setTimeout(() => {
      setGreetingVisible(true);
    }, 100);
    return () => {
      clearTimeout(timer);
      mountedRef.current = false;
      quotaRequestRef.current += 1;
      recentTokensRequestRef.current += 1;
      uptimeRequestRef.current += 1;
      userQuotaRequestRef.current += 1;
      selectedAdminUserRequestRef.current += 1;
    };
  }, []);

  useEffect(() => {
    if (!initialized.current) {
      getUserData();
      loadRecentTokenUsage();
      initialized.current = true;
    }
  }, [getUserData, loadRecentTokenUsage]);

  return {
    // 基础状态
    loading: showLoading,
    greetingVisible,
    searchModalVisible,

    // 输入状态
    inputs,
    activeTimeRange,
    dataExportDefaultTime,

    // 数据状态
    quotaData,
    consumeQuota,
    setConsumeQuota,
    consumeTokens,
    setConsumeTokens,
    times,
    setTimes,
    recentTokens,
    comparisonSummary,
    adminUsageSummary,
    selectedAdminUser,
    pieData,
    setPieData,
    lineData,
    setLineData,
    modelColors,
    setModelColors,

    // 图表状态
    activeChartTab,
    setActiveChartTab,

    // 趋势数据
    trendData,
    setTrendData,

    // Uptime 数据
    uptimeData,
    uptimeLoading,
    activeUptimeTab,
    setActiveUptimeTab,

    // 计算值
    timeOptions,
    timeRangeOptions,
    performanceMetrics,
    getGreeting,
    isAdminUser,
    hasApiInfoPanel,
    hasInfoPanels,
    apiInfoEnabled,
    announcementsEnabled,
    faqEnabled,
    uptimeEnabled,

    // 函数
    handleInputChange,
    handleTimeRangeChange,
    showSearchModal,
    handleCloseModal,
    loadQuotaData,
    clearAdminUserFilter,
    loadUserQuotaData,
    loadSelectedAdminUser,
    loadUptimeData,
    getUserData,
    refresh,
    handleSearchConfirm,

    // 导航和翻译
    navigate,
    t,
    isMobile,
  };
};
