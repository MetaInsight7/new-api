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

import React, { useMemo } from 'react';
import CardTable from '../../common/ui/CardTable';
import TableEmpty from '../../common/ui/TableEmpty';
import { getTokensColumns, getTokenMobileCardRender } from './TokensColumnDefs';

const TokensTable = (tokensData) => {
  const {
    tokens,
    loading,
    activePage,
    pageSize,
    tokenCount,
    compactMode,
    handlePageChange,
    handlePageSizeChange,
    handleRow,
    showKeys,
    resolvedTokenKeys,
    loadingTokenKeys,
    toggleTokenVisibility,
    copyTokenKey,
    copyTokenConnectionString,
    manageToken,
    onOpenLink,
    setEditingToken,
    setShowEdit,
    refresh,
    groupRatios,
    t,
  } = tokensData;

  // Get all columns
  const columns = useMemo(() => {
    return getTokensColumns({
      t,
      showKeys,
      resolvedTokenKeys,
      loadingTokenKeys,
      toggleTokenVisibility,
      copyTokenKey,
      copyTokenConnectionString,
      manageToken,
      onOpenLink,
      setEditingToken,
      setShowEdit,
      refresh,
      groupRatios,
    });
  }, [
    t,
    showKeys,
    resolvedTokenKeys,
    loadingTokenKeys,
    toggleTokenVisibility,
    copyTokenKey,
    copyTokenConnectionString,
    manageToken,
    onOpenLink,
    setEditingToken,
    setShowEdit,
    refresh,
    groupRatios,
  ]);

  // Handle compact mode by removing fixed positioning
  const tableColumns = useMemo(() => {
    return compactMode
      ? columns.map((col) => {
          if (col.dataIndex === 'operate') {
            const { fixed, ...rest } = col;
            return rest;
          }
          return col;
        })
      : columns;
  }, [compactMode, columns]);

  const mobileCardRender = useMemo(
    () =>
      getTokenMobileCardRender({
        t,
        showKeys,
        resolvedTokenKeys,
        loadingTokenKeys,
        toggleTokenVisibility,
        copyTokenKey,
        copyTokenConnectionString,
        manageToken,
        setEditingToken,
        setShowEdit,
        refresh,
        groupRatios,
      }),
    [
      t,
      showKeys,
      resolvedTokenKeys,
      loadingTokenKeys,
      toggleTokenVisibility,
      copyTokenKey,
      copyTokenConnectionString,
      manageToken,
      setEditingToken,
      setShowEdit,
      refresh,
      groupRatios,
    ],
  );

  return (
    <CardTable
      columns={tableColumns}
      dataSource={tokens}
      mobileCardRender={mobileCardRender}
      pagination={{
        currentPage: activePage,
        pageSize: pageSize,
        total: tokenCount,
        showSizeChanger: true,
        pageSizeOptions: [10, 20, 50, 100],
        onPageSizeChange: handlePageSizeChange,
        onPageChange: handlePageChange,
      }}
      hidePagination={true}
      loading={loading}
      onRow={handleRow}
      empty={<TableEmpty title={t('搜索无结果')} />}
      className='dmit-flat-table token-table-v2'
      size='middle'
    />
  );
};

export default TokensTable;
