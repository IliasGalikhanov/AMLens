import { t, getLocale } from '../../i18n/core.ts';
import { useLocale } from '../../i18n/react';
import { useMemo } from 'react';
import type { GraphSlice } from '../../shared/contracts';
import { formatMoney } from './labels';
import { summarizeGraph } from './statistics';

const decimal = { format: (value: number) => new Intl.NumberFormat(getLocale(), { maximumFractionDigits: 1 }).format(value) };
const compactMoney = (value: number) => new Intl.NumberFormat(getLocale(), { notation: 'compact', maximumFractionDigits: 2 }).format(value) + ' ₸';
const percent = (value: number | null | undefined) => value == null ? '—' : `${decimal.format(value * 100)}%`;

export default function GraphStatistics({ graph, loading, full = false }: { graph: GraphSlice | null; loading: boolean; full?: boolean }) {
  useLocale();
  const stats = useMemo(() => graph ? summarizeGraph(graph) : null, [graph]);
  const unavailable = loading || !stats;
  return (
    <section className="statistics panel" aria-label={full ? t("Статистика всего анализа") : t("Статистика текущего среза")} aria-busy={loading}>
      <div className="statistics__scope">
        <h2>{t("Обзор сети")}</h2>
        <span>{loading ? t("Загрузка данных…") : stats ? t("{0} · {1} узлов", [full ? t("Весь анализ") : t("Демонстрационный срез"), stats.count]) : t("Данные не загружены")}</span>
      </div>
      <dl className="statistics__grid">
        <div><dt>{t("Оборот переводов")}</dt><dd>{unavailable ? '—' : <span className="metric-value" tabIndex={0} aria-label={formatMoney(stats.turnover)}><span aria-hidden="true">{compactMoney(stats.turnover)}</span><span className="metric-value__exact" aria-hidden="true">{formatMoney(stats.turnover)}</span></span>}</dd><span>{full ? t("по всем рёбрам анализа") : t("по рёбрам среза")}</span></div>
        <div><dt>{t("Высокий приоритет")}</dt><dd>{unavailable ? '—' : percent(stats.highShare)}</dd><span>{unavailable ? t("Нет данных") : t("{0} из {1} узлов · ≥ 0,80", [stats.highCount, stats.count])}</span></div>
        <div><dt>{t("Кластеры")}</dt><dd>{unavailable ? '—' : stats.clusterCount}</dd><span>{unavailable || stats.averageClusterSize === null ? t("Средний размер —") : t("Средний размер: {0}", [decimal.format(stats.averageClusterSize)])}</span></div>
        <div><dt>{t("Консолидация / координация")}</dt><dd>{unavailable ? '—' : `${stats.consolidators} / ${stats.coordinators}`}</dd><span>{t("кандидаты для проверки")}</span></div>
        <div><dt>{t("Вне крупнейшей компоненты")}</dt><dd>{unavailable ? '—' : percent(stats.outsideShare)}</dd><span>{unavailable ? t("Нет данных") : t("{0} из {1} узлов · слабая связность", [stats.outsideCount, stats.count])}</span></div>
      </dl>
    </section>
  );
}
