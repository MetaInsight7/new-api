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

import React, { useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Tooltip } from '@douyinfe/semi-ui';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';
import { BarChart3, Megaphone, Server } from 'lucide-react';

import DashboardHeader from './DashboardHeader';
import StatsCards from './StatsCards';
import ChartsPanel from './ChartsPanel';
import AvailabilityRail from './AvailabilityRail';
import AnnouncementPreview from './AnnouncementPreview';
import WorkspacePanel from './WorkspacePanel';
import SearchModal from './modals/SearchModal';
import NoticeModal from '../layout/NoticeModal';
import './workspace.css';

import { useDashboardData } from '../../hooks/dashboard/useDashboardData';
import { useDashboardStats } from '../../hooks/dashboard/useDashboardStats';
import { useDashboardCharts } from '../../hooks/dashboard/useDashboardCharts';
import { useNotifications } from '../../hooks/common/useNotifications';

import {
  CHART_CONFIG,
  UPTIME_STATUS_MAP,
} from '../../constants/dashboard.constants';

const Dashboard = () => {
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);

  const dashboardData = useDashboardData(userState, userDispatch, statusState);

  const dashboardCharts = useDashboardCharts(
    dashboardData.dataExportDefaultTime,
    dashboardData.setTrendData,
    dashboardData.setConsumeQuota,
    dashboardData.setTimes,
    dashboardData.setConsumeTokens,
    dashboardData.setPieData,
    dashboardData.setLineData,
    dashboardData.setModelColors,
    dashboardData.t,
  );

  const { statsData, quickActions, balanceCaption } = useDashboardStats(
    userState,
    dashboardData.consumeQuota,
    dashboardData.consumeTokens,
    dashboardData.times,
    dashboardData.recentTokens,
    dashboardData.comparisonSummary,
    dashboardData.adminUsageSummary,
    dashboardData.selectedAdminUser,
    dashboardData.activeTimeRange,
    dashboardData.isAdminUser,
    dashboardData.inputs.username,
    dashboardData.performanceMetrics,
    dashboardData.navigate,
    dashboardData.t,
  );

  const timeRangeReady = useRef(false);
  const [apiState, setApiState] = useState({
    ready: null,
    lineCount: 0,
    tokenCount: 0,
  });
  const {
    noticeVisible,
    unreadCount,
    announcements,
    handleNoticeOpen,
    handleNoticeClose,
    markAllAsRead,
    getUnreadKeys,
  } = useNotifications(statusState);

  const loadUserData = async () => {
    if (dashboardData.isAdminUser) {
      const userData = await dashboardData.loadUserQuotaData();
      if (userData === null) return null;
      if (userData && userData.length > 0) {
        dashboardCharts.updateUserChartData(userData);
      }
    }
    return [];
  };

  const initChart = async () => {
    const data = await dashboardData.loadQuotaData();
    if (data === null) return;
    if (data.length > 0) {
      dashboardCharts.updateChartData(data);
    }
    await loadUserData();
    await dashboardData.loadUptimeData();
  };

  const handleRefresh = async () => {
    const data = await dashboardData.refresh();
    if (data === null) return;
    if (data && data.length > 0) {
      dashboardCharts.updateChartData(data);
    }
    await loadUserData();
  };

  const handleSearchConfirm = async () => {
    await dashboardData.handleSearchConfirm(dashboardCharts.updateChartData);
    await loadUserData();
  };

  const handleAnnouncementOpen = () => {
    markAllAsRead();
    handleNoticeOpen();
  };

  const balanceStatus = useMemo(
    () => ({
      text: balanceCaption.text,
      tone: balanceCaption.tone,
    }),
    [balanceCaption],
  );

  const allMonitors = useMemo(
    () =>
      (dashboardData.uptimeData || []).flatMap((g) => g.monitors || []),
    [dashboardData.uptimeData],
  );

  const uptimeLegendData = useMemo(
    () =>
      Object.entries(UPTIME_STATUS_MAP).map(([status, info]) => ({
        status: Number(status),
        color: info.color,
        label: dashboardData.t(info.label),
      })),
    [dashboardData.t],
  );

  useEffect(() => {
    initChart();
  }, []);

  useEffect(() => {
    if (!timeRangeReady.current) {
      timeRangeReady.current = true;
      return;
    }
    if (dashboardData.activeTimeRange === 'custom') return;
    handleRefresh();
  }, [dashboardData.activeTimeRange]);

  return (
    <div className='dashboard-page h-full'>
      <DashboardHeader
        getGreeting={dashboardData.getGreeting}
        greetingVisible={dashboardData.greetingVisible}
        activeTimeRange={dashboardData.activeTimeRange}
        timeRangeOptions={dashboardData.timeRangeOptions}
        handleTimeRangeChange={dashboardData.handleTimeRangeChange}
        showSearchModal={dashboardData.showSearchModal}
        refresh={handleRefresh}
        loading={dashboardData.loading}
        balanceStatus={balanceStatus}
        quickActions={quickActions}
        unreadCount={unreadCount}
        onNoticeOpen={
          dashboardData.announcementsEnabled
            ? handleAnnouncementOpen
            : undefined
        }
        t={dashboardData.t}
      />

      <SearchModal
        searchModalVisible={dashboardData.searchModalVisible}
        handleSearchConfirm={handleSearchConfirm}
        handleCloseModal={dashboardData.handleCloseModal}
        isMobile={dashboardData.isMobile}
        isAdminUser={dashboardData.isAdminUser}
        inputs={dashboardData.inputs}
        dataExportDefaultTime={dashboardData.dataExportDefaultTime}
        timeOptions={dashboardData.timeOptions}
        handleInputChange={dashboardData.handleInputChange}
        t={dashboardData.t}
      />

      <NoticeModal
        visible={noticeVisible}
        onClose={handleNoticeClose}
        isMobile={dashboardData.isMobile}
        defaultTab='system'
        unreadKeys={getUnreadKeys()}
      />

      {/* DMIT-style 3-column card */}
      <div className='dashboard-main-card mb-4'>
        {/* Column 1: 使用情况 (4 metric cards) */}
        <div className='dashboard-main-section'>
          <div className='dashboard-main-section__header'>
            <BarChart3 size={16} />
            <span>{dashboardData.t('使用情况')}</span>
          </div>
          <div className='dashboard-main-section__body'>
            <StatsCards
              statsData={statsData}
              loading={dashboardData.loading}
              t={dashboardData.t}
            />
          </div>
        </div>

        {/* Column 2: 概要 (API Info) */}
        {dashboardData.apiInfoEnabled && (
          <div className='dashboard-main-section'>
            <div className='dashboard-main-section__header'>
              <Server size={16} />
              <span>{dashboardData.t('概要')}</span>
              {apiState.ready !== null && (
                <>
                  <span
                    className={`dashboard-api-status-chip is-${
                      apiState.ready ? 'ready' : 'pending'
                    }`}
                  >
                    <span className='dashboard-api-status-chip__dot' />
                    {apiState.ready
                      ? dashboardData.t('已就绪')
                      : dashboardData.t('待配置')}
                  </span>
                  <span className='dashboard-api-status-meta'>
                    {dashboardData.t(
                      '{{lineCount}} 条线路 · {{tokenCount}} 个令牌',
                      {
                        lineCount: apiState.lineCount,
                        tokenCount: apiState.tokenCount,
                      },
                    )}
                  </span>
                </>
              )}
            </div>
            <div className='dashboard-main-section__body'>
              <WorkspacePanel
                user={userState?.user}
                status={statusState?.status}
                t={dashboardData.t}
                onStateChange={setApiState}
              />
            </div>
          </div>
        )}

        {/* Column 3: 系统公告预览（原余额充值位置） */}
        <div className='dashboard-main-section is-notice'>
          <div className='dashboard-main-section__header'>
            <Megaphone size={16} />
            <span>{dashboardData.t('系统公告')}</span>
            <div className='dashboard-main-section__header-actions'>
              <button
                type='button'
                title={dashboardData.t('查看全部')}
                onClick={handleAnnouncementOpen}
              >
                <Megaphone size={13} />
                {dashboardData.t('查看')}
              </button>
            </div>
          </div>
          <div className='dashboard-main-section__body'>
            <AnnouncementPreview
              announcements={announcements}
              onViewAll={handleAnnouncementOpen}
              t={dashboardData.t}
            />
          </div>
        </div>
      </div>

      {/* Chart section + 服务可用性侧栏 */}
      <div
        className={`dashboard-chart-row mb-4${
          dashboardData.uptimeEnabled ? ' has-rail' : ''
        }`}
      >
        <div className='dashboard-chart-row__main'>
          <ChartsPanel
            activeChartTab={dashboardData.activeChartTab}
            setActiveChartTab={dashboardData.setActiveChartTab}
            spec_line={dashboardCharts.spec_line}
            spec_model_line={dashboardCharts.spec_model_line}
            spec_pie={dashboardCharts.spec_pie}
            spec_rank_bar={dashboardCharts.spec_rank_bar}
            spec_user_rank={dashboardCharts.spec_user_rank}
            spec_user_trend={dashboardCharts.spec_user_trend}
            isAdminUser={dashboardData.isAdminUser}
            CHART_CONFIG={CHART_CONFIG}
            t={dashboardData.t}
          />
        </div>
        {dashboardData.uptimeEnabled && (
          <AvailabilityRail
            monitors={allMonitors}
            legend={uptimeLegendData}
            t={dashboardData.t}
          />
        )}
      </div>
    </div>
  );
};

export default Dashboard;
