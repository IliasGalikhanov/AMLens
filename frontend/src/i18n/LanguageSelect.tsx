import { getLocale, setLocale, t, type Locale } from './core.ts';
import { useLocale } from './react';

export default function LanguageSelect() {
  useLocale();
  return <label className="language-select">
    <span className="sr-only">{t('Language')}</span>
    <select aria-label={t('Language')} value={getLocale()} onChange={event => setLocale(event.target.value as Locale)}>
      <option value="en" lang="en">English</option>
      <option value="ru" lang="ru">Русский</option>
      <option value="kk" lang="kk">Қазақша</option>
    </select>
  </label>;
}
