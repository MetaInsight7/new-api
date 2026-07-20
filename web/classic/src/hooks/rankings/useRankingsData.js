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

import { useState, useEffect, useCallback, useRef } from 'react';
import { API } from '../../helpers/api';

const VALID_PERIODS = ['today', 'week', 'month', 'year', 'all'];

export function useRankingsData(initialPeriod = 'week') {
  const [period, setPeriod] = useState(
    VALID_PERIODS.includes(initialPeriod) ? initialPeriod : 'week'
  );
  const [snapshot, setSnapshot] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const requestSeqRef = useRef(0);

  const fetchRankings = useCallback(async (p) => {
    const requestSeq = ++requestSeqRef.current;
    setLoading(true);
    setError(null);
    try {
      const res = await API.get('/api/rankings', { params: { period: p } });
      if (requestSeq !== requestSeqRef.current) return;
      const { success, message, data } = res.data;
      if (success) {
        setSnapshot(data);
      } else {
        setError(message);
      }
    } catch (err) {
      if (requestSeq !== requestSeqRef.current) return;
      const msg = err?.response?.data?.message || err.message;
      setError(msg);
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    fetchRankings(period);
  }, [period, fetchRankings]);

  useEffect(() => () => {
    requestSeqRef.current += 1;
  }, []);

  const changePeriod = useCallback((p) => {
    if (VALID_PERIODS.includes(p)) setPeriod(p);
  }, []);

  const retry = useCallback(() => fetchRankings(period), [fetchRankings, period]);

  return { period, changePeriod, snapshot, loading, error, retry };
}
