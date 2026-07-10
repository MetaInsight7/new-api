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

import HeaderBar from './headerbar';
import { Layout } from '@douyinfe/semi-ui';
import SiderBar from './SiderBar';
import ConsoleSubNav from './ConsoleSubNav';
import App from '../../App';
import FooterBar from './Footer';
import { ToastContainer } from 'react-toastify';
import ErrorBoundary from '../common/ErrorBoundary';
import React, { useContext, useEffect, useState } from 'react';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { useSidebarCollapsed } from '../../hooks/common/useSidebarCollapsed';
import { useTranslation } from 'react-i18next';
import {
  API,
  getLogo,
  getSystemName,
  showError,
  setStatusData,
} from '../../helpers';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import { useLocation } from 'react-router-dom';
import { normalizeLanguage } from '../../i18n/language';
const { Sider, Content, Header } = Layout;

const PageLayout = () => {
  const [userState, userDispatch] = useContext(UserContext);
  const [, statusDispatch] = useContext(StatusContext);
  const isMobile = useIsMobile();
  const [collapsed, , setCollapsed] = useSidebarCollapsed();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const { i18n } = useTranslation();
  const location = useLocation();

  const cardProPages = [
    '/console/channel',
    '/console/log',
    '/console/redemption',
    '/console/user',
    '/console/token',
    '/console/midjourney',
    '/console/task',
    '/console/models',
    '/pricing',
    '/rankings',
  ];

  const isHomePage = location.pathname === '/';

  const authPages = ['/login', '/register', '/reset', '/user/reset'];
  const isAuthPage = authPages.includes(location.pathname);
  const shouldHideHeader = isAuthPage && isMobile;

  const shouldHideFooter =
    isHomePage || isAuthPage || cardProPages.includes(location.pathname);

  const shouldInnerPadding =
    location.pathname.includes('/console') &&
    !location.pathname.startsWith('/console/chat');

  const isConsoleRoute = location.pathname.startsWith('/console');
  const showSider = isConsoleRoute && (!isMobile || drawerOpen);
  const isFixedLayout = isConsoleRoute;

  // 二级 pill 导航：console 路由显示（聊天/操练场等全屏工具页除外）
  const showConsoleSubNav =
    isConsoleRoute &&
    !location.pathname.startsWith('/console/chat') &&
    location.pathname !== '/console/playground';

  useEffect(() => {
    if (isMobile && drawerOpen && collapsed) {
      setCollapsed(false);
    }
  }, [isMobile, drawerOpen, collapsed, setCollapsed]);

  const loadUser = () => {
    let user = localStorage.getItem('user');
    if (user) {
      let data = JSON.parse(user);
      userDispatch({ type: 'login', payload: data });
    }
  };

  const loadStatus = async () => {
    try {
      const res = await API.get('/api/status');
      const { success, data } = res.data;
      if (success) {
        statusDispatch({ type: 'set', payload: data });
        setStatusData(data);
      } else {
        showError('Unable to connect to server');
      }
    } catch (error) {
      showError('Failed to load status');
    }
  };

  useEffect(() => {
    loadUser();
    loadStatus().catch(console.error);
    let systemName = getSystemName();
    if (systemName) {
      document.title = systemName;
    }
    let logo = getLogo();
    if (logo) {
      let linkElement = document.querySelector("link[rel~='icon']");
      if (linkElement) {
        linkElement.href = logo;
      }
    }
  }, []);

  useEffect(() => {
    let preferredLang;

    if (userState?.user?.setting) {
      try {
        const settings = JSON.parse(userState.user.setting);
        preferredLang = normalizeLanguage(settings.language);
      } catch (e) {
        // Ignore parse errors
      }
    }

    if (!preferredLang) {
      const savedLang = localStorage.getItem('i18nextLng');
      if (savedLang) {
        preferredLang = normalizeLanguage(savedLang);
      }
    }

    if (preferredLang) {
      localStorage.setItem('i18nextLng', preferredLang);
      if (preferredLang !== i18n.language) {
        i18n.changeLanguage(preferredLang);
      }
    }
  }, [i18n, userState?.user?.setting]);

  return (
    <Layout
      className={`app-layout${isFixedLayout ? ' app-layout-fixed' : ''}${isConsoleRoute ? ' app-console' : ''}${shouldHideHeader ? ' app-layout-auth-mobile' : ''}`}
      style={{
        display: 'flex',
        flexDirection: 'column',
        overflow: isFixedLayout && !isMobile ? 'hidden' : 'visible',
      }}
    >
      {!shouldHideHeader && (
        <Header
          style={{
            padding: 0,
            height: 'auto',
            lineHeight: 'normal',
            position: 'fixed',
            top: 0,
            zIndex: 100,
            left:
              isConsoleRoute && !isMobile
                ? 'var(--sidebar-current-width)'
                : 0,
            width:
              isConsoleRoute && !isMobile
                ? 'calc(100% - var(--sidebar-current-width))'
                : '100%',
          }}
        >
          <HeaderBar
            onMobileMenuToggle={() => setDrawerOpen((prev) => !prev)}
            drawerOpen={drawerOpen}
          />
        </Header>
      )}
      <Layout
        style={{
          overflow: isFixedLayout && !isMobile ? 'auto' : 'visible',
          display: 'flex',
          flexDirection: 'column',
          flex: '1 1 auto',
        }}
      >
        {showSider && (
          <Sider
            className='app-sider'
            style={{
              position: 'fixed',
              left: 0,
              top: isMobile ? '64px' : 0,
              height: isMobile ? 'calc(100dvh - 64px)' : '100dvh',
              zIndex: isMobile ? 99 : 101,
              border: 'none',
              paddingRight: '0',
              width: 'var(--sidebar-current-width)',
            }}
          >
            <SiderBar
              onNavigate={() => {
                if (isMobile) setDrawerOpen(false);
              }}
            />
          </Sider>
        )}
        <Layout
          style={{
            marginLeft: isMobile
              ? '0'
              : showSider
                ? 'var(--sidebar-current-width)'
                : '0',
            flex: '1 1 auto',
            display: 'flex',
            flexDirection: 'column',
            minHeight: 0,
          }}
        >
          <Content
            className={isFixedLayout ? undefined : 'public-page-content'}
            style={{
              flex: isFixedLayout ? '1 0 auto' : '1 1 auto',
              overflowY: isFixedLayout && !isMobile ? 'hidden' : 'visible',
              // 注意:此处不能加 -webkit-overflow-scrolling: touch。
              // Content 自身 overflowY:hidden(不滚动),该属性会让 Safari 把它
              // 提升为合成层却不绘制折叠线以下的内容,导致长页面(如运营设置)
              // 下滑变空白。真正的滚动容器是外层 Layout(overflow:auto)。
              //
              // Safari 合成层重绘 bug:桌面 /console 下,长内容在外层
              // overflow:auto 容器里滚动时,WebKit 不刷新折叠线以下的画面
              // (缩放/改窗口大小能强制重绘让内容回来)。给被滚动的内容层一个
              // 稳定的 GPU 背衬,强制其随滚动正确重绘。Content 内无 position:fixed
              // 后代,提升为合成层不会影响固定侧边栏/头部布局。
              transform:
                isFixedLayout && !isMobile ? 'translateZ(0)' : undefined,
              padding: shouldInnerPadding ? (isMobile ? '5px' : '24px') : '0',
              position: 'relative',
              minHeight: 0,
            }}
          >
            <ErrorBoundary>
              {showConsoleSubNav && <ConsoleSubNav />}
              <App />
            </ErrorBoundary>
          </Content>
          {!shouldHideFooter && (
            <Layout.Footer
              style={{
                flex: '0 0 auto',
                width: '100%',
              }}
            >
              <FooterBar />
            </Layout.Footer>
          )}
        </Layout>
      </Layout>
      <ToastContainer />
    </Layout>
  );
};

export default PageLayout;
