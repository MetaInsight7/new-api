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
import { Layout } from '@douyinfe/semi-ui';
import CardPro from '../../common/ui/CardPro';
import TaskLogsTable from './TaskLogsTable';
import TaskLogsActions from './TaskLogsActions';
import TaskLogsFilters from './TaskLogsFilters';
import ColumnSelectorModal from './modals/ColumnSelectorModal';
import ContentModal from './modals/ContentModal';
import AudioPreviewModal from './modals/AudioPreviewModal';
import { useTaskLogsData } from '../../../hooks/task-logs/useTaskLogsData';
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

const TaskLogsPage = () => {
  const taskLogsData = useTaskLogsData();
  const isMobile = useIsMobile();

  return (
    <>
      {/* Modals */}
      <ColumnSelectorModal {...taskLogsData} />
      <ContentModal {...taskLogsData} isVideo={false} />
      {/* 视频预览弹窗 */}
      <ContentModal
        isModalOpen={taskLogsData.isVideoModalOpen}
        setIsModalOpen={taskLogsData.setIsVideoModalOpen}
        modalContent={taskLogsData.videoUrl}
        isVideo={true}
      />
      <AudioPreviewModal
        isModalOpen={taskLogsData.isAudioModalOpen}
        setIsModalOpen={taskLogsData.setIsAudioModalOpen}
        audioClips={taskLogsData.audioClips}
      />

      {/* 二级导航右侧:快捷时间区间 + 筛选按钮 */}
      <SubnavActionsPortal>
        <TaskLogsActions {...taskLogsData} />
      </SubnavActionsPortal>

      {/* 筛选弹窗(Form 常驻挂载,供快捷区间写入 dateRange) */}
      <TaskLogsFilters {...taskLogsData} />

      {/* Main Content:纯表格,无统计 / 无内嵌筛选 */}
      <Layout>
        <CardPro
          type='type2'
          className='dmit-flat-card'
          paginationArea={createCardProPagination({
            currentPage: taskLogsData.activePage,
            pageSize: taskLogsData.pageSize,
            total: taskLogsData.logCount,
            onPageChange: taskLogsData.handlePageChange,
            onPageSizeChange: taskLogsData.handlePageSizeChange,
            isMobile: isMobile,
            t: taskLogsData.t,
          })}
          t={taskLogsData.t}
        >
          <TaskLogsTable {...taskLogsData} />
        </CardPro>
      </Layout>
    </>
  );
};

export default TaskLogsPage;
