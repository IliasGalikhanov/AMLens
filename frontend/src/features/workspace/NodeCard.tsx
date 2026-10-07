import { t } from '../../i18n/core.ts';
import { useLocale } from '../../i18n/react';
import { useState } from 'react';
import type { NodeCardResponse } from '../../shared/api/types';
import type { GraphSlice, NodeDetails, NodeSummary } from '../../shared/contracts';
import Icon from './Icon';
import { formatMoney, formatScore, roleLabels } from './labels';
import { flowInSlice } from './workspaceModel';
import './NodeCard.css';

export default function NodeCard({ node, detail, graph, card }: { node: NodeSummary; detail: NodeDetails | null; graph: GraphSlice | null; card?: NodeCardResponse | null }) {
  useLocale();
  const [copyStatus, setCopyStatus] = useState('');
  const observation = card?.node.gid === node.gid ? card : null;
  const flow = detail || flowInSlice(graph, node.gid);
  const high = node.priority_score >= 0.8;
  const copy = async () => {
    try { await navigator.clipboard.writeText(node.gid); setCopyStatus(t("gid скопирован")); }
    catch { setCopyStatus(t("Не удалось скопировать. Выделите gid вручную.")); }
  };
  return <div className="node-card">
    <div className="node-card__identity">
      <span className="node-avatar"><Icon name="network" size={22} /></span>
      <div><p className="eyebrow">{t("КЛИЕНТ")}</p><h3 className="node-card__gid">{node.gid}</h3></div>
      <button type="button" className="icon-button" aria-label={t("Скопировать gid")} title={t("Скопировать gid")} onClick={() => { void copy(); }}><Icon name={copyStatus === t("gid скопирован") ? 'check' : 'copy'} size={16} /></button>
    </div>
    {copyStatus && <p className="copy-status" role="status">{t(copyStatus)}</p>}
    <div className="node-card__role"><span>{roleLabels[node.role]}</span><span className="secondary">{t("Кластер")}{" "}{node.cluster_id}</span></div>
    <section className={'priority-summary' + (high ? ' priority-summary--high' : '')} aria-label={t("Приоритет проверки")}>
      <div><span>{t("Приоритет проверки")}</span><strong>{formatScore(node.priority_score)}<span> / 1</span></strong></div>
      <div className="priority-meter" role="meter" aria-label={t("Приоритет")} aria-valuemin={0} aria-valuemax={1} aria-valuenow={node.priority_score}><span style={{ width: Math.max(0, Math.min(1, node.priority_score)) * 100 + '%' }} /></div>
      <p>{high ? t("Высокий приоритет · рекомендуется проверить") : t("Приоритет по структурным признакам")}</p>
    </section>
    <dl className="node-facts">
      <div><dt>{t("Сила признаков роли")}</dt><dd>{formatScore(node.role_score)}</dd></div>
      <div><dt>{t("Глубина связей")}</dt><dd>{node.depth}</dd></div>
      <div><dt>{t("Исходный узел (seed)")}</dt><dd>{node.is_seed ? t("Да") : t("Нет")}</dd></div>
    </dl>
    <section className="node-card__section">
      <h3>{detail ? observation ? t("Переводы в загруженной выборке") : t("Наблюдаемый поток") : flow ? t("Поток в показанном срезе") : t("Денежные потоки")}</h3>
      {flow ? <><dl className="flow-metrics">
        <div><dt><span className="flow-arrow">↙</span>{t("Входящие")}</dt><dd>{formatMoney(flow.incoming_sum_kzt)}</dd></div>
        <div><dt><span className="flow-arrow flow-arrow--out">↗</span>{t("Исходящие")}</dt><dd>{formatMoney(flow.outgoing_sum_kzt)}</dd></div>
      </dl><div className="counterparties"><span>{t("Плательщики")}{" "}<strong>{flow.unique_payers}</strong></span><span>{t("Получатели")}{" "}<strong>{flow.unique_recipients}</strong></span></div></>
        : <p className="secondary">{t("Детализация переводов для этого клиента пока не получена.")}</p>}
    </section>
    <section className="node-card__section evidence-section">
      <h3><Icon name="info" size={16} />{t("Основание для проверки")}</h3>
      <p>{t(node.evidence) || t("Описание признаков пока не получено.")}</p>
    </section>
    {observation && <section className="node-card__section node-observation" aria-label={t("Контекст наблюдения")}>
      <h3>{t("Контекст наблюдения")}</h3>
      {observation.limitations.length > 0 && <details className="node-observation__group" open>
        <summary><Icon name="chevron" size={14} /><span>{t("Ограничения выборки")}</span><span className="node-observation__count">{observation.limitations.length}</span></summary>
        <ul className="node-observation__items">
          {observation.limitations.map((limitation, index) => <li key={index}>{limitation}</li>)}
        </ul>
      </details>}
      {observation.data_gaps.length > 0 && <details className="node-observation__group">
        <summary><Icon name="chevron" size={14} /><span>{t("Пробелы в данных")}</span><span className="node-observation__count">{observation.data_gaps.length}</span></summary>
        <ul className="node-observation__items">
          {observation.data_gaps.map(gap => <li key={gap.code}>
            <p className="node-observation__label">{gap.description}</p>
            <p className="node-observation__evidence">{gap.evidence}</p>
          </li>)}
        </ul>
      </details>}
      {observation.next_requests.length > 0 && <details className="node-observation__group">
        <summary><Icon name="chevron" size={14} /><span>{t("Что запросить для проверки")}</span><span className="node-observation__count">{observation.next_requests.length}</span></summary>
        <ul className="node-observation__items">
          {observation.next_requests.map(request => <li key={request.gap_code}>
            <p className="node-observation__label">{request.request}</p>
            <p className="node-observation__evidence">{request.reason}</p>
          </li>)}
        </ul>
      </details>}
      {!observation.limitations.length && !observation.data_gaps.length && !observation.next_requests.length && <p className="node-observation__evidence">{t("Дополнительные пояснения для этого клиента не получены.")}</p>}
    </section>}
    <p className="node-card__disclaimer">{t("Выводы ограничены доступной выборкой. Роль не подтверждает нарушение.")}</p>
  </div>;
}
