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

import React, { useMemo, useState, useCallback } from 'react';
import { Descriptions, Modal, Typography } from '@douyinfe/semi-ui';
import CardTable from '../../common/ui/CardTable';
import TableEmpty from '../../common/ui/TableEmpty';
import { getLogsColumns } from './UsageLogsColumnDefs';

const LogsTable = (logsData) => {
  const {
    logs,
    expandData,
    loading,
    activePage,
    pageSize,
    logCount,
    compactMode,
    visibleColumns,
    handlePageChange,
    handlePageSizeChange,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    hasExpandableRows,
    isAdminUser,
    billingDisplayMode,
    t,
    COLUMN_KEYS,
  } = logsData;

  // The 详情 action button opens a modal with the full row breakdown (replaces
  // the legacy row-expand so there is no expand-arrow column).
  const [detailRecord, setDetailRecord] = useState(null);
  const onOpenDetail = useCallback((record) => {
    setDetailRecord(record);
  }, []);

  // Get all columns
  const allColumns = useMemo(() => {
    return getLogsColumns({
      t,
      COLUMN_KEYS,
      copyText,
      showUserInfoFunc,
      openChannelAffinityUsageCacheModal,
      isAdminUser,
      billingDisplayMode,
      onOpenDetail,
    });
  }, [
    t,
    COLUMN_KEYS,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    isAdminUser,
    billingDisplayMode,
    onOpenDetail,
  ]);

  // Filter columns based on visibility settings
  const getVisibleColumns = () => {
    return allColumns.filter((column) => visibleColumns[column.key]);
  };

  const visibleColumnsList = useMemo(() => {
    return getVisibleColumns();
  }, [visibleColumns, allColumns]);

  const tableColumns = useMemo(() => {
    return compactMode
      ? visibleColumnsList.map(({ fixed, ...rest }) => rest)
      : visibleColumnsList;
  }, [compactMode, visibleColumnsList]);

  const detailData =
    detailRecord && expandData[detailRecord.key]
      ? expandData[detailRecord.key]
      : [];

  return (
    <>
      <CardTable
        className='usage-log-table-v2'
        columns={tableColumns}
        dataSource={logs}
        rowKey='key'
        loading={loading}
        size='small'
        empty={<TableEmpty title={t('搜索无结果')} />}
        pagination={{
          currentPage: activePage,
          pageSize: pageSize,
          total: logCount,
          pageSizeOptions: [10, 20, 50, 100],
          showSizeChanger: true,
          onPageSizeChange: (size) => {
            handlePageSizeChange(size);
          },
          onPageChange: handlePageChange,
        }}
        hidePagination={true}
      />

      <Modal
        title={t('详情')}
        visible={!!detailRecord}
        onCancel={() => setDetailRecord(null)}
        footer={null}
        width={560}
      >
        {detailData.length > 0 ? (
          <Descriptions data={detailData} />
        ) : (
          <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
            {detailRecord?.content || t('暂无更多详情')}
          </Typography.Paragraph>
        )}
      </Modal>
    </>
  );
};

export default LogsTable;
