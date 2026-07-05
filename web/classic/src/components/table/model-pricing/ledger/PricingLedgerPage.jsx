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
import './pricing-ledger.css';
import LedgerHero from './LedgerHero';
import LedgerToolbar from './LedgerToolbar';
import LedgerTable from './LedgerTable';
import LedgerMobileList from './LedgerMobileList';
import { useModelPricingData } from '../../../../hooks/model-pricing/useModelPricingData';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const PricingLedgerPage = () => {
  const data = useModelPricingData();
  const isMobile = useIsMobile();
  const { t } = data;

  // 价格计算上下文（跟随站点配置，无交互展示面板）
  const ctx = useMemo(
    () => ({
      groupRatio: data.groupRatio,
      tokenUnit: data.tokenUnit,
      displayPrice: data.displayPrice,
      currency: data.currency,
      siteDisplayType: data.siteDisplayType,
    }),
    [
      data.groupRatio,
      data.tokenUnit,
      data.displayPrice,
      data.currency,
      data.siteDisplayType,
    ],
  );

  const listProps = {
    t,
    models: data.filteredModels,
    loading: data.loading,
    filterGroup: data.filterGroup,
    pageSize: data.pageSize,
    currentPage: data.currentPage,
    setCurrentPage: data.setCurrentPage,
    ctx,
  };

  return (
    <div className='pricing-dazi'>
      <div className='shell wrappad'>
        <LedgerHero t={t} />

        <div className='manifest'>
          <LedgerToolbar {...data} />

          {isMobile ? (
            <LedgerMobileList {...listProps} />
          ) : (
            <LedgerTable {...listProps} />
          )}
        </div>
      </div>
    </div>
  );
};

export default PricingLedgerPage;
