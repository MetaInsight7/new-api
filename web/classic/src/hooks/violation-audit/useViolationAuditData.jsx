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
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';

// 违规审计数据 hook:记录列表(分页+筛选)、监控聚合、词库读写。
export function useViolationAuditData() {
  const { t } = useTranslation();

  // 列表
  const [logs, setLogs] = useState([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);

  // 筛选(dateRange 为 [Date, Date])
  const now = Date.now();
  const [filters, setFilters] = useState({
    dateRange: [
      new Date(now - 7 * 24 * 3600 * 1000),
      new Date(now + 3600 * 1000),
    ],
    username: '',
    model_name: '',
    category: '',
    severity: '',
    action: '',
    request_id: '',
  });

  // 监控
  const [stat, setStat] = useState(null);
  const [statLoading, setStatLoading] = useState(false);

  // 快捷时间区间 + 筛选弹窗(与 /console/log 一致)
  const [activeTimeRange, setActiveTimeRange] = useState('7d');
  const [showFilterModal, setShowFilterModal] = useState(false);

  // 词库
  const [words, setWords] = useState([]);
  const [wordsMeta, setWordsMeta] = useState({ enabled: false });
  const [wordsLoading, setWordsLoading] = useState(false);
  const [wordsSaving, setWordsSaving] = useState(false);

  const filtersRef = useRef(filters);
  const mountedRef = useRef(true);
  const logsRequestSeqRef = useRef(0);
  const statRequestSeqRef = useRef(0);
  const wordsRequestSeqRef = useRef(0);
  const wordsSaveRequestSeqRef = useRef(0);
  filtersRef.current = filters;

  const rangeParams = useCallback(() => {
    const f = filtersRef.current;
    const p = {};
    if (Array.isArray(f.dateRange) && f.dateRange.length === 2) {
      const [s, e] = f.dateRange;
      if (s) p.start_timestamp = Math.floor(new Date(s).getTime() / 1000);
      if (e) p.end_timestamp = Math.floor(new Date(e).getTime() / 1000);
    }
    return p;
  }, []);

  const loadLogs = useCallback(
    async (targetPage = page, targetSize = pageSize) => {
      const requestSeq = ++logsRequestSeqRef.current;
      setLoading(true);
      try {
        const f = filtersRef.current;
        const params = {
          p: targetPage,
          page_size: targetSize,
          ...rangeParams(),
        };
        [
          'username',
          'model_name',
          'category',
          'severity',
          'action',
          'request_id',
        ].forEach((k) => {
          if (f[k]) params[k] = f[k];
        });
        const res = await API.get('/api/violation/logs', { params });
        if (!mountedRef.current || requestSeq !== logsRequestSeqRef.current)
          return;
        const { success, message, data } = res.data;
        if (success) {
          setLogs(data.items || []);
          setTotal(data.total || 0);
          setPage(targetPage);
          setPageSize(targetSize);
        } else {
          showError(message);
        }
      } catch (e) {
        if (mountedRef.current && requestSeq === logsRequestSeqRef.current) {
          showError(e.message);
        }
      } finally {
        if (mountedRef.current && requestSeq === logsRequestSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [page, pageSize, rangeParams],
  );

  const loadStat = useCallback(async () => {
    const requestSeq = ++statRequestSeqRef.current;
    setStatLoading(true);
    try {
      const res = await API.get('/api/violation/stat', {
        params: rangeParams(),
      });
      if (!mountedRef.current || requestSeq !== statRequestSeqRef.current)
        return;
      const { success, message, data } = res.data;
      if (success) setStat(data);
      else showError(message);
    } catch (e) {
      if (mountedRef.current && requestSeq === statRequestSeqRef.current) {
        showError(e.message);
      }
    } finally {
      if (mountedRef.current && requestSeq === statRequestSeqRef.current) {
        setStatLoading(false);
      }
    }
  }, [rangeParams]);

  const loadWords = useCallback(async () => {
    const requestSeq = ++wordsRequestSeqRef.current;
    setWordsLoading(true);
    try {
      const res = await API.get('/api/violation/words');
      if (!mountedRef.current || requestSeq !== wordsRequestSeqRef.current)
        return;
      const { success, message, data } = res.data;
      if (success) {
        setWords(data.words || []);
        setWordsMeta({ enabled: data.enabled });
      } else showError(message);
    } catch (e) {
      if (mountedRef.current && requestSeq === wordsRequestSeqRef.current) {
        showError(e.message);
      }
    } finally {
      if (mountedRef.current && requestSeq === wordsRequestSeqRef.current) {
        setWordsLoading(false);
      }
    }
  }, []);

  const saveWords = useCallback(async (nextWords) => {
    const requestSeq = ++wordsSaveRequestSeqRef.current;
    setWordsSaving(true);
    try {
      const res = await API.put('/api/violation/words', { words: nextWords });
      if (
        !mountedRef.current ||
        requestSeq !== wordsSaveRequestSeqRef.current
      ) {
        return false;
      }
      const { success, message, data } = res.data;
      if (success) {
        setWords(data.words || []);
        showSuccess(t('词库已保存'));
        return true;
      }
      showError(message);
      return false;
    } catch (e) {
      if (mountedRef.current && requestSeq === wordsSaveRequestSeqRef.current) {
        showError(e.message);
      }
      return false;
    } finally {
      if (mountedRef.current && requestSeq === wordsSaveRequestSeqRef.current) {
        setWordsSaving(false);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    loadLogs(1, pageSize);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(
    () => () => {
      mountedRef.current = false;
      logsRequestSeqRef.current += 1;
      statRequestSeqRef.current += 1;
      wordsRequestSeqRef.current += 1;
      wordsSaveRequestSeqRef.current += 1;
    },
    [],
  );

  const applyFilters = useCallback(() => {
    loadLogs(1, pageSize);
  }, [loadLogs, pageSize]);

  return {
    t,
    // list
    logs,
    loading,
    page,
    pageSize,
    total,
    loadLogs,
    setPage,
    setPageSize,
    // filters
    filters,
    setFilters,
    applyFilters,
    // stat
    stat,
    statLoading,
    loadStat,
    // 快捷区间 + 筛选弹窗
    activeTimeRange,
    setActiveTimeRange,
    showFilterModal,
    setShowFilterModal,
    // words
    words,
    setWords,
    wordsMeta,
    wordsLoading,
    wordsSaving,
    loadWords,
    saveWords,
  };
}
