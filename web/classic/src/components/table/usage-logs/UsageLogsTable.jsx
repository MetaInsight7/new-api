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
import CardTable from '../../common/ui/CardTable';
import TableEmpty from '../../common/ui/TableEmpty';
import { getLogsColumns, getLogMobileCardRender } from './UsageLogsColumnDefs';
import UsageLogInlineDetail from './UsageLogInlineDetail';

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
    isAdminUser,
    billingDisplayMode,
    t,
    COLUMN_KEYS,
  } = logsData;

  const [expandedRowKeys, setExpandedRowKeys] = useState([]);
  const onOpenDetail = useCallback((record) => {
    setExpandedRowKeys((current) =>
      current[0] === record.key ? [] : [record.key],
    );
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
      expandedRowKeys,
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
    expandedRowKeys,
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

  const expandedRowRender = useCallback(
    (record) => (
      <UsageLogInlineDetail
        record={record}
        detailData={expandData[record.key] || []}
        isAdminUser={isAdminUser}
        copyText={copyText}
        t={t}
      />
    ),
    [expandData, isAdminUser, copyText, t],
  );

  const mobileCardRender = useMemo(
    () =>
      getLogMobileCardRender({
        t,
        copyText,
        showUserInfoFunc,
        openChannelAffinityUsageCacheModal,
        isAdminUser,
        billingDisplayMode,
        onOpenDetail,
        expandedRowKeys,
        expandedRowRender,
      }),
    [
      t,
      copyText,
      showUserInfoFunc,
      openChannelAffinityUsageCacheModal,
      isAdminUser,
      billingDisplayMode,
      onOpenDetail,
      expandedRowKeys,
      expandedRowRender,
    ],
  );

  return (
    <CardTable
      className='usage-log-table-v2'
      columns={tableColumns}
      dataSource={logs}
      rowKey='key'
      loading={loading}
      size='small'
      empty={<TableEmpty title={t('搜索无结果')} />}
      mobileCardRender={mobileCardRender}
      mobileBreakpoint={1180}
      expandedRowKeys={expandedRowKeys}
      expandedRowRender={expandedRowRender}
      rowExpandable={(record) =>
        Boolean(
          expandData[record.key]?.length || record.content || record.request_id,
        )
      }
      hideExpandedColumn
      expandIcon={false}
      expandRowByClick={false}
      onExpandedRowsChange={(keys) => setExpandedRowKeys(keys.slice(-1))}
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
  );
};

export default LogsTable;
