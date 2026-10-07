import { t } from '../../i18n/core.ts';
import { useLocale } from '../../i18n/react';
import { formatScore } from './labels';
import { HIGH_PRIORITY_THRESHOLD } from './statistics';

export default function PriorityIndicator({ score }: { score: number }) {
  useLocale();
  const high = score >= HIGH_PRIORITY_THRESHOLD;
  const label = `${high ? t("Высокий приоритет") : t("Приоритет")}: ${formatScore(score)}`;
  return <span className={`priority-indicator${high ? ' priority-indicator--high' : ''}`} title={label} aria-label={label}>
    {high && <span className="priority-indicator__dot" aria-hidden="true" />}
    <span>{formatScore(score)}</span>
  </span>;
}
