import { t, getLocale } from '../../i18n/core.ts';
import type { Role } from '../../shared/contracts';

export const roleLabels: Record<Role, string> = {
  get consolidator() { return t("Консолидация"); },
  get transit() { return t("Транзит"); },
  get distributor() { return t("Распределение"); },
  get terminal() { return t("Конечный получатель"); },
  get coordinator() { return t("Координация"); },
  get peripheral() { return t("Периферия"); },
};

export const formatMoney = (value: number) =>
  new Intl.NumberFormat(getLocale(), { maximumFractionDigits: 0 }).format(value) + ' ₸';

export const formatScore = (value: number) => new Intl.NumberFormat(getLocale(), { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);
