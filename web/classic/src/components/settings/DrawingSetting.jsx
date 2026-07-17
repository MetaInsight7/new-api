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

import React from 'react';
import { Card, Spin } from '@douyinfe/semi-ui';
import SettingsDrawing from '../../pages/Setting/Drawing/SettingsDrawing';
import { toBoolean } from '../../helpers';
import { useOptionSettings } from '../../hooks/settings/useOptionSettings';

const DrawingSetting = () => {
  const { inputs, loading, refresh } = useOptionSettings({
    initialValues: {
      DrawingEnabled: false,
      MjNotifyEnabled: false,
      MjAccountFilterEnabled: false,
      MjForwardUrlEnabled: false,
      MjModeClearEnabled: false,
      MjActionCheckSuccessEnabled: false,
    },
    parseOptions: (data) => {
      const nextInputs = {};
      data.forEach((item) => {
        nextInputs[item.key] = item.key.endsWith('Enabled')
          ? toBoolean(item.value)
          : item.value;
      });
      return nextInputs;
    },
  });

  return (
    <>
      <Spin spinning={loading} size='large'>
        {/* 绘图设置 */}
        <Card style={{ marginTop: '10px' }}>
          <SettingsDrawing options={inputs} refresh={refresh} />
        </Card>
      </Spin>
    </>
  );
};

export default DrawingSetting;
