import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  RadioGroup,
  Radio,
  TextArea,
  Banner,
  Typography,
} from '@douyinfe/semi-ui';
import { Save } from 'lucide-react';
import { VIOLATION_CATEGORIES } from './violationHelpers';

const SEVS = ['high', 'low'];
const emptyBuckets = () => {
  const b = {};
  VIOLATION_CATEGORIES.forEach((c) => {
    b[c.value] = { high: '', low: '' };
  });
  return b;
};

const lines = (text) =>
  (text || '')
    .split(/[\n,，]/)
    .map((s) => s.trim())
    .filter(Boolean);

// 词库管理:分类切换 + 高危/低危两个大文本框(批量粘贴,一行一个词)。
const ViolationWordsView = ({ data }) => {
  const { t } = data;
  const [buckets, setBuckets] = useState(emptyBuckets());
  const [activeCat, setActiveCat] = useState('politics');

  useEffect(() => {
    data.loadWords();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 词条列表 → 分类×分级 文本
  useEffect(() => {
    const b = emptyBuckets();
    (data.words || []).forEach((w) => {
      const cat = b[w.category] ? w.category : 'other';
      const sev = w.severity === 'low' ? 'low' : 'high';
      b[cat][sev] += (b[cat][sev] ? '\n' : '') + w.word;
    });
    setBuckets(b);
  }, [data.words]);

  const setBucket = (cat, sev, val) =>
    setBuckets((prev) => ({ ...prev, [cat]: { ...prev[cat], [sev]: val } }));

  // 统计(去重后)
  const counts = useMemo(() => {
    const seen = new Set();
    let total = 0;
    const per = {};
    VIOLATION_CATEGORIES.forEach((c) => {
      per[c.value] = 0;
      SEVS.forEach((sev) => {
        lines(buckets[c.value][sev]).forEach((w) => {
          const k = w.toLowerCase();
          if (!seen.has(k)) {
            seen.add(k);
            per[c.value] += 1;
            total += 1;
          }
        });
      });
    });
    return { total, per };
  }, [buckets]);

  const onSave = () => {
    const seen = new Set();
    const out = [];
    VIOLATION_CATEGORIES.forEach((c) => {
      SEVS.forEach((sev) => {
        lines(buckets[c.value][sev]).forEach((w) => {
          const k = w.toLowerCase();
          if (!seen.has(k)) {
            seen.add(k);
            out.push({ word: w, category: c.value, severity: sev });
          }
        });
      });
    });
    data.saveWords(out);
  };

  const catLabel = (c) => `${t(c.label)} (${counts.per[c.value] || 0})`;

  return (
    <div className='va-words'>
      {!data.wordsMeta.enabled && (
        <Banner
          type='warning'
          description={t(
            '违规审计当前未启用。请在「运营设置」中开启后,词库才会对请求生效(未启用时仍可编辑词库)。',
          )}
          closeIcon={null}
          style={{ marginBottom: 12 }}
        />
      )}

      <div className='va-words__bar'>
        <RadioGroup
          type='button'
          buttonSize='middle'
          value={activeCat}
          onChange={(e) => setActiveCat(e.target.value)}
        >
          {VIOLATION_CATEGORIES.map((c) => (
            <Radio key={c.value} value={c.value}>
              {catLabel(c)}
            </Radio>
          ))}
        </RadioGroup>
        <div className='va-words__bar-right'>
          <Typography.Text type='tertiary'>
            {t('共 {{n}} 词', { n: counts.total })}
          </Typography.Text>
          <Button
            theme='solid'
            type='primary'
            size='small'
            icon={<Save size={14} />}
            loading={data.wordsSaving}
            onClick={onSave}
          >
            {t('保存词库')}
          </Button>
        </div>
      </div>

      <div className='va-words__grid'>
        <div className='va-wordbox'>
          <div className='va-wordbox__title'>
            <span className='va-dot va-dot--high' />
            {t('高危词')}
            <Typography.Text type='danger' size='small'>
              {t('命中拦截')}
            </Typography.Text>
          </div>
          <TextArea
            value={buckets[activeCat].high}
            onChange={(v) => setBucket(activeCat, 'high', v)}
            placeholder={t('每行一个词,可整段粘贴')}
            rows={16}
            style={{ resize: 'vertical' }}
          />
        </div>
        <div className='va-wordbox'>
          <div className='va-wordbox__title'>
            <span className='va-dot va-dot--low' />
            {t('低危词')}
            <Typography.Text type='warning' size='small'>
              {t('放行仅记录')}
            </Typography.Text>
          </div>
          <TextArea
            value={buckets[activeCat].low}
            onChange={(v) => setBucket(activeCat, 'low', v)}
            placeholder={t('每行一个词,可整段粘贴')}
            rows={16}
            style={{ resize: 'vertical' }}
          />
        </div>
      </div>
    </div>
  );
};

export default ViolationWordsView;
