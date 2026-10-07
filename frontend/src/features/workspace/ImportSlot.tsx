import { t, getLocale } from '../../i18n/core.ts';
import { useLocale } from '../../i18n/react';
import { useId, useRef, useState, type DragEvent } from 'react';
import type { ImportKey, ImportSlotState } from './importModel';

const descriptions = { get edges() { return t("Связи между клиентами"); }, get nodes() { return t("Участники сети"); }, get transactions() { return t("История переводов"); } };
export const statusLabels = {
  get empty() { return t("Не выбран"); }, get validating() { return t("Проверка файла…"); }, get selected() { return t("Выбран · ожидает отправки"); },
  get uploading() { return t("Отправка…"); }, get checking() { return t("Проверка структуры…"); }, get ready() { return t("Файл загружен и проверен"); }, get error() { return t("Ошибка"); },
};
const formatSize = (size: number) => new Intl.NumberFormat(getLocale(), { maximumFractionDigits: 1 }).format(size >= 1048576 ? size / 1048576 : size >= 1024 ? size / 1024 : size) + (size >= 1048576 ? t(" МБ") : size >= 1024 ? t(" КБ") : t(" Б"));

export default function ImportSlot({ name, state, locked = false, onChoose, onRemove }: {
  name: ImportKey; state: ImportSlotState; locked?: boolean;
  onChoose: (files: File[]) => void; onRemove: () => void;
}) {
  useLocale();
  const input = useRef<HTMLInputElement>(null);
  const dragDepth = useRef(0);
  const [dragging, setDragging] = useState(false);
  const statusId = useId();
  const busy = ['validating', 'uploading', 'checking'].includes(state.status);
  const progress = Number.isFinite(state.progress) ? Math.max(0, Math.min(100, state.progress!)) : undefined;
  const drop = (event: DragEvent) => {
    event.preventDefault(); dragDepth.current = 0; setDragging(false);
    if (!locked) onChoose(Array.from(event.dataTransfer.files));
  };
  return (
    <section className={`import-slot import-slot--${state.status}${dragging ? ' import-slot--drag' : ''}`}
      aria-label={t("Импорт {0}.parquet", [name])} aria-busy={busy}
      onDragEnter={(event) => { event.preventDefault(); if (!locked && event.dataTransfer.types.includes('Files')) { dragDepth.current++; setDragging(true); } }}
      onDragLeave={(event) => { event.preventDefault(); dragDepth.current = Math.max(0, dragDepth.current - 1); if (!dragDepth.current) setDragging(false); }}
      onDragOver={(event) => { event.preventDefault(); event.dataTransfer.dropEffect = locked ? 'none' : 'copy'; }} onDrop={drop}>
      <div className="import-slot__heading"><h3>{name}.parquet</h3><span>{descriptions[name]}</span></div>
      <input ref={input} hidden type="file" accept=".parquet" tabIndex={-1} disabled={locked}
        aria-label={t("Выбрать {0}.parquet", [name])} aria-describedby={statusId}
        onChange={(event) => { const files = Array.from(event.currentTarget.files ?? []); event.currentTarget.value = ''; if (files.length) onChoose(files); }} />
      <div className="import-slot__body">
        <div className="import-slot__description">
          {state.file ? <><span className="import-slot__filename" title={state.file.name}>{state.file.name}</span><span className="secondary">{formatSize(state.file.size)}</span></>
            : <span className="secondary">{t("Перетащите .parquet")}<br />{t("или выберите файл")}</span>}
        </div>
        <div className="import-slot__actions">
          <button type="button" disabled={locked} aria-label={`${state.file ? t("Заменить") : t("Выбрать")} ${name}.parquet`} onClick={() => input.current?.click()}>{state.file ? t("Заменить") : t("Выбрать")}</button>
          {(state.file || state.status === 'error') && <button type="button" className="import-slot__remove" disabled={locked} aria-label={t("Убрать {0}.parquet", [name])} onClick={onRemove}>{t("Убрать")}</button>}
        </div>
      </div>
      <div key={state.revision + state.status} id={statusId} className="import-slot__status" role={state.status === 'error' ? 'alert' : 'status'}>
        <span className="import-slot__icon" aria-hidden="true">{state.status === 'ready' ? '✓' : state.status === 'error' ? '!' : state.status === 'selected' ? '✓' : busy ? '◌' : '○'}</span>
        <span>{t(state.error || statusLabels[state.status])}{state.status === 'uploading' && progress !== undefined ? ` ${Math.round(progress)}%` : ''}</span>
      </div>
      {state.status === 'uploading' && progress !== undefined && <progress className="import-slot__progress" aria-label={t("Загрузка {0}.parquet", [name])} max={100} value={progress} />}
    </section>
  );
}
