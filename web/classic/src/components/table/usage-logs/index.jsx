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

import React, { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import CardPro from '../../common/ui/CardPro';
import LogsTable from './UsageLogsTable';
import LogsActions from './UsageLogsActions';
import LogsFilters from './UsageLogsFilters';
import ColumnSelectorModal from './modals/ColumnSelectorModal';
import UserInfoModal from './modals/UserInfoModal';
import ChannelAffinityUsageCacheModal from './modals/ChannelAffinityUsageCacheModal';
import ParamOverrideModal from './modals/ParamOverrideModal';
import { useLogsData } from '../../../hooks/usage-logs/useUsageLogsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

// 将内容注入二级导航右侧动作槽(ConsoleSubNav 里的 #console-subnav-actions)。
const SubnavActionsPortal = ({ children }) => {
  const [target, setTarget] = useState(null);
  useEffect(() => {
    let raf;
    const find = () => {
      const el = document.getElementById('console-subnav-actions');
      if (el) {
        setTarget(el);
      } else {
        raf = requestAnimationFrame(find);
      }
    };
    find();
    return () => raf && cancelAnimationFrame(raf);
  }, []);
  return target ? createPortal(children, target) : null;
};

const LogsPage = () => {
  const logsData = useLogsData();
  const isMobile = useIsMobile();

  return (
    <>
      {/* Modals */}
      <ColumnSelectorModal {...logsData} />
      <UserInfoModal {...logsData} />
      <ChannelAffinityUsageCacheModal {...logsData} />
      <ParamOverrideModal {...logsData} />

      {/* 二级导航右侧:快捷时间区间 + 筛选按钮 */}
      <SubnavActionsPortal>
        <LogsActions {...logsData} />
      </SubnavActionsPortal>

      {/* 筛选弹窗(Form 常驻挂载,供快捷区间写入 dateRange) */}
      <LogsFilters {...logsData} />

      {/* Main Content:纯表格,无统计 / 无内嵌筛选 */}
      <CardPro
        type='type2'
        className='usage-log-card'
        paginationArea={createCardProPagination({
          currentPage: logsData.activePage,
          pageSize: logsData.pageSize,
          total: logsData.logCount,
          onPageChange: logsData.handlePageChange,
          onPageSizeChange: logsData.handlePageSizeChange,
          isMobile: isMobile,
          t: logsData.t,
        })}
        t={logsData.t}
      >
        <LogsTable {...logsData} />
      </CardPro>
    </>
  );
};

export default LogsPage;
