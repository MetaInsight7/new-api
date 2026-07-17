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

import { useState, useEffect, useContext, useCallback, useMemo } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import { useSetTheme, useTheme, useActualTheme } from '../../context/Theme';
import { API } from '../../helpers/api';
import {
  getLogo,
  getStoredJSON,
  removeStoredValue,
  setStoredValue,
  getSystemName,
} from '../../helpers/siteStorage';
import { showSuccess } from '../../helpers/notifications';
import { normalizeLanguage } from '../../i18n/language';
import { useIsMobile } from './useIsMobile';
import { useSidebarCollapsed } from './useSidebarCollapsed';
import { useMinimumLoadingTime } from './useMinimumLoadingTime';
import { normalizeHeaderNavModules } from '../../helpers/navigationConfig';
import { toBoolean } from '../../helpers/boolean';

export const useHeaderBar = ({ onMobileMenuToggle, drawerOpen }) => {
  const { t, i18n } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const isMobile = useIsMobile();
  const [collapsed, toggleCollapsed] = useSidebarCollapsed();
  const [logoLoaded, setLogoLoaded] = useState(false);
  const navigate = useNavigate();
  const [currentLang, setCurrentLang] = useState(
    normalizeLanguage(i18n.language),
  );
  const location = useLocation();

  const [cachedStatus] = useState(() => getStoredJSON('status', {}));
  const effectiveStatus = useMemo(
    () =>
      statusState?.status && Object.keys(statusState.status).length > 0
        ? statusState.status
        : cachedStatus,
    [statusState?.status, cachedStatus],
  );
  const loading =
    statusState?.status === undefined && Object.keys(cachedStatus).length === 0;
  const isLoading = useMinimumLoadingTime(loading, 200);

  const systemName = getSystemName();
  const logo = getLogo();
  const currentDate = new Date();
  const isNewYear = currentDate.getMonth() === 0 && currentDate.getDate() === 1;

  const isSelfUseMode = toBoolean(effectiveStatus?.self_use_mode_enabled);
  const docsLink = effectiveStatus?.docs_link || '';
  const isDemoSiteMode = toBoolean(effectiveStatus?.demo_site_enabled);

  // 获取顶栏模块配置
  const headerNavModulesConfig = effectiveStatus?.HeaderNavModules;

  // 使用useMemo确保headerNavModules正确响应statusState变化
  const headerNavModules = useMemo(
    () => normalizeHeaderNavModules(headerNavModulesConfig),
    [headerNavModulesConfig],
  );

  // 获取模型广场权限配置
  const pricingRequireAuth = useMemo(() => {
    if (headerNavModules?.pricing) {
      return headerNavModules.pricing.requireAuth;
    }
    return false; // 默认不需要登录
  }, [headerNavModules]);

  const isConsoleRoute = location.pathname.startsWith('/console');

  const theme = useTheme();
  const actualTheme = useActualTheme();
  const setTheme = useSetTheme();

  // Logo loading effect
  useEffect(() => {
    setLogoLoaded(false);
    if (!logo) return;
    const img = new Image();
    img.src = logo;
    img.onload = () => setLogoLoaded(true);
  }, [logo]);

  // Keep language state in sync with i18next. Embedded pages are responsible
  // for receiving their own messages; broadcasting to the first iframe on the
  // page could leak UI state to an unrelated frame.
  useEffect(() => {
    const handleLanguageChanged = (lng) => {
      const normalizedLang = normalizeLanguage(lng);
      setCurrentLang(normalizedLang);
    };

    i18n.on('languageChanged', handleLanguageChanged);
    return () => {
      i18n.off('languageChanged', handleLanguageChanged);
    };
  }, [i18n]);

  // Actions
  const logout = useCallback(async () => {
    try {
      await API.get('/api/user/logout');
    } finally {
      // Clear local auth state even when the server is unavailable.
      userDispatch({ type: 'logout' });
      removeStoredValue('user');
      navigate('/login');
    }
    showSuccess(t('注销成功!'));
  }, [navigate, t, userDispatch]);

  const handleLanguageChange = useCallback(
    async (lang) => {
      // Change language immediately for responsive UX
      const previousLang = normalizeLanguage(i18n.language);
      i18n.changeLanguage(lang);
      setStoredValue('i18nextLng', lang);

      // If user is logged in, save preference to backend
      if (userState?.user?.id) {
        try {
          const res = await API.put('/api/user/self', {
            language: lang,
          });
          if (res.data.success) {
            // Keep user preference and local cache in sync so route changes
            // don't reapply an older remembered language.
            let settings = {};
            if (userState?.user?.setting) {
              try {
                settings = JSON.parse(userState.user.setting) || {};
              } catch (e) {
                settings = {};
              }
            }

            settings.language = lang;
            const nextUser = {
              ...userState.user,
              setting: JSON.stringify(settings),
            };

            userDispatch({
              type: 'login',
              payload: nextUser,
            });
            setStoredValue('user', JSON.stringify(nextUser));
          }
        } catch (error) {
          if (previousLang) {
            i18n.changeLanguage(previousLang);
            setStoredValue('i18nextLng', previousLang);
          }
          console.error('Failed to save language preference:', error);
        }
      }
    },
    [i18n, userState, userDispatch],
  );

  const handleThemeToggle = useCallback(
    (newTheme) => {
      if (
        !newTheme ||
        (newTheme !== 'light' && newTheme !== 'dark' && newTheme !== 'auto')
      ) {
        return;
      }
      setTheme(newTheme);
    },
    [setTheme],
  );

  const handleMobileMenuToggle = useCallback(() => {
    if (isMobile) {
      onMobileMenuToggle();
    } else {
      toggleCollapsed();
    }
  }, [isMobile, onMobileMenuToggle, toggleCollapsed]);

  return {
    // State
    userState,
    statusState,
    isMobile,
    collapsed,
    logoLoaded,
    currentLang,
    location,
    isLoading,
    systemName,
    logo,
    isNewYear,
    isSelfUseMode,
    docsLink,
    isDemoSiteMode,
    isConsoleRoute,
    theme,
    drawerOpen,
    headerNavModules,
    pricingRequireAuth,

    // Actions
    logout,
    handleLanguageChange,
    handleThemeToggle,
    handleMobileMenuToggle,
    navigate,
    t,
  };
};
