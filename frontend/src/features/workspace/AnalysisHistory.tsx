import { useEffect, useState } from 'react';
import { activateAnalysis, getHistory, type AnalysisRecord } from '../../shared/api/workspace';
import './AnalysisHistory.css';

export default function AnalysisHistory({ analysisId, onOpen }: { analysisId: string | null; onOpen: () => void }) {
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
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : 'История недоступна');
    });
    return () => controller.abort();
  }, [analysisId]);
  async function open() {
    setBusy(true); setError('');
    try { await activateAnalysis(selected); onOpen(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Не удалось открыть анализ'); }
    finally { setBusy(false); }
  }
  return <section className="analysis-history" aria-label="Сохранённые анализы">
    <label htmlFor="analysis-history">История анализов</label>
    <select id="analysis-history" value={selected} disabled={busy || !records.length} onChange={(event) => setSelected(event.target.value)}>
      {!records.length && <option value="">Пока нет сохранённых анализов</option>}
      {records.map((item) => <option key={item.analysis_id} value={item.analysis_id}>
        {new Date(item.created_at).toLocaleString('ru-RU')} · {item.n_nodes} узлов · {item.analysis_id.slice(0, 8)}
      </option>)}
    </select>
    <button className="text-button" type="button" disabled={busy || !selected || selected === analysisId} onClick={open}>{busy ? 'Открытие…' : 'Открыть'}</button>
    {error && <span role="alert">{error}</span>}
  </section>;
}
