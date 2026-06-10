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
  generateDashboardMockQuotaData,
  getDashboardTimeRange,
  getDefaultTime,
  shouldUseDashboardMockCharts,
} from '../../helpers/dashboard';
import {
  TIME_OPTIONS,
  TIME_RANGE_OPTIONS,
} from '../../constants/dashboard.constants';
import { useIsMobile } from '../common/useIsMobile';
import { useMinimumLoadingTime } from '../common/useMinimumLoadingTime';

export const useDashboardData = (userState, userDispatch, statusState) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const isMobile = useIsMobile();
  const initialized = useRef(false);

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
  const [adminUsageSummary, setAdminUsageSummary] = useState({
    activeUsers: 0,
  });
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
      localStorage.setItem('data_export_default_time', value);
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
  }, []);

  const showSearchModal = useCallback(() => {
    setSearchModalVisible(true);
  }, []);

  const handleCloseModal = useCallback(() => {
    setSearchModalVisible(false);
  }, []);

  // ========== API 调用函数 ==========
  const loadQuotaData = useCallback(
    async (inputOverrides = {}) => {
      setLoading(true);
      try {
        let url = '';
        const effectiveInputs = { ...inputs, ...inputOverrides };
        const { username } = effectiveInputs;
        let { start_timestamp, end_timestamp } = effectiveInputs;
        if (activeTimeRange !== 'custom') {
          const range = getDashboardTimeRange(activeTimeRange);
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

        if (isAdminUser) {
          url = `/api/data/?username=${encodeURIComponent(username || '')}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}&default_time=${dataExportDefaultTime}`;
        } else {
          url = `/api/data/self/?start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}&default_time=${dataExportDefaultTime}`;
        }

        const res = await API.get(url);
        const { success, message, data } = res.data;
        if (success) {
          let normalizedData = [...(data || [])];
          if (normalizedData.length === 0 && shouldUseDashboardMockCharts()) {
            normalizedData = generateDashboardMockQuotaData(
              localStartTimestamp,
              localEndTimestamp,
              dataExportDefaultTime,
            );
          }
          setQuotaData(normalizedData);
          if (normalizedData.length === 0) {
            normalizedData.push({
              count: 0,
              model_name: '无数据',
              quota: 0,
              created_at: Date.now() / 1000,
            });
          }
          normalizedData.sort((a, b) => a.created_at - b.created_at);
          return normalizedData;
        } else {
          showError(message);
          return [];
        }
      } finally {
        setLoading(false);
      }
    },
    [inputs, activeTimeRange, dataExportDefaultTime, isAdminUser],
  );

  const clearAdminUserFilter = useCallback(async () => {
    if (!isAdminUser) return [];
    setInputs((inputs) => ({ ...inputs, username: '' }));
    const data = await loadQuotaData({ username: '' });
    setSearchModalVisible(false);
    return data;
  }, [isAdminUser, loadQuotaData]);

  const loadRecentTokenUsage = useCallback(async () => {
    if (isAdminUser) {
      setRecentTokens(0);
      return 0;
    }

    try {
      const nowTimestamp = Math.floor(Date.now() / 1000);
      const startTimestamp = nowTimestamp - 86400 * 30;
      const url = `/api/data/self/?start_timestamp=${startTimestamp}&end_timestamp=${nowTimestamp}&default_time=day`;
      const res = await API.get(url);
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
      setRecentTokens(0);
      return 0;
    }
  }, [isAdminUser]);

  const loadUptimeData = useCallback(async () => {
    setUptimeLoading(true);
    try {
      const res = await API.get('/api/uptime/status');
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
      setUptimeLoading(false);
    }
  }, [activeUptimeTab]);

  const loadUserQuotaData = useCallback(async () => {
    if (!isAdminUser) return [];
    try {
      let { start_timestamp, end_timestamp } = inputs;
      if (activeTimeRange !== 'custom') {
        const range = getDashboardTimeRange(activeTimeRange);
        start_timestamp = range.start_timestamp;
        end_timestamp = range.end_timestamp;
      }
      const localStartTimestamp = Date.parse(start_timestamp) / 1000;
      const localEndTimestamp = Date.parse(end_timestamp) / 1000;
      const url = `/api/data/users?start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}`;
      const res = await API.get(url);
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
      setAdminUsageSummary({ activeUsers: 0 });
      return [];
    }
  }, [inputs, activeTimeRange, isAdminUser]);

  const getUserData = useCallback(async () => {
    let res = await API.get(`/api/user/self`);
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  }, [userDispatch]);

  const refresh = useCallback(async () => {
    const data = await loadQuotaData();
    await loadRecentTokenUsage();
    await loadUptimeData();
    return data;
  }, [loadQuotaData, loadRecentTokenUsage, loadUptimeData]);

  const handleSearchConfirm = useCallback(
    async (updateChartDataCallback) => {
      const data = await refresh();
      if (data && data.length > 0 && updateChartDataCallback) {
        updateChartDataCallback(data);
      }
      setSearchModalVisible(false);
    },
    [refresh],
  );

  // ========== Effects ==========
  useEffect(() => {
    const timer = setTimeout(() => {
      setGreetingVisible(true);
    }, 100);
    return () => clearTimeout(timer);
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
    adminUsageSummary,
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
