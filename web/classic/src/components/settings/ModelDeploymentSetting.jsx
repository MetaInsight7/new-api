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
import { toBoolean } from '../../helpers';
import { useOptionSettings } from '../../hooks/settings/useOptionSettings';
import SettingModelDeployment from '../../pages/Setting/Model/SettingModelDeployment';

const ModelDeploymentSetting = () => {
  const { inputs, loading, refresh } = useOptionSettings({
    initialValues: {
      'model_deployment.ionet.api_key': '',
      'model_deployment.ionet.enabled': false,
    },
    parseOptions: (data) => {
      const nextInputs = {
        'model_deployment.ionet.api_key': '',
        'model_deployment.ionet.enabled': false,
      };
      data.forEach((item) => {
        nextInputs[item.key] =
          item.key.endsWith('Enabled') || item.key.endsWith('enabled')
            ? toBoolean(item.value)
            : item.value;
      });
      return nextInputs;
    },
  });

  return (
    <>
      <Spin spinning={loading} size='large'>
        <Card style={{ marginTop: '10px' }}>
          <SettingModelDeployment options={inputs} refresh={refresh} />
        </Card>
      </Spin>
    </>
  );
};

export default ModelDeploymentSetting;
