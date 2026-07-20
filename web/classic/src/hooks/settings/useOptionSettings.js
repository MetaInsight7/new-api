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

import { useCallback, useEffect, useRef, useState } from 'react';
import { API, showError } from '../../helpers';

/**
 * Shared loader for Semi admin settings backed by /api/option/.
 * The parser stays page-specific while request state and cleanup stay shared.
 */
export const useOptionSettings = ({ initialValues, parseOptions }) => {
  const initialValuesRef = useRef(initialValues || {});
  const parseOptionsRef = useRef(parseOptions || ((data) => data));
  const [inputs, setInputs] = useState(() => ({ ...initialValuesRef.current }));
  const [loading, setLoading] = useState(false);

  parseOptionsRef.current = parseOptions || ((data) => data);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/option/');

      const { success, message, data } = response.data || {};
      if (!success) {
        showError(message);
        return;
      }

      const nextInputs = parseOptionsRef.current(
        Array.isArray(data) ? data : [],
        initialValuesRef.current,
      );
      setInputs(nextInputs || {});
    } catch (error) {
      showError(error?.message || '刷新失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return { inputs, setInputs, loading, setLoading, refresh };
};

export default useOptionSettings;
