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

import { useState, useEffect, useMemo, useContext, useRef } from 'react';
import { StatusContext } from '../../context/Status';
import { API } from '../../helpers/api';
import { getStoredJSON } from '../../helpers/siteStorage';

// 创建一个全局事件系统来同步所有useSidebar实例
const sidebarEventTarget = new EventTarget();
const SIDEBAR_REFRESH_EVENT = 'sidebar-refresh';

export const DEFAULT_ADMIN_CONFIG = {
  chat: {
    enabled: true,
    playground: true,
    chat: true,
  },
  console: {
    enabled: true,
    detail: true,
    token: true,
    log: true,
    midjourney: true,
    task: true,
  },
  personal: {
    enabled: true,
    topup: true,
    personal: true,
  },
  admin: {
    enabled: true,
    channel: true,
    models: true,
    deployment: true,
    redemption: true,
    user: true,
    subscription: true,
    setting: true,
  },
};

const deepClone = (value) => JSON.parse(JSON.stringify(value));

const isEnabledValue = (value) =>
  value === true || value === 'true' || value === 1 || value === '1';

const normalizeSection = (section, fallback = {}) => {
  const source = section && typeof section === 'object' ? section : {};
  return Object.keys({ ...fallback, ...source }).reduce((result, key) => {
    if (key === 'enabled') {
      result[key] =
        source[key] === undefined
          ? fallback[key] !== false
          : isEnabledValue(source[key]);
    } else if (source[key] !== undefined) {
      result[key] = isEnabledValue(source[key]);
    } else if (fallback[key] !== undefined) {
      result[key] = fallback[key];
    }
    return result;
  }, {});
};

export const buildDefaultUserConfig = (adminConfig) => {
  const defaultUserConfig = {};
  Object.entries(adminConfig || {}).forEach(([sectionKey, sectionConfig]) => {
    if (!sectionConfig?.enabled) return;

    defaultUserConfig[sectionKey] = { enabled: true };
    Object.entries(sectionConfig).forEach(([moduleKey, enabled]) => {
      if (moduleKey !== 'enabled' && enabled === true) {
        defaultUserConfig[sectionKey][moduleKey] = true;
      }
    });
  });
  return defaultUserConfig;
};

export const mergeAdminConfig = (savedConfig) => {
  const merged = deepClone(DEFAULT_ADMIN_CONFIG);
  if (!savedConfig || typeof savedConfig !== 'object') return merged;

  for (const [sectionKey, sectionConfig] of Object.entries(savedConfig)) {
    if (!sectionConfig || typeof sectionConfig !== 'object') continue;

    if (!merged[sectionKey]) {
      merged[sectionKey] = normalizeSection(sectionConfig, {});
      continue;
    }

    merged[sectionKey] = normalizeSection(sectionConfig, merged[sectionKey]);
  }

  return merged;
};

export const normalizeUserConfig = (savedConfig, adminConfig) => {
  if (!savedConfig || typeof savedConfig !== 'object') {
    return buildDefaultUserConfig(adminConfig);
  }

  const normalized = {};
  Object.entries(adminConfig || {}).forEach(([sectionKey, sectionConfig]) => {
    const source = savedConfig[sectionKey];
    normalized[sectionKey] = normalizeSection(
      source,
      buildDefaultUserConfig({ [sectionKey]: sectionConfig })[sectionKey],
    );
  });
  return normalized;
};

export const useSidebar = () => {
  const [statusState] = useContext(StatusContext);
  const [userConfig, setUserConfig] = useState(null);
  const [loading, setLoading] = useState(true);
  const instanceIdRef = useRef(null);
  const hasLoadedOnceRef = useRef(false);
  const requestSeqRef = useRef(0);

  if (!instanceIdRef.current) {
    const randomPart = Math.random().toString(16).slice(2);
    instanceIdRef.current = `sidebar-${Date.now()}-${randomPart}`;
  }

  // 获取管理员配置
  const [cachedStatus] = useState(() => getStoredJSON('status', {}));
  const effectiveStatus = useMemo(
    () =>
      statusState?.status && Object.keys(statusState.status).length > 0
        ? statusState.status
        : cachedStatus,
    [statusState?.status, cachedStatus],
  );
  const adminConfig = useMemo(() => {
    if (effectiveStatus?.SidebarModulesAdmin) {
      const rawConfig = effectiveStatus.SidebarModulesAdmin;
      if (typeof rawConfig === 'string') {
        try {
          return mergeAdminConfig(JSON.parse(rawConfig));
        } catch {
          return mergeAdminConfig(null);
        }
      }
      return mergeAdminConfig(rawConfig);
    }
    return mergeAdminConfig(null);
  }, [effectiveStatus?.SidebarModulesAdmin]);

  // 加载用户配置的通用方法
  const loadUserConfig = async ({ withLoading } = {}) => {
    const requestSeq = ++requestSeqRef.current;
    const shouldShowLoader =
      typeof withLoading === 'boolean'
        ? withLoading
        : !hasLoadedOnceRef.current;

    try {
      if (shouldShowLoader) {
        setLoading(true);
      }

      const res = await API.get('/api/user/self');
      if (res.data.success && res.data.data.sidebar_modules) {
        let config;
        // 检查sidebar_modules是字符串还是对象
        if (typeof res.data.data.sidebar_modules === 'string') {
          try {
            config = JSON.parse(res.data.data.sidebar_modules);
          } catch {
            config = null;
          }
        } else {
          config = res.data.data.sidebar_modules;
        }
        if (requestSeq === requestSeqRef.current) {
          // A malformed or legacy value must not leave the navigation empty.
          // Fall back to the admin-approved defaults and let explicit false
          // values in a valid user config continue to hide modules.
          setUserConfig(normalizeUserConfig(config, adminConfig));
        }
      } else {
        if (requestSeq === requestSeqRef.current) {
          setUserConfig(buildDefaultUserConfig(adminConfig));
        }
      }
    } catch (error) {
      if (requestSeq === requestSeqRef.current) {
        setUserConfig(buildDefaultUserConfig(adminConfig));
      }
    } finally {
      if (
        shouldShowLoader && requestSeq === requestSeqRef.current
      ) {
        setLoading(false);
      }
      hasLoadedOnceRef.current = true;
    }
  };

  // 刷新用户配置的方法（供外部调用）
  const refreshUserConfig = async () => {
    if (Object.keys(adminConfig).length > 0) {
      await loadUserConfig({ withLoading: false });
    }

    // 触发全局刷新事件，通知所有useSidebar实例更新
    sidebarEventTarget.dispatchEvent(
      new CustomEvent(SIDEBAR_REFRESH_EVENT, {
        detail: { sourceId: instanceIdRef.current, skipLoader: true },
      }),
    );
  };

  // 加载用户配置
  useEffect(() => {
    // 只有当管理员配置加载完成后才加载用户配置
    if (Object.keys(adminConfig).length > 0) {
      loadUserConfig();
    }
  }, [adminConfig]);

  useEffect(
    () => () => {
      requestSeqRef.current += 1;
    },
    [],
  );

  // 监听全局刷新事件
  useEffect(() => {
    const handleRefresh = (event) => {
      if (event?.detail?.sourceId === instanceIdRef.current) {
        return;
      }

      if (Object.keys(adminConfig).length > 0) {
        loadUserConfig({
          withLoading: event?.detail?.skipLoader ? false : undefined,
        });
      }
    };

    sidebarEventTarget.addEventListener(SIDEBAR_REFRESH_EVENT, handleRefresh);

    return () => {
      sidebarEventTarget.removeEventListener(
        SIDEBAR_REFRESH_EVENT,
        handleRefresh,
      );
    };
  }, [adminConfig]);

  // 计算最终的显示配置
  const finalConfig = useMemo(() => {
    const result = {};

    // 确保adminConfig已加载
    if (!adminConfig || Object.keys(adminConfig).length === 0) {
      return result;
    }

    // Render the admin-approved defaults while the personal preference request
    // is in flight. This avoids a misleading sidebar that briefly contains
    // only the always-available support link.
    const effectiveUserConfig =
      userConfig || buildDefaultUserConfig(adminConfig);

    // 遍历所有区域
    Object.keys(adminConfig).forEach((sectionKey) => {
      const adminSection = adminConfig[sectionKey];
      const userSection = effectiveUserConfig[sectionKey];

      // 如果管理员禁用了整个区域，则该区域不显示
      if (!adminSection?.enabled) {
        result[sectionKey] = { enabled: false };
        return;
      }

      // 区域级别：用户可以选择隐藏管理员允许的区域
      // 当userSection存在时检查enabled状态，否则默认为true
      const sectionEnabled = userSection
        ? userSection.enabled !== false && userSection.enabled !== 'false'
        : true;
      result[sectionKey] = { enabled: sectionEnabled };

      // 功能级别：只有管理员和用户都允许的功能才显示
      Object.keys(adminSection).forEach((moduleKey) => {
        if (moduleKey === 'enabled') return;

        const adminAllowed = adminSection[moduleKey];
        // 当userSection存在时检查模块状态，否则默认为true
        const userAllowed = userSection
          ? userSection[moduleKey] !== false &&
            userSection[moduleKey] !== 'false'
          : true;

        result[sectionKey][moduleKey] =
          adminAllowed && userAllowed && sectionEnabled;
      });
    });

    return result;
  }, [adminConfig, userConfig]);

  // 检查特定功能是否应该显示
  const isModuleVisible = (sectionKey, moduleKey = null) => {
    if (moduleKey) {
      return finalConfig[sectionKey]?.[moduleKey] === true;
    } else {
      return finalConfig[sectionKey]?.enabled === true;
    }
  };

  // 检查区域是否有任何可见的功能
  const hasSectionVisibleModules = (sectionKey) => {
    const section = finalConfig[sectionKey];
    if (!section?.enabled) return false;

    return Object.keys(section).some(
      (key) => key !== 'enabled' && section[key] === true,
    );
  };

  // 获取区域的可见功能列表
  const getVisibleModules = (sectionKey) => {
    const section = finalConfig[sectionKey];
    if (!section?.enabled) return [];

    return Object.keys(section).filter(
      (key) => key !== 'enabled' && section[key] === true,
    );
  };

  return {
    loading,
    adminConfig,
    userConfig,
    finalConfig,
    isModuleVisible,
    hasSectionVisibleModules,
    getVisibleModules,
    refreshUserConfig,
  };
};
