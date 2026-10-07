import { t, getLocale } from '../../i18n/core.ts';
import { useLocale } from '../../i18n/react';
import { useEffect, useState } from 'react';
import { activateAnalysis, getHistory, type AnalysisRecord } from '../../shared/api/workspace';
import './AnalysisHistory.css';

export default function AnalysisHistory({ analysisId, onOpen }: { analysisId: string | null; onOpen: () => void }) {
  useLocale();
  const [records, setRecords] = useState<AnalysisRecord[]>([]);
  const [selected, setSelected] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    const controller = new AbortController();
    setError('');
    getHistory(controller.signal).then((items) => {
      setRecords(items); setSelected(analysisId ?? items[0]?.analysis_id ?? '');
    }).catch((cause: unknown) => {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : t("История недоступна"));
    });
    return () => controller.abort();
  }, [analysisId]);
  async function open() {
    setBusy(true); setError('');
    try { await activateAnalysis(selected); onOpen(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : t("Не удалось открыть анализ")); }
    finally { setBusy(false); }
  }
  return <section className="analysis-history" aria-label={t("Сохранённые анализы")}>
    <label htmlFor="analysis-history">{t("История анализов")}</label>
    <select id="analysis-history" value={selected} disabled={busy || !records.length} onChange={(event) => setSelected(event.target.value)}>
      {!records.length && <option value="">{t("Пока нет сохранённых анализов")}</option>}
      {records.map((item) => <option key={item.analysis_id} value={item.analysis_id}>
        {new Date(item.created_at).toLocaleString(getLocale())} · {item.n_nodes}{" "}{t("узлов ·")}{" "}{item.analysis_id.slice(0, 8)}
      </option>)}
    </select>
    <button className="text-button" type="button" disabled={busy || !selected || selected === analysisId} onClick={open}>{busy ? t("Открытие…") : t("Открыть")}</button>
    {error && <span role="alert">{t(error)}</span>}
  </section>;
}
