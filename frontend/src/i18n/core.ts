import messages from './messages.json' with { type: 'json' };

export type Locale = 'en' | 'ru' | 'kk';
export const locales: Locale[] = ['en', 'ru', 'kk'];
const storageKey = 'amlens.language';
const supported = (value: unknown): value is Locale => locales.includes(value as Locale);
export function readLocale(storage?: Pick<Storage, 'getItem'>): Locale {
  try { const saved = storage?.getItem(storageKey); return supported(saved) ? saved : 'en'; }
  catch { return 'en'; }
}
let locale: Locale = 'en';
try { locale = readLocale(globalThis.localStorage); } catch { /* Storage may be blocked. */ }
const listeners = new Set<() => void>();
export const getLocale = () => locale;
export function subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; }
export function setLocale(next: Locale) {
  if (!supported(next) || next === locale) return;
  locale = next;
  try { globalThis.localStorage?.setItem(storageKey, next); } catch { /* Keep the session choice. */ }
  updateDocument();
  for (const listener of listeners) listener();
}
type Entry = Record<Locale, string>;
const catalog = messages as Record<string, Entry>;
// Reverse lookup also updates already displayed validation and status messages.
const lookup = new Map<string, Entry>();
for (const entry of Object.values(catalog)) for (const value of Object.values(entry)) lookup.set(value, entry);
export function t(key: string, parameters: unknown[] = []): string {
  const template = (catalog[key] ?? lookup.get(key))?.[locale] ?? key;
  return template.replace(/\{(\d+)\}/g, (match, index) => {
    const value = parameters[Number(index)];
    return value === undefined ? match : typeof value === 'number' ? value.toLocaleString(locale) : String(value);
  });
}
export function updateDocument() {
  if (typeof document === 'undefined') return;
  document.documentElement.lang = locale;
  document.title = t('AMLens — Transfer network analysis');
}
updateDocument();
if (typeof window !== 'undefined') window.addEventListener('storage', (event) => {
  if (event.key === storageKey) setLocale(supported(event.newValue) ? event.newValue : 'en');
});
