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

import React, { useContext, useEffect, useMemo, useRef } from 'react';
import { getRelativeTime } from '../../helpers';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';

import DashboardHeader from './DashboardHeader';
import StatsCards from './StatsCards';
import ChartsPanel from './ChartsPanel';
import AnnouncementsPanel from './AnnouncementsPanel';
import FaqPanel from './FaqPanel';
import UptimePanel from './UptimePanel';
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
  ANNOUNCEMENT_LEGEND_DATA,
  CARD_PROPS,
  FLEX_CENTER_GAP2,
  ILLUSTRATION_SIZE,
  UPTIME_STATUS_MAP,
} from '../../constants/dashboard.constants';
import {
  getUptimeStatusColor,
  getUptimeStatusText,
  renderMonitorList,
} from '../../helpers/dashboard';

const Dashboard = () => {
  // ========== Context ==========
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);

  // ========== 主要数据管理 ==========
  const dashboardData = useDashboardData(userState, userDispatch, statusState);

  // ========== 图表管理 ==========
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

  // ========== 统计数据 ==========
  const { statsData, summaryTitle, summaryNote } = useDashboardStats(
    userState,
    dashboardData.consumeQuota,
    dashboardData.consumeTokens,
    dashboardData.times,
    dashboardData.recentTokens,
    dashboardData.adminUsageSummary,
    dashboardData.activeTimeRange,
    dashboardData.isAdminUser,
    dashboardData.inputs.username,
    dashboardData.performanceMetrics,
    dashboardData.navigate,
    dashboardData.t,
  );
  const timeRangeReady = useRef(false);
  const {
    noticeVisible,
    unreadCount,
    handleNoticeOpen,
    handleNoticeClose,
    markAllAsRead,
    getUnreadKeys,
  } = useNotifications(statusState);

  // ========== 数据处理 ==========
  const loadUserData = async () => {
    if (dashboardData.isAdminUser) {
      const userData = await dashboardData.loadUserQuotaData();
      if (userData && userData.length > 0) {
        dashboardCharts.updateUserChartData(userData);
      }
    }
  };

  const initChart = async () => {
    await dashboardData.loadQuotaData().then((data) => {
      if (data && data.length > 0) {
        dashboardCharts.updateChartData(data);
      }
    });
    await loadUserData();
    await dashboardData.loadUptimeData();
  };

  const handleRefresh = async () => {
    const data = await dashboardData.refresh();
    if (data && data.length > 0) {
      dashboardCharts.updateChartData(data);
    }
    await loadUserData();
  };

  const handleClearAdminUserFilter = async () => {
    const data = await dashboardData.clearAdminUserFilter();
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

  const rangeCaption = useMemo(() => {
    const captionMap = {
      today: '今日 00:00 至当前',
      last_24h: '近 24 小时滚动统计',
      last_7d: '近 7 天统计',
      last_30d: '近 30 天统计',
      custom: '自定义时间范围',
    };
    return dashboardData.t(
      captionMap[dashboardData.activeTimeRange] || captionMap.today,
    );
  }, [dashboardData]);

  // ========== 数据准备 ==========
  const announcementData = (statusState?.status?.announcements || []).map(
    (item) => {
      const pubDate = item?.publishDate ? new Date(item.publishDate) : null;
      const absoluteTime =
        pubDate && !isNaN(pubDate.getTime())
          ? `${pubDate.getFullYear()}-${String(pubDate.getMonth() + 1).padStart(2, '0')}-${String(pubDate.getDate()).padStart(2, '0')} ${String(pubDate.getHours()).padStart(2, '0')}:${String(pubDate.getMinutes()).padStart(2, '0')}`
          : item?.publishDate || '';
      const relativeTime = getRelativeTime(item.publishDate);
      return {
        ...item,
        time: absoluteTime,
        relative: relativeTime,
      };
    },
  );
  const faqData = statusState?.status?.faq || [];

  const uptimeLegendData = Object.entries(UPTIME_STATUS_MAP).map(
    ([status, info]) => ({
      status: Number(status),
      color: info.color,
      label: dashboardData.t(info.label),
    }),
  );

  const announcementLegendData = ANNOUNCEMENT_LEGEND_DATA.map((item) => ({
    ...item,
    label: dashboardData.t(item.label),
  }));

  // ========== Effects ==========
  useEffect(() => {
    initChart();
  }, []);

  useEffect(() => {
    if (!timeRangeReady.current) {
      timeRangeReady.current = true;
      return;
    }
    if (dashboardData.activeTimeRange === 'custom') {
      return;
    }
    handleRefresh();
  }, [dashboardData.activeTimeRange]);

  return (
    <div className='h-full'>
      <DashboardHeader
        getGreeting={dashboardData.getGreeting}
        greetingVisible={dashboardData.greetingVisible}
        rangeCaption={rangeCaption}
        activeTimeRange={dashboardData.activeTimeRange}
        timeRangeOptions={dashboardData.timeRangeOptions}
        handleTimeRangeChange={dashboardData.handleTimeRangeChange}
        showSearchModal={dashboardData.showSearchModal}
        refresh={handleRefresh}
        loading={dashboardData.loading}
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

      <div
        className={`dashboard-overview-grid mb-4 ${
          dashboardData.announcementsEnabled ? '' : 'is-single'
        }`}
      >
        <StatsCards
          statsData={statsData}
          loading={dashboardData.loading}
          CARD_PROPS={CARD_PROPS}
          title={summaryTitle}
          note={summaryNote}
          statusAction={
            dashboardData.isAdminUser &&
            String(dashboardData.inputs.username || '').trim()
              ? handleClearAdminUserFilter
              : undefined
          }
          statusTitle={`${dashboardData.t('清空')} ${dashboardData.t('用户筛选')}`}
          t={dashboardData.t}
        />

        {dashboardData.announcementsEnabled && (
          <div className='dashboard-announcement-panel-slot'>
            <AnnouncementsPanel
              announcementData={announcementData}
              announcementLegendData={announcementLegendData}
              CARD_PROPS={CARD_PROPS}
              ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
              variant='dashboard'
              unreadCount={unreadCount}
              onNoticeOpen={handleAnnouncementOpen}
              t={dashboardData.t}
            />
          </div>
        )}
      </div>

      {/* 图表和 API 接入面板 */}
      <div className='mb-4'>
        <div
          className={`dashboard-workbench-grid ${
            dashboardData.apiInfoEnabled ? '' : 'is-single'
          }`}
        >
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
            CARD_PROPS={CARD_PROPS}
            CHART_CONFIG={CHART_CONFIG}
            FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
            hasApiInfoPanel={dashboardData.apiInfoEnabled}
            t={dashboardData.t}
          />

          {dashboardData.apiInfoEnabled && (
            <WorkspacePanel
              user={userState?.user}
              status={statusState?.status}
              t={dashboardData.t}
            />
          )}
        </div>
      </div>

      {/* 常见问答和服务可用性卡片 */}
      {(dashboardData.faqEnabled || dashboardData.uptimeEnabled) && (
        <div className='mb-4'>
          <div
            className={`dashboard-support-grid ${
              dashboardData.faqEnabled && dashboardData.uptimeEnabled
                ? ''
                : 'is-single'
            }`}
          >
            {/* 常见问答卡片 */}
            {dashboardData.faqEnabled && (
              <FaqPanel
                faqData={faqData}
                CARD_PROPS={CARD_PROPS}
                FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
                ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
                t={dashboardData.t}
              />
            )}

            {/* 服务可用性卡片 */}
            {dashboardData.uptimeEnabled && (
              <UptimePanel
                uptimeData={dashboardData.uptimeData}
                uptimeLoading={dashboardData.uptimeLoading}
                activeUptimeTab={dashboardData.activeUptimeTab}
                setActiveUptimeTab={dashboardData.setActiveUptimeTab}
                loadUptimeData={dashboardData.loadUptimeData}
                uptimeLegendData={uptimeLegendData}
                renderMonitorList={(monitors) =>
                  renderMonitorList(
                    monitors,
                    (status) => getUptimeStatusColor(status, UPTIME_STATUS_MAP),
                    (status) =>
                      getUptimeStatusText(
                        status,
                        UPTIME_STATUS_MAP,
                        dashboardData.t,
                      ),
                    dashboardData.t,
                  )
                }
                CARD_PROPS={CARD_PROPS}
                ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
                t={dashboardData.t}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default Dashboard;

